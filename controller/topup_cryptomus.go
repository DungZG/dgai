package controller

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/cryptomus"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

func cryptoClient() cryptomus.Client {
	return cryptomus.Client{MerchantID: setting.CryptomusMerchantID, PaymentAPIKey: setting.CryptomusPaymentAPIKey}
}
func cryptoOrigin() string {
	u, err := url.Parse(strings.TrimRight(system_setting.ServerAddress, "/"))
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" {
		return ""
	}
	return u.String()
}
func isCryptoTopUpEnabled() bool {
	return operation_setting.IsPaymentComplianceConfirmed() && setting.CryptoEnabled && strings.TrimSpace(setting.CryptomusMerchantID) != "" && strings.TrimSpace(setting.CryptomusPaymentAPIKey) != "" && strings.TrimSpace(setting.CryptoNetwork) != "" && cryptoOrigin() != ""
}

// cryptoAmount turns the wallet amount the customer typed into the USD figure
// the invoice is denominated in. The display currency never reaches the
// invoice, and operation_setting.Price (a CNY top-up markup) is not applied.
func cryptoAmount(amount int64) (int, string, error) {
	if amount <= 0 {
		return 0, "", common.NewMessage("Invalid top-up amount")
	}
	usd := decimal.NewFromInt(amount)
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		usd = usd.Div(decimal.NewFromFloat(common.QuotaPerUnit))
	} else {
		usd = usd.Div(decimal.NewFromFloat(operation_setting.GetUsdToCurrencyRate(operation_setting.USDExchangeRate)))
	}
	usd = usd.Round(2)
	if usd.LessThan(decimal.NewFromInt(int64(setting.CryptoMinTopUp))) || usd.GreaterThan(decimal.NewFromInt(int64(setting.CryptoMaxTopUp))) {
		return 0, "", common.NewMessage("Invalid top-up amount")
	}
	quota, err := common.WalletQuotaFromDecimalStrict(usd.Mul(decimal.NewFromFloat(common.QuotaPerUnit)))
	if err != nil || quota <= 0 {
		return 0, "", common.NewMessage("Invalid top-up amount")
	}
	return quota, usd.String(), nil
}
func cryptoResponse(payment *model.CryptoPayment) gin.H {
	return gin.H{"trade_no": payment.TradeNo, "status": payment.Status, "provider_status": payment.ProviderStatus, "checkout_url": payment.CheckoutURL, "amount_usd": payment.AmountUSD, "currency": "USDT", "network": payment.Network, "base_quota": payment.BaseQuota, "bonus_quota": payment.BonusQuota, "expires_at": payment.ExpiresAt, "needs_review": payment.NeedsReview}
}
func RequestCryptomusPay(c *gin.Context) {
	if !isCryptoTopUpEnabled() {
		common.ApiErrorT(c, "Payment method is not available")
		return
	}
	var request struct {
		Amount int64 `json:"amount"`
	}
	if c.ShouldBindJSON(&request) != nil {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	quota, usd, err := cryptoAmount(request.Amount)
	if err != nil {
		common.ApiErrorT(c, "Invalid top-up amount")
		return
	}
	bonus, err := operation_setting.TopUpBonus(quota)
	if err != nil {
		common.ApiErrorT(c, "Invalid top-up amount")
		return
	}
	total, err := common.WalletQuotaFromDecimalStrict(decimal.NewFromInt(int64(quota)).Add(decimal.NewFromInt(int64(bonus))))
	if err != nil || model.ValidateTopUpQuotaCapacity(c.GetInt("id"), total) != nil {
		common.ApiErrorT(c, "Top-up quota exceeds the range the system can represent")
		return
	}
	now := common.GetTimestamp()
	tradeNo := "CRY" + common.GetRandomString(29)
	money, _ := decimal.RequireFromString(usd).Float64()
	order := &model.TopUp{UserId: c.GetInt("id"), Amount: request.Amount, Money: money, TradeNo: tradeNo, PaymentMethod: model.PaymentMethodCryptomus, PaymentProvider: model.PaymentProviderCryptomus, CreateTime: now, Status: common.TopUpStatusPending}
	payment := &model.CryptoPayment{TradeNo: tradeNo, UserId: c.GetInt("id"), AmountUSD: usd, BaseQuota: quota, BonusQuota: bonus, Network: setting.CryptoNetwork, Currency: "USDT", ExpiresAt: now + 3600, NextCheckAt: now + 60, CreateTime: now}
	if model.CreateCryptoTopUp(order, payment) != nil {
		common.ApiErrorT(c, "Failed to create order")
		return
	}
	origin := cryptoOrigin()
	client := cryptoClient()
	info, err := client.Create(c.Request.Context(), cryptomus.CreateRequest{Amount: usd, Currency: "USD", OrderID: tradeNo, Network: payment.Network, ToCurrency: "USDT", Currencies: []cryptomus.Currency{{Currency: "USDT", Network: payment.Network}}, URLCallback: origin + "/api/cryptomus/webhook", URLReturn: origin + "/wallet", URLSuccess: origin + "/wallet", Lifetime: 3600, IsPaymentMultiple: true, AccuracyPaymentPercent: cryptomus.SettlementTolerancePercent})
	if err != nil {
		info, err = client.Info(c.Request.Context(), tradeNo)
	}
	if err == nil {
		err = model.ApplyCryptomusPayment(info, c.ClientIP())
	}
	if err != nil {
		_ = model.DeferCryptoPayment(tradeNo, now+60)
	}
	stored, err := model.GetCryptoPayment(tradeNo, c.GetInt("id"))
	if err != nil {
		common.ApiErrorT(c, "Failed to get order")
		return
	}
	common.ApiSuccess(c, cryptoResponse(stored))
}
func GetCryptomusOrder(c *gin.Context) {
	payment, err := model.GetCryptoPayment(c.Param("trade_no"), c.GetInt("id"))
	if err != nil {
		common.ApiErrorT(c, "Order not found")
		return
	}
	common.ApiSuccess(c, cryptoResponse(payment))
}
func CryptomusWebhook(c *gin.Context) {
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, cryptomus.MaxResponseBytes+1))
	if err != nil || len(body) > cryptomus.MaxResponseBytes {
		c.Status(http.StatusBadRequest)
		return
	}
	info, err := cryptomus.VerifyWebhook(body, setting.CryptomusPaymentAPIKey)
	if err != nil {
		c.Status(http.StatusUnauthorized)
		return
	}
	if info.OrderID == "" || info.UUID == "" {
		// Test webhooks may carry only one identifier; nothing can be queued.
		c.JSON(http.StatusOK, gin.H{"success": true})
		return
	}
	if order := model.GetSubscriptionOrderByTradeNo(info.OrderID); order != nil {
		// A plan invoice never touches the wallet, so it skips the top-up queue.
		if err := applyCryptoSubscriptionInfo(info); err != nil && !errors.Is(err, model.ErrSubscriptionOrderNotFound) {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true})
		return
	}
	err = model.QueueCryptoPayment(info.OrderID, info.UUID)
	// An unknown or already-closed order is not retryable; only a storage
	// failure asks the provider to deliver again.
	if err != nil && !errors.Is(err, model.ErrTopUpNotFound) && !errors.Is(err, model.ErrPaymentMethodMismatch) {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
func StartCryptomusSettlementWorker(interval time.Duration) {
	if interval <= 0 {
		interval = time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		settleCryptomusPayments()
	}
}
func settleCryptomusPayments() {
	if setting.CryptomusMerchantID == "" || setting.CryptomusPaymentAPIKey == "" {
		return
	}
	payments, err := model.GetDueCryptoPayments(common.GetTimestamp(), 50)
	if err != nil {
		return
	}
	client := cryptoClient()
	for _, payment := range payments {
		info, err := client.Info(context.Background(), payment.TradeNo)
		if err == nil {
			err = model.ApplyCryptomusPayment(info, "")
		}
		if err != nil {
			_ = model.DeferCryptoPayment(payment.TradeNo, common.GetTimestamp()+60)
		}
	}
	settleCryptomusSubscriptionOrders(client)
}
