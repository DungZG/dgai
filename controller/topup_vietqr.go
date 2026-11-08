package controller

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

type VietQRPayRequest struct {
	Amount int64 `json:"amount"`
}

// vietqrMemo builds the transfer memo. VietQR caps `content` at 23 characters
// and `orderId` at 13, both Latin-only, so the code is kept short and unique.
func vietqrMemo(userId int, tradeNo string) string {
	return fmt.Sprintf("DGAI%d%s", userId, tradeNo[:6])
}

// vietqrOrderId is what VietQR echoes back in the transaction callback.
func vietqrOrderId(tradeNo string) string {
	return tradeNo
}

// vietqrVNDAmount converts the order amount (billing currency, USD by default)
// into the VND figure the bank transfer must carry.
func vietqrVNDAmount(payMoney float64) int64 {
	rate := setting.VietQRVNDRate
	if rate <= 0 {
		rate = 26500
	}
	return int64(math.Round(payMoney * rate))
}

func vietqrQRImageURL(vndAmount int64, memo string) string {
	base := "https://img.vietqr.io/image"
	template := "compact2.png"
	q := url.Values{}
	q.Set("amount", strconv.FormatInt(vndAmount, 10))
	q.Set("addInfo", memo)
	if strings.TrimSpace(setting.VietQRAccountName) != "" {
		q.Set("accountName", setting.VietQRAccountName)
	}
	return fmt.Sprintf("%s/%s-%s-%s?%s", base, setting.VietQRBankCode, setting.VietQRBankAccount, template, q.Encode())
}

func isVietQRTopUpEnabled() bool {
	if !isPaymentComplianceConfirmed() {
		return false
	}
	return setting.VietQREnabled &&
		strings.TrimSpace(setting.VietQRBankCode) != "" &&
		strings.TrimSpace(setting.VietQRBankAccount) != ""
}

// RequestVietQRPay creates a pending top-up order and returns the QR image plus
// the exact memo the customer must keep in the transfer description.
func RequestVietQRPay(c *gin.Context) {
	if !isVietQRTopUpEnabled() {
		paymentError(c, common.NewMessage("VietQR top-up is not configured"))
		return
	}
	var req VietQRPayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		paymentError(c, common.NewMessage("Invalid parameters"))
		return
	}
	minTopUp := int64(setting.VietQRMinTopUp)
	if minTopUp <= 0 {
		minTopUp = 1
	}
	if req.Amount < minTopUp {
		paymentError(c, common.NewMessage("Top-up amount cannot be less than {{min}}", map[string]any{"min": minTopUp}))
		return
	}
	id := c.GetInt("id")
	if rejectInvalidTopUpQuota(c, id, req.Amount) {
		return
	}

	group, err := model.GetUserGroup(id, true)
	if err != nil {
		paymentError(c, common.NewMessage("Failed to get the user group"))
		return
	}
	payMoney := getPayMoney(req.Amount, group)
	if payMoney < 0.01 {
		paymentError(c, common.NewMessage("Payment amount is too low"))
		return
	}

	tradeNo := fmt.Sprintf("VQR%d%s", id, common.GetRandomString(8))
	memo := vietqrMemo(id, tradeNo)
	vndAmount := vietqrVNDAmount(payMoney)

	topUp := &model.TopUp{
		UserId:          id,
		Amount:          req.Amount,
		Money:           payMoney,
		TradeNo:         tradeNo,
		PaymentMethod:   model.PaymentMethodVietQR,
		PaymentProvider: model.PaymentProviderVietQR,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}
	if err := topUp.Insert(); err != nil {
		logger.LogError(c.Request.Context(), common.LogText("vietqr failed to create top-up order user_id=%d trade_no=%s error=%q", id, tradeNo, err.Error()))
		paymentError(c, common.NewMessage("Failed to create order"))
		return
	}

	common.ApiSuccess(c, gin.H{
		"trade_no":     tradeNo,
		"amount":       req.Amount,
		"pay_money":    payMoney,
		"memo":         memo,
		"qr_image_url": vietqrQRImageURL(vndAmount, memo),
		"vnd_amount":   vndAmount,
		"vnd_rate":     setting.VietQRVNDRate,
		"bank_code":    setting.VietQRBankCode,
		"bank_account": setting.VietQRBankAccount,
		"account_name": setting.VietQRAccountName,
		"expires_at":   time.Now().Add(30 * time.Minute).Unix(),
	})
}

// vietqrSyncBearer is the token VietQR must present when pushing transactions.
// It is derived from the configured secret so both sides agree without storage.
func vietqrSyncBearer(now time.Time) string {
	window := now.Unix() / 300
	mac := hmac.New(sha256.New, []byte(setting.VietQRSyncSecretKey))
	mac.Write([]byte(fmt.Sprintf("vietqr-sync:%d", window)))
	return hex.EncodeToString(mac.Sum(nil))
}

