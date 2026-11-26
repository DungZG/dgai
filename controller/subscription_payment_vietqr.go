package controller

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
)

// subscriptionVietQRPrefix keeps a plan payment distinguishable from a wallet
// top-up ("VQR") while staying inside the fixed 11-character trade number the
// SePay memo matcher slices out.
const subscriptionVietQRPrefix = "SVR"

type SubscriptionVietQRPayRequest struct {
	PlanId int `json:"plan_id"`
}

// RequestSubscriptionVietQRPay opens a bank-transfer order that buys a plan
// directly; the money never passes through the wallet.
func RequestSubscriptionVietQRPay(c *gin.Context) {
	if !isVietQRTopUpEnabled() {
		common.ApiErrorT(c, "Payment method is not available")
		return
	}
	var req SubscriptionVietQRPayRequest
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
	tradeNo := subscriptionVietQRPrefix + strings.ToUpper(common.GetRandomString(vietqrTradeNoLen-len(subscriptionVietQRPrefix)))
	order := &model.SubscriptionOrder{
		UserId:          userId,
		PlanId:          plan.Id,
		Money:           plan.PriceAmount,
		TradeNo:         tradeNo,
		PaymentMethod:   model.PaymentMethodVietQR,
		PaymentProvider: model.PaymentProviderVietQR,
		CreateTime:      common.GetTimestamp(),
		Status:          common.TopUpStatusPending,
	}
	if err := order.Insert(); err != nil {
		common.ApiErrorT(c, "Failed to create order")
		return
	}
	vndAmount := vietqrVNDAmount(plan.PriceAmount)
	memo := vietqrMemo(tradeNo)
	common.ApiSuccess(c, gin.H{
		"trade_no":     tradeNo,
		"amount_usd":   plan.PriceAmount,
		"amount_vnd":   vndAmount,
		"memo":         memo,
		"qr_url":       vietqrQRImageURL(vndAmount, memo),
		"bank_code":    setting.VietQRBankCode,
		"bank_account": setting.VietQRBankAccount,
		"account_name": setting.VietQRAccountName,
		"expires_at":   common.GetTimestamp() + 3600,
	})
}

// matchSePaySubscriptionOrder finds the pending plan order a transfer pays for.
// SePay reports the payment code separately from the raw content, so both are
// tried, and only the plan prefix is considered here.
func matchSePaySubscriptionOrder(req sePayWebhookRequest) *model.SubscriptionOrder {
	candidates := make([]string, 0, 2)
	if req.Code != nil {
		candidates = append(candidates, strings.TrimSpace(*req.Code))
	}
	memo := vietqrNormalizeMemo(req.Content)
	if i := strings.Index(memo, "DGAI"); i >= 0 && len(memo) >= i+4+vietqrTradeNoLen {
		candidates = append(candidates, memo[i+4:i+4+vietqrTradeNoLen])
	}
	for _, candidate := range candidates {
		code := vietqrNormalizeMemo(candidate)
		if i := strings.Index(code, "DGAI"); i >= 0 && len(code) >= i+4+vietqrTradeNoLen {
			code = code[i+4 : i+4+vietqrTradeNoLen]
		}
		if len(code) != vietqrTradeNoLen || !strings.HasPrefix(code, subscriptionVietQRPrefix) {
			continue
		}
		order := model.GetSubscriptionOrderByTradeNo(code)
		if order != nil && order.PaymentProvider == model.PaymentProviderVietQR && order.Status == common.TopUpStatusPending {
			return order
		}
	}
	return nil
}
