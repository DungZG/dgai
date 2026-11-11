package controller

import (
	"context"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
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

// vietqrTradeNoLen is the fixed length of a VietQR trade number: the "VQR"
// prefix plus 8 random upper-case characters.
const vietqrTradeNoLen = 11

// vietqrMemo builds the transfer memo. VietQR caps `content` at 23 characters
// and Latin-only, so the prefix stays short; tradeNo already carries the user
// id and is unique, which is what the callback matches on.
func vietqrMemo(tradeNo string) string {
	return "DGAI" + tradeNo
}

// vietqrNormalizeMemo makes a transfer description comparable: banks add
// spaces and case freely, and some apps append the payer's own note.
func vietqrNormalizeMemo(content string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(content) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
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
	// SePay reconciles the same QR order, so a bank-transfer top-up is usable
	// when either channel is on.
	return (setting.VietQREnabled || isSePayEnabled()) &&
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

	// Fixed 11-char trade number so the memo can be split back apart even when a
	// bank app appends its own note. Uppercase because bank apps may upper-case
	// the description, and the callback is matched against the memo.
	tradeNo := "VQR" + strings.ToUpper(common.GetRandomString(8))
	memo := vietqrMemo(tradeNo)
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

	// Prefer a QR minted by VietQR: it registers this order id on their side,
	// which is what lets the customer's "check payment" find the transfer.
	qrImageURL := vietqrQRImageURL(vndAmount, memo)
	qrContent := ""
	if qr, content, err := vietqrCreateOrderQR(c.Request.Context(), tradeNo, vndAmount); err != nil {
		logger.LogError(c.Request.Context(), common.LogText("vietqr generate-customer failed trade_no=%s error=%q", tradeNo, err.Error()))
	} else {
		// VietQR prefixes its own reference to the content it expects back, and
		// only its own QR is registered for the check-payment lookup.
		qrContent = qr
		memo = content
	}

	common.ApiSuccess(c, gin.H{
		"trade_no":     tradeNo,
		"amount":       req.Amount,
		"pay_money":    payMoney,
		"memo":         memo,
		"qr_image_url": qrImageURL,
		"qr_content":   qrContent,
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
	// The sync pair is what we issue to VietQR; the merchant pair is what they
	// issue to us. Reusing one for both lets VietQR authenticate to our own
	// token endpoint, so the merchant pair must never be accepted here.
	if setting.VietQRUsername != "" &&
		strings.EqualFold(setting.VietQRSyncUsername, setting.VietQRUsername) {
		logger.LogError(c.Request.Context(), common.LogText("vietqr sync credentials collide with the merchant credentials; reconfigure VietQRSyncUsername"))
		c.JSON(http.StatusUnauthorized, gin.H{"status": "FAILED", "message": "sync credentials are misconfigured"})
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

// vietqrAPIToken fetches a bearer token for VietQR's own APIs. This is the
// merchant credential pair VietQR issued to us, not the pair we issue them.
func vietqrAPIToken(ctx context.Context) (string, error) {
	if setting.VietQRUsername == "" || setting.VietQRPassword == "" {
		return "", errors.New("vietqr api credentials not configured")
	}
	credentials := base64.StdEncoding.EncodeToString([]byte(setting.VietQRUsername + ":" + setting.VietQRPassword))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, setting.VietQRAPIBaseURL+"/vqr/api/token_generate", strings.NewReader("{}"))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Basic "+credentials)

	client := &http.Client{Timeout: 20 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 8<<10))
	if err != nil {
		return "", err
	}
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("vietqr token endpoint returned %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	var token struct {
		AccessToken string `json:"access_token"`
	}
	if err := common.Unmarshal(body, &token); err != nil {
		return "", err
	}
	if token.AccessToken == "" {
		return "", errors.New("vietqr token endpoint returned an empty token")
	}
	return token.AccessToken, nil
}

// vietqrCheckOrderStatus asks VietQR whether the order has been paid. Status 1
// means paid; 0 is still pending; 2 is expired. A missing transaction returns
// E96, which is reported as pending so the caller can retry.
func vietqrCheckOrderStatus(ctx context.Context, orderId string) (int, error) {
	token, err := vietqrAPIToken(ctx)
	if err != nil {
		return 0, err
	}
	checkSum := fmt.Sprintf("%x", md5.Sum([]byte(setting.VietQRBankAccount+setting.VietQRUsername)))
	payload, err := common.Marshal(map[string]string{
		"bankAccount": setting.VietQRBankAccount,
		"type":        "0",
		"value":       orderId,
		"checkSum":    checkSum,
	})
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, setting.VietQRAPIBaseURL+"/vqr/api/ecommerce-transactions/check-order", strings.NewReader(string(payload)))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 20 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 64<<10))
	if err != nil {
		return 0, err
	}
	if res.StatusCode != http.StatusOK {
		bodyText := strings.TrimSpace(string(body))
		// E96 is "transaction not found": the customer has not transferred yet,
		// which is a pending state rather than a failure.
		if strings.Contains(bodyText, "E96") {
			return 0, nil
		}
		return 0, fmt.Errorf("vietqr check-order returned %d: %s", res.StatusCode, bodyText)
	}
	var orders []struct {
		Status int `json:"status"`
	}
	if err := common.Unmarshal(body, &orders); err != nil {
		return 0, err
	}
	if len(orders) == 0 {
		return 0, nil
	}
	return orders[0].Status, nil
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

	// VietQR only echoes orderId for QRs minted through its own generate API.
	// Our QR comes from img.vietqr.io, so the round-trip identifier is the memo
	// the customer typed into the transfer description. Bank apps may append
	// their own note, so read the fixed-width trade number that follows DGAI.
	var topUp *model.TopUp
	if req.OrderId != "" {
		topUp = model.GetTopUpByTradeNo(req.OrderId)
	}
	if topUp == nil {
		memo := vietqrNormalizeMemo(req.Content)
		if i := strings.Index(memo, "DGAI"); i >= 0 && len(memo) >= i+4+vietqrTradeNoLen {
			topUp = model.GetTopUpByTradeNo(memo[i+4 : i+4+vietqrTradeNoLen])
		}
	}
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

// vietqrCreateOrder QR asks VietQR to mint the QR for this order. Registering
// the order id there is what makes the customer's later "check payment" work:
// check-order only knows about QR codes VietQR itself produced. It returns the
// QR payload as a string plus the exact transfer content VietQR expects.
func vietqrCreateOrderQR(ctx context.Context, tradeNo string, vndAmount int64) (qrContent string, content string, err error) {
	token, err := vietqrAPIToken(ctx)
	if err != nil {
		return "", "", err
	}
	memo := vietqrMemo(tradeNo)
	payload, err := common.Marshal(map[string]any{
		"bankCode":     setting.VietQRBankCode,
		"bankAccount":  setting.VietQRBankAccount,
		"userBankName": setting.VietQRAccountName,
		"content":      memo,
		"qrType":       0,
		"amount":       vndAmount,
		"orderId":      tradeNo,
		"transType":    "C",
	})
	if err != nil {
		return "", "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, setting.VietQRAPIBaseURL+"/vqr/api/qr/generate-customer", strings.NewReader(string(payload)))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 20 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 64<<10))
	if err != nil {
		return "", "", err
	}
	if res.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("vietqr generate-customer returned %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	var order struct {
		QRCode  string `json:"qrCode"`
		Content string `json:"content"`
	}
	if err := common.Unmarshal(body, &order); err != nil {
		return "", "", err
	}
	if order.QRCode == "" {
		return "", "", errors.New("vietqr generate-customer returned an empty QR")
	}
	return order.QRCode, order.Content, nil
}

// VietQRConfirmPayment lets a customer ask the gateway to check their order
// against VietQR and credit it as soon as the transfer is visible. It is the
// manual fallback for the callback: the callback credits automatically, this
// endpoint covers the window where the customer already paid but the callback
// has not arrived yet.
func VietQRConfirmPayment(c *gin.Context) {
	userId := c.GetInt("id")
	var req struct {
		TradeNo string `json:"trade_no"`
	}
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		paymentError(c, common.NewMessage("Invalid parameters"))
		return
	}
	tradeNo := strings.TrimSpace(req.TradeNo)
	if tradeNo == "" {
		paymentError(c, common.NewMessage("Invalid parameters"))
		return
	}

	topUp := model.GetTopUpByTradeNo(tradeNo)
	if topUp == nil || topUp.UserId != userId || topUp.PaymentProvider != model.PaymentProviderVietQR {
		paymentError(c, common.NewMessage("Top-up order does not exist"))
		return
	}
	if topUp.Status == common.TopUpStatusSuccess {
		common.ApiSuccess(c, gin.H{"status": "success", "trade_no": topUp.TradeNo})
		return
	}
	if topUp.Status != common.TopUpStatusPending {
		paymentError(c, common.NewMessage("Order is not pending payment and cannot be completed"))
		return
	}

	// SePay reconciles the bank account directly, so a paid transfer is credited
	// by its webhook within seconds. VietQR's own check-order only knows QR codes
	// it minted on a linked account, which excludes most banks, so polling it
	// here would only tell a paying customer to keep waiting.
	if isSePayEnabled() {
		common.ApiSuccess(c, gin.H{
			"status":   "pending",
			"trade_no": topUp.TradeNo,
			"reason":   "manual",
		})
		return
	}

	status, err := vietqrCheckOrderStatus(c.Request.Context(), topUp.TradeNo)
	if err != nil {
		logger.LogError(c.Request.Context(), common.LogText("vietqr check-order failed trade_no=%s error=%q", tradeNo, err.Error()))
		paymentError(c, common.NewMessage("Failed to query the transaction, please try again later"))
		return
	}
	if status != 1 {
		// Sandbox only sees transactions minted in the test system, so a real
		// bank transfer can never be confirmed there. Say so instead of telling
		// the customer to keep waiting.
		if setting.VietQRSandbox {
			common.ApiSuccess(c, gin.H{
				"status":   "pending",
				"trade_no": topUp.TradeNo,
				"reason":   "sandbox",
			})
			return
		}
		// Still waiting for the bank, or no transaction yet.
		common.ApiSuccess(c, gin.H{"status": "pending", "trade_no": topUp.TradeNo})
		return
	}

	LockOrder(topUp.TradeNo)
	defer UnlockOrder(topUp.TradeNo)
	if err := model.ManualCompleteTopUp(topUp.TradeNo, c.ClientIP()); err != nil {
		logger.LogError(c.Request.Context(), common.LogText("vietqr confirm failed trade_no=%s error=%q", tradeNo, err.Error()))
		paymentError(c, common.NewMessage("Failed to credit the top-up"))
		return
	}
	common.ApiSuccess(c, gin.H{"status": "success", "trade_no": topUp.TradeNo})
}

// StartVietQRSettlementWorker credits pending VietQR orders by polling the
// gateway's own check-order API. It covers the window where VietQR's callback
// is delayed or has not been configured on their side yet, which is exactly
// when a customer has paid but their wallet is still empty.
//
// Sandbox is skipped: its check-order only knows QR codes minted in the test
// system, so polling it would never settle a real transfer.
func StartVietQRSettlementWorker(interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		if setting.VietQRSandbox || setting.VietQRUsername == "" || setting.VietQRPassword == "" {
			continue
		}
		// SePay pushes every bank transaction itself; polling VietQR would only
		// re-ask an account it does not track.
		if isSePayEnabled() {
			continue
		}
		settlePendingVietQROrders()
	}
}

// settlePendingVietQROrders walks the pending orders that are still inside
// their payment window and credits every one the gateway reports as paid.
func settlePendingVietQROrders() {
	now := time.Now().Unix()
	orders, err := model.GetPendingVietQROrders(now - 2*3600)
	if err != nil {
		common.SysError(common.LogText("failed to list pending vietqr orders: %s", err.Error()))
		return
	}
	for _, order := range orders {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		status, err := vietqrCheckOrderStatus(ctx, order.TradeNo)
		cancel()
		if err != nil {
			logger.LogError(context.Background(), common.LogText("vietqr poll check-order failed trade_no=%s error=%q", order.TradeNo, err.Error()))
			continue
		}
		if status != 1 {
			continue
		}
		LockOrder(order.TradeNo)
		err = model.ManualCompleteTopUp(order.TradeNo, "vietqr-poller")
		UnlockOrder(order.TradeNo)
		if err != nil {
			logger.LogError(context.Background(), common.LogText("vietqr poll credit failed trade_no=%s error=%q", order.TradeNo, err.Error()))
			continue
		}
		common.SysLog(common.LogText("vietqr poll credited trade_no=%s user_id=%d", order.TradeNo, order.UserId))
	}
}
