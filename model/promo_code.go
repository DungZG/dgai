package model

import (
	"errors"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"gorm.io/gorm"
)

// PromoCodeRulesOptionKey holds the promotion list as a JSON array in the
// options table, so a campaign is a data edit rather than a deploy.
const PromoCodeRulesOptionKey = "PromoCodeRules"

// Promo code grants are recorded as zero-money top-ups whose trade number
// encodes the code and the user; the unique index on trade_no is what keeps a
// promotion to one grant per account.
const promoCodePaymentMethod = "promo"

type PromoCodeRule struct {
	Code        string  `json:"code"`
	Quota       int     `json:"quota"`
	MinTopUpUSD float64 `json:"min_top_up_usd"`
	ExpiresAt   int64   `json:"expires_at"`
}

func LoadPromoCodeRules() []PromoCodeRule {
	common.OptionMapRWMutex.RLock()
	raw := strings.TrimSpace(common.OptionMap[PromoCodeRulesOptionKey])
	common.OptionMapRWMutex.RUnlock()
	if raw == "" {
		return nil
	}
	var rules []PromoCodeRule
	if err := common.UnmarshalJsonStr(raw, &rules); err != nil {
		common.SysError(common.LogText("failed to parse promo code rules: %s", err.Error()))
		return nil
	}
	return rules
}

func findPromoCodeRule(rules []PromoCodeRule, code string) *PromoCodeRule {
	for i := range rules {
		if strings.EqualFold(strings.TrimSpace(rules[i].Code), code) {
			return &rules[i]
		}
	}
	return nil
}

var errPromoCodeUnavailable = errors.New("promo code unavailable")

// RedeemPromoCode grants a promotion once per user and only after the paid
// total reaches the configured minimum.
func RedeemPromoCode(code string, userId int) (int, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" || userId <= 0 {
		return 0, errPromoCodeUnavailable
	}
	rule := findPromoCodeRule(LoadPromoCodeRules(), code)
	if rule == nil {
		return 0, errPromoCodeUnavailable
	}
	if rule.ExpiresAt > 0 && rule.ExpiresAt < common.GetTimestamp() {
		return 0, errPromoCodeUnavailable
	}
	if rule.Quota <= 0 {
		return 0, errPromoCodeUnavailable
	}
	if err := common.ValidateWalletQuota(rule.Quota); err != nil {
		return 0, errPromoCodeUnavailable
	}
	tradeNo := promoCodePaymentMethod + "-" + code + "-" + strconv.Itoa(userId)
	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).Select("id").Where("id = ?", userId).First(&user).Error; err != nil {
			return err
		}
		if rule.MinTopUpUSD > 0 {
			paid, err := paidTopUpTotalTx(tx, userId)
			if err != nil {
				return err
			}
			if paid < rule.MinTopUpUSD {
				return errPromoCodeUnavailable
			}
		}
		now := common.GetTimestamp()
		grant := &TopUp{
			UserId:          userId,
			Amount:          int64(rule.Quota),
			Money:           0,
			TradeNo:         tradeNo,
			PaymentMethod:   promoCodePaymentMethod,
			PaymentProvider: promoCodePaymentMethod,
			CreateTime:      now,
			CompleteTime:    now,
			Status:          common.TopUpStatusSuccess,
		}
		if err := tx.Create(grant).Error; err != nil {
			// The unique trade number rejects a second redemption by the same
			// account, including concurrent attempts.
			return errPromoCodeUnavailable
		}
		return creditTopUpQuota(tx, userId, rule.Quota, nil)
	})
	if err != nil {
		return 0, errPromoCodeUnavailable
	}
	syncCreditUserQuotaCache(userId, rule.Quota, common.LogText("promo code"))
	RecordLog(userId, LogTypeTopup, common.NewMessage("Promo code {{code}} applied, amount added: {{quota}}", map[string]any{
		"code":  code,
		"quota": logger.FormatQuota(rule.Quota),
	}))
	return rule.Quota, nil
}
