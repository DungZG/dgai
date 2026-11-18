package controller

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/cryptomus"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	cryptoTestKey      = "test-payment-key"
	cryptoTestMerchant = "8b03432e-385b-4670-8d06-064591096795"
	cryptoTestUUID     = "62f88b36-a9d5-4fa6-aa26-e040c3dbf26d"
	cryptoTestTrade    = "CRYTESTORDER1234567890123456789"
)

// The unsigned body mirrors PHP json_encode(..., JSON_UNESCAPED_UNICODE):
// UTF-8 kept raw, slashes escaped. The sign below was computed independently
// with base64(JSON)+API key -> MD5.
const cryptoWebhookUnsigned = `{"type":"payment","uuid":"62f88b36-a9d5-4fa6-aa26-e040c3dbf26d","order_id":"CRYTESTORDER1234567890123456789","amount":"10.00","payment_amount":"10.00","payment_amount_usd":"10.00","merchant_amount":"9.70","commission":"0.30","is_final":true,"status":"paid","network":"tron","currency":"USD","payer_currency":"USDT","payer_amount":"10.00","additional_data":"Gói Việt\/法","txid":"someTxidWith\/Slash"}`

func cryptoWebhookBody(unsigned, sign string) string {
	return unsigned[:len(unsigned)-1] + `,"sign":"` + sign + `"}`
}

func cryptoSign(value, key string) string {
	sum := md5.Sum([]byte(base64.StdEncoding.EncodeToString([]byte(value)) + key))
	return hex.EncodeToString(sum[:])
}

func TestCryptomusWebhookSignatureVerification(t *testing.T) {
	body := cryptoWebhookBody(cryptoWebhookUnsigned, cryptoSign(cryptoWebhookUnsigned, cryptoTestKey))
	payment, err := cryptomus.VerifyWebhook([]byte(body), cryptoTestKey)
	require.NoError(t, err)
	assert.Equal(t, cryptoTestTrade, payment.OrderID)
	assert.Equal(t, "10.00", payment.Amount)
	assert.Equal(t, "USD", payment.Currency)
	assert.Equal(t, "USDT", payment.PayerCurrency)
	assert.Equal(t, "tron", payment.Network)
	assert.Equal(t, "paid", payment.Status)
	assert.True(t, payment.IsFinal)

	cases := map[string]struct {
		body []byte
		key  string
	}{
		"tampered amount": {[]byte(strings.Replace(body, `"amount":"10.00"`, `"amount":"99.00"`, 1)), cryptoTestKey},
		"wrong key":       {[]byte(body), "other-key"},
		"duplicate key":   {[]byte(strings.Replace(body, `"sign":"`, `"status":"paid","sign":"`, 1)), cryptoTestKey},
		"trailing json":   {[]byte(body + `{}`), cryptoTestKey},
		"missing sign":    {[]byte(cryptoWebhookUnsigned), cryptoTestKey},
		"empty key":       {[]byte(body), ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := cryptomus.VerifyWebhook(tc.body, tc.key)
			assert.ErrorIs(t, err, cryptomus.ErrProtocol)
		})
	}
}

type cryptoTransport func(*http.Request) (*http.Response, error)

func (f cryptoTransport) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

func cryptoHTTPResponse(body string, status int) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}
}

