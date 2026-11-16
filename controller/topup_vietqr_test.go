package controller

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// The memo round-trips through bank apps, which upper-case it, add spaces and
// sometimes append their own note. Matching must survive all of that.
func TestVietQRNormalizeMemo(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"plain", "DGAIVQR11XF4LRX", "DGAIVQR11XF4LRX"},
		{"bank spaces", "DGAI VQR11 XF4LRX", "DGAIVQR11XF4LRX"},
		{"lowercase", "dgai vqr11xf4lrx", "DGAIVQR11XF4LRX"},
		{"appended note", "DGAIVQR11XF4LRX chuyen tien", "DGAIVQR11XF4LRXCHUYENTIEN"},
		{"dashes", "DGAI-VQR11-XF4LRX", "DGAIVQR11XF4LRX"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, vietqrNormalizeMemo(tc.content))
		})
	}
}

func TestVietQRTradeNoFromMemo(t *testing.T) {
	tradeNo := "VQR11XF4LRX"
	assert.Len(t, tradeNo, vietqrTradeNoLen)
	assert.Equal(t, "DGAIVQR11XF4LRX", vietqrMemo(tradeNo))

	for _, content := range []string{
		"DGAIVQR11XF4LRX",
		"DGAI VQR11 XF4LRX",
		"DGAIVQR11XF4LRX chuyen tien mua goi",
	} {
		normalized := vietqrNormalizeMemo(content)
		start := len("DGAI")
		assert.Equal(t, tradeNo, normalized[start:start+vietqrTradeNoLen], content)
	}
}

func TestCreateVietQRTopUpCreatesPendingOrder(t *testing.T) {
	oldDB := model.DB
	oldQuotaPerUnit := common.QuotaPerUnit
	oldRedis, oldMemoryCache := common.RedisEnabled, common.MemoryCacheEnabled
	oldDisplay := operation_setting.GetGeneralSetting().QuotaDisplayType
	oldPrice := operation_setting.Price
	oldMinTopUp := operation_setting.MinTopUp
	oldPayment := *operation_setting.GetPaymentSetting()
	oldVietQREnabled := setting.VietQREnabled
	oldBankCode, oldBankAccount := setting.VietQRBankCode, setting.VietQRBankAccount
	oldAccountName, oldMin := setting.VietQRAccountName, setting.VietQRMinTopUp
	oldRate := setting.VietQRVNDRate
	oldUsername, oldPassword := setting.VietQRUsername, setting.VietQRPassword
	t.Cleanup(func() {
		model.DB = oldDB
		common.QuotaPerUnit = oldQuotaPerUnit
		common.RedisEnabled, common.MemoryCacheEnabled = oldRedis, oldMemoryCache
		operation_setting.GetGeneralSetting().QuotaDisplayType = oldDisplay
		operation_setting.Price = oldPrice
		operation_setting.MinTopUp = oldMinTopUp
		*operation_setting.GetPaymentSetting() = oldPayment
		setting.VietQREnabled = oldVietQREnabled
		setting.VietQRBankCode, setting.VietQRBankAccount = oldBankCode, oldBankAccount
		setting.VietQRAccountName, setting.VietQRMinTopUp = oldAccountName, oldMin
		setting.VietQRVNDRate = oldRate
		setting.VietQRUsername, setting.VietQRPassword = oldUsername, oldPassword
	})

	common.QuotaPerUnit = 500000
	common.RedisEnabled, common.MemoryCacheEnabled = false, false
	operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeUSD
	operation_setting.Price = 1
	operation_setting.MinTopUp = 1
	payment := operation_setting.GetPaymentSetting()
	payment.ComplianceConfirmed = true
	payment.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion
	payment.AmountDiscount = nil
	setting.VietQREnabled = true
	setting.VietQRBankCode = "TPB"
	setting.VietQRBankAccount = "10006460824"
	setting.VietQRAccountName = "NGUYEN TRI DUNG"
	setting.VietQRMinTopUp = 1
	setting.VietQRVNDRate = 26500
	// Empty merchant credentials make the optional VietQR minting step fail
	// immediately, so the fallback img.vietqr.io QR is exercised without HTTP.
	setting.VietQRUsername, setting.VietQRPassword = "", ""

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.TopUp{}))
	model.DB = db
	t.Cleanup(func() {
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	user := &model.User{
		Id:       42,
		Username: "vietqr-user",
		Password: "unused-password",
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}
	require.NoError(t, db.Create(user).Error)

	result, err := createVietQRTopUp(context.Background(), user.Id, 10)
	require.NoError(t, err)
	assert.Len(t, result.TradeNo, vietqrTradeNoLen)
	assert.EqualValues(t, 10, result.Amount)
	assert.InDelta(t, 10, result.PayMoney, 0.0001)
	assert.EqualValues(t, 265000, result.VNDAmount)
	assert.Equal(t, "DGAI"+result.TradeNo, result.Memo)
	assert.Equal(t, "DGAI"+result.TradeNo, result.TransferMemo)
	assert.Contains(t, result.QRImageURL, "img.vietqr.io/image/TPB-10006460824")
	assert.Contains(t, result.QRImageURL, "DGAIVQR")

	var stored model.TopUp
	require.NoError(t, db.First(&stored, "trade_no = ?", result.TradeNo).Error)
	assert.Equal(t, common.TopUpStatusPending, stored.Status)
	assert.Equal(t, user.Id, stored.UserId)
	assert.EqualValues(t, 10, stored.Amount)
}
