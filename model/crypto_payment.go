package model

import (
	"bytes"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/pkg/cryptomus"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// Decimal strings preserve the provider quote independently of display settings.
type CryptoPayment struct {
	TradeNo        string  `json:"trade_no" gorm:"primaryKey;type:varchar(100)"`
	InvoiceID      *string `json:"-" gorm:"uniqueIndex;type:varchar(36)"`
	UserId         int     `json:"-" gorm:"index"`
	AmountUSD      string  `json:"amount_usd" gorm:"type:varchar(128)"`
	BaseQuota      int     `json:"base_quota" gorm:"type:bigint"`
	BonusQuota     int     `json:"bonus_quota" gorm:"type:bigint"`
	Currency       string  `json:"currency" gorm:"type:varchar(16)"`
	Network        string  `json:"network" gorm:"type:varchar(32)"`
	CheckoutURL    string  `json:"checkout_url" gorm:"type:text"`
	PayerAmount    string  `json:"payer_amount" gorm:"type:varchar(128)"`
	PaidAmount     string  `json:"-" gorm:"type:varchar(128)"`
	PaidUSD        string  `json:"-" gorm:"type:varchar(128)"`
	MerchantAmount string  `json:"-" gorm:"type:varchar(128)"`
	Commission     string  `json:"-" gorm:"type:varchar(128)"`
	ProviderStatus string  `json:"provider_status" gorm:"type:varchar(32)"`
	ExpiresAt      int64   `json:"expires_at"`
	NextCheckAt    int64   `json:"-" gorm:"index"`
	NeedsReview    bool    `json:"needs_review"`
	CreateTime     int64   `json:"-"`
	Status         string  `json:"status" gorm:"-"`
}

var cryptoNetworkCode = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,32}$`)
var cryptoDecimalAmount = regexp.MustCompile(`^[0-9]{1,32}(\.[0-9]{1,32})?$`)

func parseCryptoAmount(value string) (decimal.Decimal, error) {
	if !cryptoDecimalAmount.MatchString(value) {
		return decimal.Zero, ErrInvalidTopUpQuota
	}
	return decimal.NewFromString(value)
}

func validateCryptoOption(key, value string) error {
	switch key {
	case "CryptomusPaymentAPIKeyConfigured":
		return common.NewMessage("Cryptomus key configuration status is read-only")
	case "CryptoEnabled":
		if value != "true" && value != "false" {
			return common.NewMessage("Invalid parameters")
		}
		// Readiness is evaluated when the payment method is listed, so saving
		// the toggle before the credentials in the same form cannot fail here.
	case "CryptomusMerchantID":
		if value != "" {
			if _, err := uuid.Parse(strings.TrimSpace(value)); err != nil {
				return common.NewMessage("Invalid Cryptomus merchant ID")
			}
		}
	case "CryptomusPaymentAPIKey":
		if len(strings.TrimSpace(value)) > 512 {
			return common.NewMessage("Invalid Cryptomus payment API key")
		}
	case "CryptoNetwork":
		if value != "" && !cryptoNetworkCode.MatchString(strings.TrimSpace(value)) {
			return common.NewMessage("Invalid crypto network code")
		}
	case "CryptoMinTopUp", "CryptoMaxTopUp":
		amount, err := strconv.Atoi(value)
		if err != nil || amount < 1 || amount > 1_000_000 {
			return common.NewMessage("Crypto top-up limits must be whole USD amounts between 1 and 1000000")
		}
	}
	return nil
}

func CreateCryptoTopUp(order *TopUp, payment *CryptoPayment) error {
	if order == nil || payment == nil || order.TradeNo == "" || order.TradeNo != payment.TradeNo || order.UserId != payment.UserId || order.PaymentProvider != PaymentProviderCryptomus || order.PaymentMethod != PaymentMethodCryptomus || order.Status != common.TopUpStatusPending || payment.BaseQuota <= 0 || payment.BonusQuota < 0 || payment.Currency != "USDT" || payment.Network == "" {
		return ErrInvalidTopUpQuota
	}
	if amount, err := parseCryptoAmount(payment.AmountUSD); err != nil || !amount.IsPositive() {
		return ErrInvalidTopUpQuota
	}
	if _, err := common.WalletQuotaFromDecimalStrict(decimal.NewFromInt(int64(payment.BaseQuota)).Add(decimal.NewFromInt(int64(payment.BonusQuota)))); err != nil {
		return err
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(order).Error; err != nil {
			return err
		}
		return tx.Create(payment).Error
	})
}

func GetCryptoPayment(tradeNo string, userId int) (*CryptoPayment, error) {
	var payment CryptoPayment
	query := DB.Where("trade_no = ?", tradeNo)
	if userId > 0 {
		query = query.Where("user_id = ?", userId)
	}
	if err := query.First(&payment).Error; err != nil {
		return nil, err
	}
	var order TopUp
	if err := DB.Select("status").Where("trade_no = ? AND payment_provider = ?", tradeNo, PaymentProviderCryptomus).First(&order).Error; err != nil {
		return nil, err
	}
	payment.Status = order.Status
	return &payment, nil
}

func GetDueCryptoPayments(now int64, limit int) ([]CryptoPayment, error) {
	var payments []CryptoPayment
	err := DB.Where("next_check_at > 0 AND next_check_at <= ?", now).Order("next_check_at, trade_no").Limit(min(max(limit, 1), 100)).Find(&payments).Error
	return payments, err
}

func QueueCryptoPayment(tradeNo, invoiceID string) error {
	if _, err := uuid.Parse(invoiceID); err != nil {
		return ErrPaymentMethodMismatch
	}
	result := DB.Model(&CryptoPayment{}).
		Where("trade_no = ? AND next_check_at > 0 AND (invoice_id IS NULL OR invoice_id = ?)", tradeNo, invoiceID).
		Update("next_check_at", common.GetTimestamp())
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		var count int64
		if err := DB.Model(&CryptoPayment{}).Where("trade_no = ?", tradeNo).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return ErrTopUpNotFound
		}
		// MySQL can report zero for an unchanged timestamp in the same second.
		var payment CryptoPayment
		if err := DB.Where("trade_no = ?", tradeNo).First(&payment).Error; err != nil {
			return err
		}
		if payment.InvoiceID != nil && *payment.InvoiceID != invoiceID {
			return ErrPaymentMethodMismatch
		}
	}
	return nil
}

func DeferCryptoPayment(tradeNo string, nextCheckAt int64) error {
	// An invoice the provider never issued cannot be recovered forever: after an
	// hour of failed retries the order stops consuming poll requests.
	now := common.GetTimestamp()
	if err := DB.Model(&CryptoPayment{}).Where("trade_no = ? AND invoice_id IS NULL AND create_time < ?", tradeNo, now-3600).Update("next_check_at", 0).Error; err != nil {
		return err
	}
	return DB.Model(&CryptoPayment{}).Where("trade_no = ? AND next_check_at > 0", tradeNo).Update("next_check_at", nextCheckAt).Error
}

// Only authenticated invoice queries reach this finalizer; callbacks only queue work.
func ApplyCryptomusPayment(info cryptomus.Payment, callerIP string) error {
	if _, err := uuid.Parse(info.UUID); err != nil || info.OrderID == "" {
		return ErrPaymentMethodMismatch
	}
	var creditedQuota, userId int
	err := DB.Transaction(func(tx *gorm.DB) error {
		var order TopUp
		if err := lockForUpdate(tx).Where("trade_no = ?", info.OrderID).First(&order).Error; err != nil {
			return err
		}
		if order.PaymentProvider != PaymentProviderCryptomus || order.PaymentMethod != PaymentMethodCryptomus {
			return ErrPaymentMethodMismatch
		}
		var payment CryptoPayment
		if err := lockForUpdate(tx).Where("trade_no = ?", info.OrderID).First(&payment).Error; err != nil {
			return err
		}
		if payment.UserId != order.UserId || payment.InvoiceID != nil && *payment.InvoiceID != info.UUID {
			return ErrPaymentMethodMismatch
		}

		status := info.Status
		if status == "" {
			status = info.PaymentStatus
		}
		amount, amountErr := parseCryptoAmount(info.Amount)
		expectedUSD, expectedErr := parseCryptoAmount(payment.AmountUSD)
		payerAmount, payerErr := parseCryptoAmount(info.PayerAmount)
		paidAmount, paidErr := parseCryptoAmount(info.PaymentAmount)
		paidUSD, paidUSDErr := parseCryptoAmount(info.PaymentAmountUSD)
		// Cryptomus echoes the invoice currency, which is USD here; a USDT
		// echo is the same money at the 1:1 peg the invoice was priced with.
		currencyOK := info.Currency == "USD" || info.Currency == payment.Currency
		invalid := amountErr != nil || expectedErr != nil || !expectedUSD.IsPositive() || !amount.Equal(expectedUSD) || !currencyOK || info.PayerCurrency != payment.Currency || info.Network != payment.Network || info.PaymentStatus != "" && info.PaymentStatus != status
		if len(info.ConvertFrom) > 0 && !bytes.Equal(bytes.TrimSpace(info.ConvertFrom), []byte("null")) {
			invalid = true
		}
		// payer_amount is the planned amount quoted at creation; a different
		// figure means the invoice was repriced or the wrong one came back.
		if payment.PayerAmount != "" {
			quoted, err := parseCryptoAmount(payment.PayerAmount)
			invalid = invalid || err != nil || payerErr != nil || !payerAmount.Equal(quoted)
		}
		payment.InvoiceID = &info.UUID
		payment.ProviderStatus = status
		payment.PaidAmount, payment.PaidUSD = info.PaymentAmount, info.PaymentAmountUSD
		payment.MerchantAmount, payment.Commission = info.MerchantAmount, info.Commission
		// Keep the first quote: overwriting it with a repriced invoice would let
		// a later call compare the new price against itself and credit.
		if payment.PayerAmount == "" && !invalid && payerErr == nil && payerAmount.IsPositive() {
			payment.PayerAmount = info.PayerAmount
		}
		if info.ExpiredAt > 0 {
			payment.ExpiresAt = info.ExpiredAt
		}
		if info.URL != "" {
			checkout, err := url.Parse(info.URL)
			if err != nil || checkout.Scheme != "https" || checkout.Host != "pay.cryptomus.com" || checkout.User != nil || checkout.Path != "/pay/"+info.UUID || checkout.RawQuery != "" || checkout.Fragment != "" {
				invalid = true
			} else {
				payment.CheckoutURL = info.URL
			}
		}
		// payment_amount is the settled value in the invoice currency, so the
		// invoice is covered only when it reaches the quoted USD amount; the
		// explicit USD field wins when the provider reports it.
		// The provider converts the invoice through its own rate, so the settled
		// figure can sit a cent under the quote and still be the full payment.
		paid := status == "paid" || status == "paid_over"
		expectedText := amount.String()
		covered := paidErr == nil && cryptomus.CoversAmount(paidAmount.String(), expectedText)
		if paidUSDErr == nil && paidUSD.IsPositive() {
			covered = cryptomus.CoversAmount(paidUSD.String(), expectedText)
		}
		if paid && (!info.IsFinal || payerErr != nil || !payerAmount.IsPositive() || !covered) {
			invalid = true
		}
		wasReview := payment.NeedsReview
		payment.NeedsReview = invalid || status == "paid_over" || status == "locked" || strings.HasPrefix(status, "refund_") || status == "wrong_amount"
		// A final invoice never changes again; polling it would only cost requests.
		payment.NextCheckAt = 0
		if !info.IsFinal {
			payment.NextCheckAt = common.GetTimestamp() + 60
		}
		if err := tx.Save(&payment).Error; err != nil {
			return err
		}
		// Nobody else surfaces a held payment: record it once in the user's log
		// so an administrator reviewing logs can reconcile it.
		if payment.NeedsReview && !wasReview {
			other := NewLogOther()
			log := &Log{
				UserId:    order.UserId,
				CreatedAt: common.GetTimestamp(),
				Type:      LogTypeSystem,
				Content: other.setContent([]*common.Message{common.NewMessage(
					"Crypto payment {{tradeNo}} needs manual review, provider status: {{status}}",
					map[string]any{"tradeNo": order.TradeNo, "status": status},
				)}),
				Other: other.JSONString(),
			}
			if err := tx.Create(log).Error; err != nil {
				return err
			}
		}
		if invalid || !paid || order.Status == common.TopUpStatusSuccess {
			return nil
		}
		if order.Status != common.TopUpStatusPending {
			return ErrTopUpStatusInvalid
		}
		order.Status, order.CompleteTime = common.TopUpStatusSuccess, common.GetTimestamp()
		if err := tx.Save(&order).Error; err != nil {
			return err
		}
		signup, err := claimDeferredSignupBonus(tx, order.UserId)
		if err != nil {
			return err
		}
		creditedQuota, err = common.WalletQuotaFromDecimalStrict(decimal.NewFromInt(int64(payment.BaseQuota)).Add(decimal.NewFromInt(int64(payment.BonusQuota))).Add(decimal.NewFromInt(int64(signup))))
		if err != nil || payment.BaseQuota <= 0 || payment.BonusQuota < 0 {
			return ErrInvalidTopUpQuota
		}
		if err := creditTopUpQuota(tx, order.UserId, creditedQuota, nil); err != nil {
			return err
		}
		if _, err := CreditReferralCommission(tx, order.UserId, payment.BaseQuota); err != nil {
			return err
		}
		if !payment.NeedsReview {
			if err := tx.Model(&payment).Update("next_check_at", 0).Error; err != nil {
				return err
			}
		}
		userId = order.UserId
		return nil
	})
	if err != nil || userId == 0 {
		return err
	}
	syncCreditUserQuotaCache(userId, creditedQuota, "cryptomus topup")
	RecordTopupLog(userId, common.NewMessage("Online top-up succeeded, amount added: {{quota}}, payment amount: {{amount}}", map[string]any{"quota": logger.FormatQuota(creditedQuota), "amount": fmt.Sprintf("%s USD", info.Amount)}), callerIP, PaymentMethodCryptomus, PaymentProviderCryptomus)
	return nil
}