func TestCryptomusClientSignsProtocolRequests(t *testing.T) {
	var paths, bodies, merchants, signs []string
	transport := cryptoTransport(func(request *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		paths = append(paths, request.URL.Path)
		bodies = append(bodies, string(body))
		merchants = append(merchants, request.Header.Get("merchant"))
		signs = append(signs, request.Header.Get("sign"))
		if request.URL.Path == "/v1/payment" {
			return cryptoHTTPResponse(`{"state":0,"result":{"uuid":"`+cryptoTestUUID+`","order_id":"`+cryptoTestTrade+`","amount":"10.00","currency":"USD","payer_currency":"USDT","network":"tron","status":"check","payment_status":"check","url":"https://pay.cryptomus.com/pay/`+cryptoTestUUID+`","is_final":false,"expired_at":123}}`, http.StatusOK), nil
		}
		return cryptoHTTPResponse(`{"state":0,"result":{"uuid":"`+cryptoTestUUID+`","order_id":"`+cryptoTestTrade+`","amount":"10.00","currency":"USD","status":"paid","payment_status":"paid","is_final":true}}`, http.StatusOK), nil
	})
	client := cryptomus.Client{MerchantID: cryptoTestMerchant, PaymentAPIKey: cryptoTestKey, HTTPClient: &http.Client{Transport: transport}}

	created, err := client.Create(context.Background(), cryptomus.CreateRequest{Amount: "10.00", Currency: "USD", OrderID: cryptoTestTrade, Network: "tron", ToCurrency: "USDT", Currencies: []cryptomus.Currency{{Currency: "USDT", Network: "tron"}}, URLCallback: "https://example.com/api/cryptomus/webhook", URLReturn: "https://example.com/wallet", URLSuccess: "https://example.com/wallet", AccuracyPaymentPercent: 0, Subtract: 0, Lifetime: 3600, IsPaymentMultiple: true})
	require.NoError(t, err)
	assert.Equal(t, "https://pay.cryptomus.com/pay/"+cryptoTestUUID, created.URL)
	assert.Equal(t, cryptoTestTrade, created.OrderID)

	info, err := client.Info(context.Background(), cryptoTestTrade)
	require.NoError(t, err)
	assert.Equal(t, "paid", info.Status)

	require.Len(t, paths, 2)
	assert.Equal(t, "/v1/payment", paths[0])
	assert.Equal(t, "/v1/payment/info", paths[1])
	for i := range paths {
		assert.Equal(t, cryptoTestMerchant, merchants[i])
		assert.Equal(t, cryptoSign(bodies[i], cryptoTestKey), signs[i])
	}
	assert.Contains(t, bodies[0], `"to_currency":"USDT"`)
	assert.Contains(t, bodies[0], `"accuracy_payment_percent":0`)
	assert.Contains(t, bodies[0], `"subtract":0`)
	assert.JSONEq(t, `{"order_id":"`+cryptoTestTrade+`"}`, bodies[1])
}

func TestCryptomusClientRejectsUntrustedResponses(t *testing.T) {
	cases := map[string]struct {
		status int
		body   string
	}{
		"http error":      {http.StatusBadGateway, `{"state":0,"result":{}}`},
		"provider error":  {http.StatusOK, `{"state":1,"errors":{"amount":["validation.required"]}}`},
		"untrusted host":  {http.StatusOK, `{"state":0,"result":{"uuid":"` + cryptoTestUUID + `","order_id":"x","url":"https://evil.example/pay/` + cryptoTestUUID + `"}}`},
		"plain http":      {http.StatusOK, `{"state":0,"result":{"uuid":"` + cryptoTestUUID + `","order_id":"x","url":"http://pay.cryptomus.com/pay/` + cryptoTestUUID + `"}}`},
		"query on url":    {http.StatusOK, `{"state":0,"result":{"uuid":"` + cryptoTestUUID + `","order_id":"x","url":"https://pay.cryptomus.com/pay/` + cryptoTestUUID + `?next=evil"}}`},
		"missing invoice": {http.StatusOK, `{"state":0,"result":{"order_id":"x"}}`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			client := cryptomus.Client{MerchantID: cryptoTestMerchant, PaymentAPIKey: cryptoTestKey, HTTPClient: &http.Client{Transport: cryptoTransport(func(*http.Request) (*http.Response, error) {
				return cryptoHTTPResponse(tc.body, tc.status), nil
			})}}
			_, err := client.Info(context.Background(), cryptoTestTrade)
			assert.ErrorIs(t, err, cryptomus.ErrProtocol)
		})
	}
}

func useCryptoTestDB(t *testing.T, kind, dsn string) *gorm.DB {
	t.Helper()
	var db *gorm.DB
	if kind == "sqlite" {
		opened, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
		require.NoError(t, err)
		db = opened
	} else {
		opened, _ := newAuditTestDatabase(t, kind, dsn)
		db = opened
	}
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousType := common.MainDatabaseType()
	previousRedis, previousMemory := common.RedisEnabled, common.MemoryCacheEnabled
	model.DB, model.LOG_DB = db, db
	common.SetMainDatabaseType(common.DatabaseType(kind))
	common.RedisEnabled, common.MemoryCacheEnabled = false, false
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.TopUp{}, &model.CryptoPayment{}, &model.Log{}))
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.SetMainDatabaseType(previousType)
		common.RedisEnabled, common.MemoryCacheEnabled = previousRedis, previousMemory
	})
	return db
}

