package controller

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"math"
	"net/http"
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

// sePayWebhookRequest is the transaction payload SePay posts. Field tags follow
// their camelCase wire format; only the fields the matcher needs are bound.
type sePayWebhookRequest struct {
	Id             int64   `json:"id"`
	Gateway        string  `json:"gateway"`
	AccountNumber  string  `json:"accountNumber"`
	SubAccount     string  `json:"subAccount"`
	Code           *string `json:"code"`
	Content        string  `json:"content"`
	TransferType   string  `json:"transferType"`
	Description    string  `json:"description"`
	TransferAmount float64 `json:"transferAmount"`
	ReferenceCode  string  `json:"referenceCode"`
}

// isSePayEnabled gates the webhook. Only the secret key matters: it is what
// verifies the signature, while the merchant ID belongs to SePay's checkout
// gateway and is stored for reference only.
func isSePayEnabled() bool {
	return setting.SePayEnabled && strings.TrimSpace(setting.SePaySecretKey) != ""
}

// sePaySignatureValid verifies SePay's HMAC-SHA256 signature over
// "{timestamp}.{raw_body}". SePay signs the raw bytes, so the caller must pass
// the body exactly as read from the request.
func sePaySignatureValid(timestamp string, signature string, body []byte) bool {
	if timestamp == "" || signature == "" {
		return false
	}
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}
	tolerance := int64(setting.SePayToleranceSeconds)
	if tolerance <= 0 {
		tolerance = 300
	}
	if math.Abs(float64(time.Now().Unix()-ts)) > float64(tolerance) {
		return false
	}
	mac := hmac.New(sha256.New, []byte(setting.SePaySecretKey))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(body)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

// SePayWebhook credits a pending bank-transfer top-up when SePay reports the
// matching bank transaction. SePay watches the receiving account directly, so
// it reconciles the banks VietQR cannot link (TPBank and most others).
//
// SePay counts a webhook as delivered only on HTTP 200/201 with exactly
// {"success": true}; anything else is retried. Replays are safe because
// ManualCompleteTopUp is idempotent on an already-succeeded order.
func SePayWebhook(c *gin.Context) {
	ctx := c.Request.Context()
	if !isSePayEnabled() {
		logger.LogWarn(ctx, common.LogText("sepay webhook rejected reason=disabled path=%q client_ip=%s", c.Request.RequestURI, c.ClientIP()))
		c.JSON(http.StatusForbidden, gin.H{"success": false})
		return
	}

	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
	if err != nil {
		logger.LogError(ctx, common.LogText("sepay webhook failed to read body error=%q", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"success": false})
		return
	}

	if !sePaySignatureValid(c.GetHeader("X-SePay-Timestamp"), c.GetHeader("X-SePay-Signature"), body) {
		logger.LogWarn(ctx, common.LogText("sepay webhook signature verification failed client_ip=%s", c.ClientIP()))
		c.JSON(http.StatusUnauthorized, gin.H{"success": false})
		return
	}

	var req sePayWebhookRequest
	if err := common.Unmarshal(body, &req); err != nil {
		logger.LogError(ctx, common.LogText("sepay webhook malformed payload error=%q body=%q", err.Error(), string(body)))
		c.JSON(http.StatusOK, gin.H{"success": true})
		return
	}

	// Only incoming money is a top-up; outgoing transfers are not ours.
	if req.TransferType != "in" {
		c.JSON(http.StatusOK, gin.H{"success": true})
		return
	}

	topUp := matchSePayTopUp(req)
	if topUp == nil {
		// A transfer that matches no order is still a successful delivery:
		// returning an error would make SePay retry it forever.
		logger.LogWarn(ctx, common.LogText("sepay webhook no matching top-up transaction_id=%d content=%q amount=%.0f", req.Id, req.Content, req.TransferAmount))
		c.JSON(http.StatusOK, gin.H{"success": true})
		return
	}

	expectedVND := decimal.NewFromInt(vietqrVNDAmount(topUp.Money))
	if decimal.NewFromFloat(req.TransferAmount).LessThan(expectedVND) {
		logger.LogWarn(ctx, common.LogText("sepay webhook amount below order trade_no=%s expected_vnd=%s received_vnd=%.0f", topUp.TradeNo, expectedVND.String(), req.TransferAmount))
		c.JSON(http.StatusOK, gin.H{"success": true})
		return
	}

	LockOrder(topUp.TradeNo)
	defer UnlockOrder(topUp.TradeNo)
	if err := model.ManualCompleteTopUp(topUp.TradeNo, c.ClientIP()); err != nil {
		logger.LogError(ctx, common.LogText("sepay webhook credit failed trade_no=%s error=%q", topUp.TradeNo, err.Error()))
		// A failed credit must be retried, so report it as an error.
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}

	common.SysLog(common.LogText("sepay webhook credited trade_no=%s transaction_id=%d", topUp.TradeNo, req.Id))
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// matchSePayTopUp finds the pending order for a transaction. SePay returns the
// configured payment code separately from the raw content, so both are tried.
func matchSePayTopUp(req sePayWebhookRequest) *model.TopUp {
	if req.Code != nil {
		topUp := model.GetTopUpByTradeNo(strings.TrimSpace(*req.Code))
		if topUp != nil && topUp.PaymentProvider == model.PaymentProviderVietQR {
			return topUp
		}
	}
	memo := vietqrNormalizeMemo(req.Content)
	i := strings.Index(memo, "DGAI")
	if i < 0 || len(memo) < i+4+vietqrTradeNoLen {
		return nil
	}
	topUp := model.GetTopUpByTradeNo(memo[i+4 : i+4+vietqrTradeNoLen])
	if topUp == nil || topUp.PaymentProvider != model.PaymentProviderVietQR {
		return nil
	}
	return topUp
}
