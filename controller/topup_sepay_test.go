package controller

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"

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
	previousDB := model.DB
	previousType := common.MainDatabaseType()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.TopUp{}))
	model.DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		model.DB = previousDB
		common.SetMainDatabaseType(previousType)
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