func useCryptoTestConfig(t *testing.T) {
	t.Helper()
	confirmPaymentComplianceForTest(t)
	previous := struct {
		enabled                bool
		merchant, key, network string
		min, max               int
		address                string
		display                string
		customRate, usdRate    float64
		quotaPerUnit           float64
		referral               float64
		newUser                int
		tiers                  []operation_setting.TopUpBonusTier
	}{
		enabled: setting.CryptoEnabled, merchant: setting.CryptomusMerchantID, key: setting.CryptomusPaymentAPIKey,
		network: setting.CryptoNetwork, min: setting.CryptoMinTopUp, max: setting.CryptoMaxTopUp,
		address: system_setting.ServerAddress, display: operation_setting.GetGeneralSetting().QuotaDisplayType,
		customRate: operation_setting.GetGeneralSetting().CustomCurrencyExchangeRate, usdRate: operation_setting.USDExchangeRate,
		quotaPerUnit: common.QuotaPerUnit, referral: setting.ReferralCommissionPercent, newUser: common.QuotaForNewUser,
		tiers: operation_setting.GetPaymentSetting().TopUpBonusTiers,
	}
	setting.CryptoEnabled = true
	setting.CryptomusMerchantID = cryptoTestMerchant
	setting.CryptomusPaymentAPIKey = cryptoTestKey
	setting.CryptoNetwork = "tron"
	setting.CryptoMinTopUp, setting.CryptoMaxTopUp = 10, 100
	system_setting.ServerAddress = "https://dashboard.example.com"
	operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeUSD
	operation_setting.GetGeneralSetting().CustomCurrencyExchangeRate = 1
	operation_setting.USDExchangeRate = 7.3
	common.QuotaPerUnit = 500000
	setting.ReferralCommissionPercent = 0
	common.QuotaForNewUser = 0
	operation_setting.GetPaymentSetting().TopUpBonusTiers = nil
	t.Cleanup(func() {
		setting.CryptoEnabled = previous.enabled
		setting.CryptomusMerchantID, setting.CryptomusPaymentAPIKey = previous.merchant, previous.key
		setting.CryptoNetwork = previous.network
		setting.CryptoMinTopUp, setting.CryptoMaxTopUp = previous.min, previous.max
		system_setting.ServerAddress = previous.address
		operation_setting.GetGeneralSetting().QuotaDisplayType = previous.display
		operation_setting.GetGeneralSetting().CustomCurrencyExchangeRate = previous.customRate
		operation_setting.USDExchangeRate = previous.usdRate
		common.QuotaPerUnit = previous.quotaPerUnit
		setting.ReferralCommissionPercent = previous.referral
		common.QuotaForNewUser = previous.newUser
		operation_setting.GetPaymentSetting().TopUpBonusTiers = previous.tiers
	})
}

func TestCryptomusAmountUsesWalletDisplayUnits(t *testing.T) {
	useCryptoTestConfig(t)
	general := operation_setting.GetGeneralSetting()

	quota, usd, err := cryptoAmount(10)
	require.NoError(t, err)
	assert.Equal(t, "10", usd)
	assert.Equal(t, 5000000, quota)

	general.QuotaDisplayType = operation_setting.QuotaDisplayTypeCNY
	quota, usd, err = cryptoAmount(73)
	require.NoError(t, err)
	assert.Equal(t, "10", usd)
	assert.Equal(t, 5000000, quota)

	general.QuotaDisplayType = operation_setting.QuotaDisplayTypeCustom
	general.CustomCurrencyExchangeRate = 2
	quota, usd, err = cryptoAmount(20)
	require.NoError(t, err)
	assert.Equal(t, "10", usd)
	assert.Equal(t, 5000000, quota)

	general.QuotaDisplayType = operation_setting.QuotaDisplayTypeTokens
	quota, usd, err = cryptoAmount(5000000)
	require.NoError(t, err)
	assert.Equal(t, "10", usd)
	assert.Equal(t, 5000000, quota)

	for _, amount := range []int64{0, -1, 9, 101} {
		_, _, err := cryptoAmount(amount)
		assert.Error(t, err, "amount %d", amount)
	}
}

