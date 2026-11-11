package controller

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func signSePay(secret string, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestSePaySignatureValid(t *testing.T) {
	previous := setting.SePaySecretKey
	previousTolerance := setting.SePayToleranceSeconds
	t.Cleanup(func() {
		setting.SePaySecretKey = previous
		setting.SePayToleranceSeconds = previousTolerance
	})

	setting.SePaySecretKey = "spsk_live_test"
	setting.SePayToleranceSeconds = 300
	body := []byte(`{"id":1,"transferType":"in","transferAmount":1000}`)
	now := strconv.FormatInt(time.Now().Unix(), 10)
	valid := signSePay("spsk_live_test", now, body)

	cases := []struct {
		name      string
		timestamp string
		signature string
		body      []byte
		want      bool
	}{
		{"current window", now, valid, body, true},
		{"tampered body", now, valid, []byte(`{"id":1,"transferType":"in","transferAmount":9999}`), false},
		{"wrong secret", now, signSePay("spsk_live_other", now, body), body, false},
		{"stale timestamp", strconv.FormatInt(time.Now().Add(-time.Hour).Unix(), 10), valid, body, false},
		{"missing signature", now, "", body, false},
		{"non-numeric timestamp", "abc", valid, body, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sePaySignatureValid(tc.timestamp, tc.signature, tc.body); got != tc.want {
				t.Fatalf("sePaySignatureValid() = %v, want %v", got, tc.want)
			}
		})
	}
}

func useSePayTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousType := common.MainDatabaseType()
	previousRedis, previousMemory := common.RedisEnabled, common.MemoryCacheEnabled
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.TopUp{}, &model.User{}, &model.Log{}))
	model.DB = db
	// The credit path writes a top-up log; make it share the same handle.
	model.LOG_DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	common.RedisEnabled, common.MemoryCacheEnabled = false, false
	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.SetMainDatabaseType(previousType)
		common.RedisEnabled, common.MemoryCacheEnabled = previousRedis, previousMemory
	})
	return db
}

func TestMatchSePayTopUp(t *testing.T) {
	db := useSePayTestDB(t)
	order := &model.TopUp{
		UserId:          1,
		Amount:          5,
		Money:           5,
		TradeNo:         "VQRABCD1234",
		PaymentMethod:   model.PaymentMethodVietQR,
		PaymentProvider: model.PaymentProviderVietQR,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}
	require.NoError(t, db.Create(order).Error)

	code := "VQRABCD1234"
	cases := []struct {
		name string
		req  sePayWebhookRequest
		want string
	}{
		{
			name: "matches the payment code",
			req:  sePayWebhookRequest{Code: &code},
			want: order.TradeNo,
		},
		{
			name: "matches the memo when the bank adds spaces and case",
			req:  sePayWebhookRequest{Content: "dgai vqr abcd 1234 chuyen tien"},
			want: order.TradeNo,
		},
		{
			name: "ignores a transfer with no matching memo",
			req:  sePayWebhookRequest{Content: "thanh toan tien hang"},
			want: "",
		},
		{
			name: "ignores a memo for an unknown order",
			req:  sePayWebhookRequest{Content: "DGAI VQRZZZZ9999"},
			want: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			topUp := matchSePayTopUp(tc.req)
			if tc.want == "" {
				if topUp != nil {
					t.Fatalf("matchSePayTopUp() = %q, want no match", topUp.TradeNo)
				}
				return
			}
			require.NotNil(t, topUp)
			require.Equal(t, tc.want, topUp.TradeNo)
		})
	}
}

// TestSePayWebhookCreditsMatchingTransfer exercises the full handler: a signed
// bank notification must credit the pending order exactly once, and a replayed
// delivery must not credit it twice.
func TestSePayWebhookCreditsMatchingTransfer(t *testing.T) {
	db := useSePayTestDB(t)

	previousEnabled := setting.SePayEnabled
	previousSecret := setting.SePaySecretKey
	previousRate := setting.VietQRVNDRate
	previousQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() {
		setting.SePayEnabled = previousEnabled
		setting.SePaySecretKey = previousSecret
		setting.VietQRVNDRate = previousRate
		common.QuotaPerUnit = previousQuotaPerUnit
	})
	setting.SePayEnabled = true
	setting.SePaySecretKey = "spsk_live_test"
	setting.VietQRVNDRate = 25000
	common.QuotaPerUnit = 500000

	user := &model.User{Username: "sepay", AffCode: "sepay001"}
	require.NoError(t, user.Insert(0))
	order := &model.TopUp{
		UserId:          user.Id,
		Amount:          5,
		Money:           5,
		TradeNo:         "VQRPAID0001",
		PaymentMethod:   model.PaymentMethodVietQR,
		PaymentProvider: model.PaymentProviderVietQR,
		CreateTime:      common.GetTimestamp(),
		Status:          common.TopUpStatusPending,
	}
	require.NoError(t, db.Create(order).Error)

	body := []byte(`{"id":42,"gateway":"TPBank","transactionDate":"2026-11-12 10:00:00","accountNumber":"10006460824","subAccount":"","code":null,"content":"DGAI VQRPAID0001 chuyen tien","transferType":"in","description":"NGUYEN VAN A","transferAmount":125000,"accumulated":0,"referenceCode":"FT1"}`)

	deliver := func() *httptest.ResponseRecorder {
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodPost, "/api/sepay/webhook", bytes.NewReader(body))
		ctx.Request.Header.Set("X-SePay-Timestamp", timestamp)
		ctx.Request.Header.Set("X-SePay-Signature", signSePay("spsk_live_test", timestamp, body))
		SePayWebhook(ctx)
		return recorder
	}

	first := deliver()
	require.Equal(t, http.StatusOK, first.Code)
	require.JSONEq(t, `{"success":true}`, first.Body.String())

	var credited model.User
	require.NoError(t, db.First(&credited, user.Id).Error)
	require.EqualValues(t, 5*common.QuotaPerUnit, credited.Quota, "the order amount is credited once")

	// SePay retries the same transaction; the order is already settled.
	second := deliver()
	require.Equal(t, http.StatusOK, second.Code)
	require.NoError(t, db.First(&credited, user.Id).Error)
	require.EqualValues(t, 5*common.QuotaPerUnit, credited.Quota, "a replay must not credit twice")
}
