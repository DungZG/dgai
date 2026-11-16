package operation_setting

import (
	"math"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/shopspring/decimal"
)

type TopUpBonusTier struct {
	MinAmountUSD float64 `json:"min_amount_usd"`
	Percent      float64 `json:"percent"`
}

const TopUpBonusTiersOptionKey = "payment_setting.topup_bonus_tiers"

type PaymentSetting struct {
	AmountOptions          []int            `json:"amount_options"`
	AmountDiscount         map[int]float64  `json:"amount_discount"`
	TopUpBonusTiers        []TopUpBonusTier `json:"topup_bonus_tiers"`
	SignupBonusMinTopUp    float64          `json:"signup_bonus_min_top_up"`
	ComplianceConfirmed    bool             `json:"compliance_confirmed"`
	ComplianceTermsVersion string           `json:"compliance_terms_version"`
	ComplianceConfirmedAt  int64            `json:"compliance_confirmed_at"`
	ComplianceConfirmedBy  int              `json:"compliance_confirmed_by"`
	ComplianceConfirmedIP  string           `json:"compliance_confirmed_ip"`
}

const CurrentComplianceTermsVersion = "v1"

// 默认配置
var paymentSetting = PaymentSetting{
	AmountOptions:   []int{10, 20, 50, 100, 200, 500},
	AmountDiscount:  map[int]float64{},
	TopUpBonusTiers: []TopUpBonusTier{},
}

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("payment_setting", &paymentSetting)
}

func GetPaymentSetting() *PaymentSetting {
	return &paymentSetting
}
func ValidateTopUpBonusTiersJSON(value string) error {
	trimmed := strings.TrimSpace(value)
	if !strings.HasPrefix(trimmed, "[") {
		return common.NewMessage("Top-up bonus tiers must be a JSON array")
	}
	var tiers []TopUpBonusTier
	if err := common.UnmarshalJsonStr(trimmed, &tiers); err != nil {
		return common.NewMessage("Top-up bonus tiers must be a JSON array")
	}
	if common.QuotaPerUnit <= 0 {
		return common.NewMessage("Quota per unit must be greater than 0")
	}
	previousMinimum := 0.0
	maxMinimum := float64(common.MaxWalletQuota) / common.QuotaPerUnit
	for _, tier := range tiers {
		if math.IsNaN(tier.MinAmountUSD) || math.IsInf(tier.MinAmountUSD, 0) || tier.MinAmountUSD <= previousMinimum || tier.MinAmountUSD > maxMinimum {
			return common.NewMessage("Top-up bonus tier minimum amounts must be positive, increasing, and within the wallet quota limit")
		}
		if math.IsNaN(tier.Percent) || math.IsInf(tier.Percent, 0) || tier.Percent < 0 || tier.Percent > 100 {
			return common.NewMessage("Top-up bonus tier percentages must be between 0 and 100")
		}
		previousMinimum = tier.MinAmountUSD
	}
	return nil
}
func TopUpBonus(baseQuota int) (int, error) {
	if baseQuota <= 0 || common.QuotaPerUnit <= 0 {
		return 0, nil
	}
	amountUSD := float64(baseQuota) / common.QuotaPerUnit
	var selectedMinimum, percent float64
	for _, tier := range paymentSetting.TopUpBonusTiers {
		if tier.MinAmountUSD > selectedMinimum && tier.MinAmountUSD <= amountUSD &&
			tier.MinAmountUSD > 0 && tier.Percent >= 0 && tier.Percent <= 100 &&
			!math.IsNaN(tier.MinAmountUSD) && !math.IsInf(tier.MinAmountUSD, 0) &&
			!math.IsNaN(tier.Percent) && !math.IsInf(tier.Percent, 0) {
			selectedMinimum = tier.MinAmountUSD
			percent = tier.Percent
		}
	}
	if percent == 0 {
		return 0, nil
	}
	return common.WalletQuotaFromDecimalStrict(
		decimal.NewFromInt(int64(baseQuota)).Mul(decimal.NewFromFloat(percent)).Div(decimal.NewFromInt(100)),
	)
}

func TopUpQuotaWithBonus(baseQuota int) (int, error) {
	if baseQuota <= 0 {
		return 0, common.NewMessage("Top-up quota must be greater than 0")
	}
	bonus, err := TopUpBonus(baseQuota)
	if err != nil {
		return 0, err
	}
	return common.WalletQuotaFromDecimalStrict(
		decimal.NewFromInt(int64(baseQuota)).Add(decimal.NewFromInt(int64(bonus))),
	)
}

func IsPaymentComplianceConfirmed() bool {
	return paymentSetting.ComplianceConfirmed &&
		paymentSetting.ComplianceTermsVersion == CurrentComplianceTermsVersion
}