func cryptoContext(t *testing.T, method, target string, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(method, target, bytes.NewBufferString(body))
	return context, recorder
}

func TestRequestCryptomusPayRejectsInvalidRequests(t *testing.T) {
	useCryptoTestConfig(t)

	setting.CryptoEnabled = false
	context, recorder := cryptoContext(t, http.MethodPost, "/api/user/cryptomus/pay", `{"amount":10}`)
	context.Set("id", 1)
	RequestCryptomusPay(context)
	assert.Contains(t, recorder.Body.String(), "Payment method is not available")

	setting.CryptoEnabled = true
	for _, amount := range []int64{0, 9, 101} {
		context, recorder = cryptoContext(t, http.MethodPost, "/api/user/cryptomus/pay", `{"amount":`+decimalString(amount)+`}`)
		context.Set("id", 1)
		RequestCryptomusPay(context)
		assert.Contains(t, recorder.Body.String(), "Invalid top-up amount", "amount %d", amount)
	}

	context, recorder = cryptoContext(t, http.MethodPost, "/api/user/cryptomus/pay", `{`)
	context.Set("id", 1)
	RequestCryptomusPay(context)
	assert.Contains(t, recorder.Body.String(), "Invalid parameters")
}

func decimalString(value int64) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	var digits []byte
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	if negative {
		return "-" + string(digits)
	}
	return string(digits)
}

