package controller

import (
	"context"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/cryptomus"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// subscriptionCryptomusPrefix marks an invoice that buys a plan instead of
// topping up the wallet.
const subscriptionCryptomusPrefix = "SCRY"

type SubscriptionCryptomusPayRequest struct {
	PlanId int `json:"plan_id"`
}

// RequestSubscriptionCryptomusPay opens a USDT invoice that buys a plan
// directly; the money never passes through the wallet.
func RequestSubscriptionCryptomusPay(c *gin.Context) {
	if !isCryptoTopUpEnabled() {
		common.ApiErrorT(c, "Payment method is not available")
		return
	}
	var req SubscriptionCryptomusPayRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.PlanId <= 0 {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	plan, err := model.GetSubscriptionPlanById(req.PlanId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !plan.Enabled {
		common.ApiErrorT(c, "Subscription plan is not enabled")
		return
	}
	if plan.PriceAmount < 0.01 {
		common.ApiErrorT(c, "Plan amount is too low")
		return
	}
	userId := c.GetInt("id")
	if plan.MaxPurchasePerUser > 0 {
		count, err := model.CountUserSubscriptionsByPlan(userId, plan.Id)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		if count >= int64(plan.MaxPurchasePerUser) {
			common.ApiErrorT(c, "Purchase limit for this plan has been reached")
			return
		}
	}
	now := common.GetTimestamp()
	tradeNo := subscriptionCryptomusPrefix + common.GetRandomString(20)
	order := &model.SubscriptionOrder{
		UserId:          userId,
		PlanId:          plan.Id,
		Money:           plan.PriceAmount,
		TradeNo:         tradeNo,
		PaymentMethod:   model.PaymentMethodCryptomus,
		PaymentProvider: model.PaymentProviderCryptomus,
		CreateTime:      now,
		Status:          common.TopUpStatusPending,
	}
	if err := order.Insert(); err != nil {
		common.ApiErrorT(c, "Failed to create order")
		return
	}
	origin := cryptoOrigin()
	info, err := cryptoClient().Create(c.Request.Context(), cryptomus.CreateRequest{
		Amount:            decimal.NewFromFloat(plan.PriceAmount).StringFixed(2),
		Currency:          "USD",
		OrderID:           tradeNo,
		Network:           setting.CryptoNetwork,
		ToCurrency:        "USDT",
		Currencies:        []cryptomus.Currency{{Currency: "USDT", Network: setting.CryptoNetwork}},
		URLCallback:       origin + "/api/cryptomus/webhook",
		URLReturn:         origin + "/wallet",
		URLSuccess:        origin + "/wallet",
		Lifetime:          3600,
		IsPaymentMultiple: true,
	})
	if err != nil {
		_ = model.ExpireSubscriptionOrder(tradeNo, model.PaymentProviderCryptomus)
		common.ApiErrorT(c, "Failed to create order")
		return
	}
	common.ApiSuccess(c, gin.H{
		"trade_no":     tradeNo,
		"checkout_url": info.URL,
		"amount_usd":   plan.PriceAmount,
		"currency":     "USDT",
		"network":      setting.CryptoNetwork,
		"expires_at":   now + 3600,
	})
}

// applyCryptoSubscriptionInfo activates a plan order once the provider reports
// a final, covered payment. Anything else leaves the order pending.
func applyCryptoSubscriptionInfo(info cryptomus.Payment) error {
	order := model.GetSubscriptionOrderByTradeNo(info.OrderID)
	if order == nil || order.PaymentProvider != model.PaymentProviderCryptomus {
		return nil
	}
	if order.Status != common.TopUpStatusPending {
		return nil
	}
	status := info.Status
	if status == "" {
		status = info.PaymentStatus
	}
	switch status {
	case "paid", "paid_over":
	case "expired", "cancel", "fail", "system_fail":
		return model.ExpireSubscriptionOrder(order.TradeNo, model.PaymentProviderCryptomus)
	default:
		return nil
	}
	if !cryptoPaymentCovers(info, order.Money) {
		return nil
	}
	return model.CompleteSubscriptionOrder(order.TradeNo, common.GetJsonString(info), model.PaymentProviderCryptomus, model.PaymentMethodCryptomus)
}

// cryptoPaymentCovers reports whether the settled amount reaches the plan
// price. The explicit USD figure wins when the provider sends it.
func cryptoPaymentCovers(info cryptomus.Payment, priceUSD float64) bool {
	expected := decimal.NewFromFloat(priceUSD)
	if strings.TrimSpace(info.PaymentAmountUSD) != "" {
		paid, err := decimal.NewFromString(info.PaymentAmountUSD)
		return err == nil && paid.GreaterThanOrEqual(expected)
	}
	if strings.TrimSpace(info.PaymentAmount) == "" || info.Currency != "USD" {
		return false
	}
	paid, err := decimal.NewFromString(info.PaymentAmount)
	return err == nil && paid.GreaterThanOrEqual(expected)
}

// settleCryptomusSubscriptionOrders polls plan invoices that are still pending,
// so a missed webhook cannot leave a paid order unactivated.
func settleCryptomusSubscriptionOrders(client cryptomus.Client) {
	orders, err := model.GetPendingCryptoSubscriptionOrders(common.GetTimestamp()-60, 50)
	if err != nil {
		return
	}
	for _, order := range orders {
		info, err := client.Info(context.Background(), order.TradeNo)
		if err != nil {
			continue
		}
		_ = applyCryptoSubscriptionInfo(info)
	}
}