func validVietQRSyncBearer(token string) bool {
	if strings.TrimSpace(setting.VietQRSyncSecretKey) == "" {
		return false
	}
	now := time.Now()
	// Accept the current and previous window to tolerate clock skew.
	return hmac.Equal([]byte(token), []byte(vietqrSyncBearer(now))) ||
		hmac.Equal([]byte(token), []byte(vietqrSyncBearer(now.Add(-5*time.Minute))))
}

// VietQRTokenGenerate hands VietQR a short-lived bearer token. VietQR calls this
// with the Basic credentials we define, then uses the token on transaction-sync.
func VietQRTokenGenerate(c *gin.Context) {
	if strings.TrimSpace(setting.VietQRSyncUsername) == "" || strings.TrimSpace(setting.VietQRSyncPassword) == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"status": "FAILED", "message": "sync credentials not configured"})
		return
	}
	username, password, ok := c.Request.BasicAuth()
	if !ok ||
		!hmac.Equal([]byte(username), []byte(setting.VietQRSyncUsername)) ||
		!hmac.Equal([]byte(password), []byte(setting.VietQRSyncPassword)) {
		c.JSON(http.StatusUnauthorized, gin.H{"status": "FAILED", "message": "invalid credentials"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"access_token": vietqrSyncBearer(time.Now()),
		"token_type":   "Bearer",
		"expires_in":   300,
	})
}

type VietQRTransactionSyncRequest struct {
	BankAccount     string  `json:"bankaccount"`
	Amount          float64 `json:"amount"`
	TransType       string  `json:"transType"`
	Content         string  `json:"content"`
	TransactionId   string  `json:"transactionid"`
	TransactionTime int64   `json:"transactiontime"`
	ReferenceNumber string  `json:"referencenumber"`
	OrderId         string  `json:"orderId"`
	TerminalCode    string  `json:"terminalCode"`
	SubTerminalCode string  `json:"subTerminalCode"`
	ServiceCode     string  `json:"serviceCode"`
	URLLink         string  `json:"urlLink"`
	Sign            string  `json:"sign"`
}

func vietqrSyncError(c *gin.Context, status int, reason string, message string) {
	c.JSON(status, gin.H{
		"error":        true,
		"errorReason":  reason,
		"toastMessage": message,
		"object":       nil,
	})
}

// VietQRTransactionSync receives balance changes from VietQR and credits the
// matching pending order. It is idempotent: replaying a transaction returns the
// same reference and never credits twice.
func VietQRTransactionSync(c *gin.Context) {
	auth := strings.TrimSpace(c.GetHeader("Authorization"))
	token, ok := strings.CutPrefix(auth, "Bearer ")
	if !ok || !validVietQRSyncBearer(strings.TrimSpace(token)) {
		vietqrSyncError(c, http.StatusUnauthorized, "INVALID_TOKEN", "Invalid or expired token")
		return
	}

	var req VietQRTransactionSyncRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		vietqrSyncError(c, http.StatusBadRequest, "INVALID_BODY", "Malformed transaction payload")
		return
	}
	if req.TransType != "" && req.TransType != "C" {
		// Debits are not top-ups.
		c.JSON(http.StatusOK, gin.H{
			"error":        false,
			"errorReason":  "",
			"toastMessage": "ignored",
			"object":       gin.H{"reftransactionid": req.TransactionId},
		})
		return
	}

	topUp := model.GetTopUpByTradeNo(req.OrderId)
	if topUp == nil || topUp.PaymentProvider != model.PaymentProviderVietQR {
		vietqrSyncError(c, http.StatusBadRequest, "ORDER_NOT_FOUND", "Order not found for this transaction")
		return
	}
	// The transfer amount arrives in VND; convert the order price back to VND
	// before comparing. A short transfer stays pending for manual handling
	// instead of silently crediting a partial payment.
	expectedVND := decimal.NewFromInt(vietqrVNDAmount(topUp.Money))
	if decimal.NewFromFloat(req.Amount).LessThan(expectedVND) {
		vietqrSyncError(c, http.StatusBadRequest, "AMOUNT_MISMATCH", "Transferred amount is lower than the order amount")
		return
	}

	LockOrder(topUp.TradeNo)
	defer UnlockOrder(topUp.TradeNo)
	if err := model.ManualCompleteTopUp(topUp.TradeNo, c.ClientIP()); err != nil {
		logger.LogError(c.Request.Context(), common.LogText("vietqr transaction sync failed trade_no=%s error=%q", topUp.TradeNo, err.Error()))
		vietqrSyncError(c, http.StatusBadRequest, "CREDIT_FAILED", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"error":        false,
		"errorReason":  "",
		"toastMessage": "Transaction processed successfully",
		"object":       gin.H{"reftransactionid": req.TransactionId},
	})
}