func TestGetTopUpInfoExposesCryptomus(t *testing.T) {
	useCryptoTestConfig(t)
	context, recorder := cryptoContext(t, http.MethodGet, "/api/user/topup/info", "")
	GetTopUpInfo(context)
	var response struct {
		Success bool `json:"success"`
		Data    struct {
			EnableCrypto bool                `json:"enable_crypto_topup"`
			CryptoMin    int                 `json:"crypto_min_topup"`
			CryptoMax    int                 `json:"crypto_max_topup"`
			PayMethods   []map[string]string `json:"pay_methods"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	assert.True(t, response.Data.EnableCrypto)
	assert.Equal(t, 10, response.Data.CryptoMin)
	assert.Equal(t, 100, response.Data.CryptoMax)
	found := false
	for _, method := range response.Data.PayMethods {
		if method["type"] == model.PaymentMethodCryptomus {
			found = true
			assert.Equal(t, "USDT", method["name"])
			assert.Equal(t, "10", method["min_topup"])
		}
	}
	assert.True(t, found)
}

func cryptoOrderFixture(t *testing.T, db *gorm.DB, userId int, tradeNo string, base, bonus int) *model.CryptoPayment {
	t.Helper()
	order := &model.TopUp{UserId: userId, Amount: 10, Money: 10, TradeNo: tradeNo, PaymentMethod: model.PaymentMethodCryptomus, PaymentProvider: model.PaymentProviderCryptomus, CreateTime: common.GetTimestamp(), Status: common.TopUpStatusPending}
	payment := &model.CryptoPayment{TradeNo: tradeNo, UserId: userId, AmountUSD: "10.00", BaseQuota: base, BonusQuota: bonus, Network: "tron", Currency: "USDT", ExpiresAt: common.GetTimestamp() + 3600, NextCheckAt: 123, CreateTime: common.GetTimestamp()}
	require.NoError(t, model.CreateCryptoTopUp(order, payment))
	return payment
}

func TestCryptomusWebhookQueuesWithoutCrediting(t *testing.T) {
	db := useCryptoTestDB(t, "sqlite", "")
	previousKey := setting.CryptomusPaymentAPIKey
	previousEnabled := setting.CryptoEnabled
	setting.CryptomusPaymentAPIKey = cryptoTestKey
	setting.CryptoEnabled = false
	t.Cleanup(func() {
		setting.CryptomusPaymentAPIKey = previousKey
		setting.CryptoEnabled = previousEnabled
	})
	body := cryptoWebhookBody(cryptoWebhookUnsigned, cryptoSign(cryptoWebhookUnsigned, cryptoTestKey))

	// Unknown orders are acknowledged and ignored, so the provider stops retrying.
	context, recorder := cryptoContext(t, http.MethodPost, "/api/cryptomus/webhook", body)
	CryptomusWebhook(context)
	assert.Equal(t, http.StatusOK, recorder.Code)

	user := cryptoTestUser(t, db, "crypto-webhook-user", 0, 0)
	cryptoOrderFixture(t, db, user.Id, cryptoTestTrade, 10000, 2000)

	context, recorder = cryptoContext(t, http.MethodPost, "/api/cryptomus/webhook", body)
	CryptomusWebhook(context)
	require.Equal(t, http.StatusOK, recorder.Code)
	var payment model.CryptoPayment
	require.NoError(t, db.Where("trade_no = ?", cryptoTestTrade).First(&payment).Error)
	assert.Nil(t, payment.InvoiceID)
	assert.Greater(t, payment.NextCheckAt, int64(123))
	var order model.TopUp
	require.NoError(t, db.Where("trade_no = ?", cryptoTestTrade).First(&order).Error)
	assert.Equal(t, common.TopUpStatusPending, order.Status)
	var stored model.User
	require.NoError(t, db.First(&stored, user.Id).Error)
	assert.Zero(t, stored.Quota)

	tampered := strings.Replace(body, `"amount":"10.00"`, `"amount":"99.00"`, 1)
	context, _ = cryptoContext(t, http.MethodPost, "/api/cryptomus/webhook", tampered)
	CryptomusWebhook(context)
	assert.Equal(t, http.StatusUnauthorized, context.Writer.Status())

	// A Cryptomus test webhook can carry only a uuid. It is acknowledged and
	// never queued or credited.
	testEvent := `{"uuid":"e1830f1b-50fc-432e-80ec-15b58ccac867","currency":"ETH","network":"eth","status":"paid"}`
	context, recorder = cryptoContext(t, http.MethodPost, "/api/cryptomus/webhook", cryptoWebhookBody(testEvent, cryptoSign(testEvent, cryptoTestKey)))
	CryptomusWebhook(context)
	assert.Equal(t, http.StatusOK, recorder.Code)
	var paymentCount int64
	require.NoError(t, db.Model(&model.CryptoPayment{}).Where("next_check_at > ?", int64(123)).Count(&paymentCount).Error)
	assert.EqualValues(t, 1, paymentCount)
}

func TestGetCryptomusOrderEnforcesOwnership(t *testing.T) {
	db := useCryptoTestDB(t, "sqlite", "")
	owner := cryptoTestUser(t, db, "crypto-order-owner", 0, 0)
	other := cryptoTestUser(t, db, "crypto-order-other", 0, 0)
	cryptoOrderFixture(t, db, owner.Id, cryptoTestTrade, 10000, 2000)

	previousEnabled := setting.CryptoEnabled
	setting.CryptoEnabled = false
	t.Cleanup(func() { setting.CryptoEnabled = previousEnabled })
	context, recorder := cryptoContext(t, http.MethodGet, "/api/user/cryptomus/orders/"+cryptoTestTrade, "")
	context.Params = gin.Params{{Key: "trade_no", Value: cryptoTestTrade}}
	context.Set("id", other.Id)
	GetCryptomusOrder(context)
	assert.Contains(t, recorder.Body.String(), "Order not found")

	context, recorder = cryptoContext(t, http.MethodGet, "/api/user/cryptomus/orders/"+cryptoTestTrade, "")
	context.Params = gin.Params{{Key: "trade_no", Value: cryptoTestTrade}}
	context.Set("id", owner.Id)
	GetCryptomusOrder(context)
	assert.Contains(t, recorder.Body.String(), `"success":true`)
	assert.NotContains(t, recorder.Body.String(), "invoice_id")
	assert.NotContains(t, recorder.Body.String(), "payment_api_key")
}

func cryptoInvoiceUUID(seed string) string {
	sum := md5.Sum([]byte(seed))
	value := hex.EncodeToString(sum[:16])
	return value[0:8] + "-" + value[8:12] + "-" + value[12:16] + "-" + value[16:20] + "-" + value[20:32]
}

func cryptoPaidInfo(tradeNo, uuid string) cryptomus.Payment {
	return cryptomus.Payment{UUID: uuid, OrderID: tradeNo, Amount: "10.00", Currency: "USD", PayerCurrency: "USDT", Network: "tron", PaymentAmount: "10.00", PaymentAmountUSD: "10.00", PayerAmount: "10.00", MerchantAmount: "9.70", Commission: "0.30", Status: "paid", PaymentStatus: "paid", URL: "https://pay.cryptomus.com/pay/" + uuid, IsFinal: true}
}

func cryptoTestUser(t *testing.T, db *gorm.DB, username string, quota int, inviterId int) model.User {
	t.Helper()
	sum := md5.Sum([]byte(username))
	user := model.User{Username: username, AffCode: hex.EncodeToString(sum[:]), Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Quota: quota, InviterId: inviterId}
	require.NoError(t, db.Create(&user).Error)
	return user
}

func TestCryptomusSettlementMatrix(t *testing.T) {
	for _, dialect := range []struct{ kind, env string }{{"sqlite", ""}, {"mysql", "TEST_MYSQL_DSN"}, {"postgres", "TEST_POSTGRES_DSN"}} {
		t.Run(dialect.kind, func(t *testing.T) {
			if dialect.env != "" && os.Getenv(dialect.env) == "" {
				t.Skip("set " + dialect.env + " to run this database")
			}
			db := useCryptoTestDB(t, dialect.kind, os.Getenv(dialect.env))
			useCryptoTestConfig(t)

			t.Run("valid credits snapshot once", func(t *testing.T) {
				trade := "CRYVALID" + dialect.kind
				invoice := cryptoInvoiceUUID(trade)
				user := cryptoTestUser(t, db, "crypto-valid-"+dialect.kind, 0, 0)
				cryptoOrderFixture(t, db, user.Id, trade, 10000, 2000)
				info := cryptoPaidInfo(trade, invoice)
				require.NoError(t, model.ApplyCryptomusPayment(info, "127.0.0.1"))
				var stored model.User
				require.NoError(t, db.First(&stored, user.Id).Error)
				assert.Equal(t, 12000, stored.Quota)
				require.NoError(t, model.ApplyCryptomusPayment(info, "127.0.0.1"))
				require.NoError(t, db.First(&stored, user.Id).Error)
				assert.Equal(t, 12000, stored.Quota)
				var payment model.CryptoPayment
				require.NoError(t, db.Where("trade_no = ?", trade).First(&payment).Error)
				require.NotNil(t, payment.InvoiceID)
				assert.Equal(t, invoice, *payment.InvoiceID)
				assert.Zero(t, payment.NextCheckAt)
			})

			t.Run("referral excludes bonus", func(t *testing.T) {
				previous := setting.ReferralCommissionPercent
				setting.ReferralCommissionPercent = 10
				t.Cleanup(func() { setting.ReferralCommissionPercent = previous })
				trade := "CRYREF" + dialect.kind
				inviter := cryptoTestUser(t, db, "crypto-inviter-"+dialect.kind, 0, 0)
				user := cryptoTestUser(t, db, "crypto-invitee-"+dialect.kind, 0, inviter.Id)
				cryptoOrderFixture(t, db, user.Id, trade, 10000, 2000)
				require.NoError(t, model.ApplyCryptomusPayment(cryptoPaidInfo(trade, cryptoInvoiceUUID(trade)), ""))
				var stored model.User
				require.NoError(t, db.First(&stored, inviter.Id).Error)
				assert.Equal(t, 1000, stored.AffQuota)
				assert.Equal(t, 1000, stored.AffHistoryQuota)
			})

			t.Run("review cases never credit", func(t *testing.T) {
				cases := map[string]func(*cryptomus.Payment){
					"amount mismatch":      func(p *cryptomus.Payment) { p.Amount = "11.00" },
					"invoice asset":        func(p *cryptomus.Payment) { p.Currency = "EUR" },
					"payer asset":          func(p *cryptomus.Payment) { p.PayerCurrency = "BTC" },
					"network mismatch":     func(p *cryptomus.Payment) { p.Network = "eth" },
					"non-final paid":       func(p *cryptomus.Payment) { p.IsFinal = false },
					"status mismatch":      func(p *cryptomus.Payment) { p.PaymentStatus = "check" },
					"planned payer amount": func(*cryptomus.Payment) {},
					"convert from":         func(p *cryptomus.Payment) { p.ConvertFrom = common.RawMessage(`{"from_currency":"BTC"}`) },
					"cancelled":            func(p *cryptomus.Payment) { p.Status, p.PaymentStatus, p.IsFinal = "cancel", "cancel", true },
				}
				index := 0
				for name, mutate := range cases {
					index++
					t.Run(name, func(t *testing.T) {
						trade := "CRYREVIEW" + dialect.kind + decimalString(int64(index))
						user := cryptoTestUser(t, db, "crypto-review-"+dialect.kind+"-"+decimalString(int64(index)), 0, 0)
						payment := cryptoOrderFixture(t, db, user.Id, trade, 10000, 2000)
						if name == "planned payer amount" {
							require.NoError(t, db.Model(payment).Update("payer_amount", "9.00").Error)
							var planned model.CryptoPayment
							require.NoError(t, db.Where("trade_no = ?", trade).First(&planned).Error)
							require.Equal(t, "9.00", planned.PayerAmount)
						}
						info := cryptoPaidInfo(trade, cryptoInvoiceUUID(trade))
						mutate(&info)
						require.NoError(t, model.ApplyCryptomusPayment(info, ""))
						var stored model.User
						require.NoError(t, db.First(&stored, user.Id).Error)
						assert.Zero(t, stored.Quota)
						var storedPayment model.CryptoPayment
						require.NoError(t, db.Where("trade_no = ?", trade).First(&storedPayment).Error)
						if name != "cancelled" {
							assert.True(t, storedPayment.NeedsReview)
						}
						var order model.TopUp
						require.NoError(t, db.Where("trade_no = ?", trade).First(&order).Error)
						assert.Equal(t, common.TopUpStatusPending, order.Status)
					})
				}
			})

			t.Run("wrong invoice uuid is rejected", func(t *testing.T) {
				trade := "CRYUUID" + dialect.kind
				user := cryptoTestUser(t, db, "crypto-invoice-"+dialect.kind, 0, 0)
				payment := cryptoOrderFixture(t, db, user.Id, trade, 10000, 2000)
				bound := cryptoInvoiceUUID(trade + "-bound")
				require.NoError(t, db.Model(payment).Update("invoice_id", bound).Error)
				err := model.ApplyCryptomusPayment(cryptoPaidInfo(trade, cryptoInvoiceUUID(trade+"-other")), "")
				assert.ErrorIs(t, err, model.ErrPaymentMethodMismatch)
				var stored model.User
				require.NoError(t, db.First(&stored, user.Id).Error)
				assert.Zero(t, stored.Quota)
			})

			t.Run("wallet capacity rolls back order", func(t *testing.T) {
				trade := "CRYLIMIT" + dialect.kind
				user := cryptoTestUser(t, db, "crypto-limit-"+dialect.kind, 1, 0)
				cryptoOrderFixture(t, db, user.Id, trade, common.MaxWalletQuota, 0)
				err := model.ApplyCryptomusPayment(cryptoPaidInfo(trade, cryptoInvoiceUUID(trade)), "")
				assert.ErrorIs(t, err, model.ErrTopUpQuotaLimitExceeded)
				var order model.TopUp
				require.NoError(t, db.Where("trade_no = ?", trade).First(&order).Error)
				assert.Equal(t, common.TopUpStatusPending, order.Status)
				var payment model.CryptoPayment
				require.NoError(t, db.Where("trade_no = ?", trade).First(&payment).Error)
				assert.Nil(t, payment.InvoiceID)
				var stored model.User
				require.NoError(t, db.First(&stored, user.Id).Error)
				assert.Equal(t, 1, stored.Quota)
			})
		})
	}
}
