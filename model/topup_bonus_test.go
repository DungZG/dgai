package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func useTopUpBonusDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB := DB
	previousType := common.MainDatabaseType()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&TopUp{}, &User{}, &Log{}))
	DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		DB = previousDB
		common.SetMainDatabaseType(previousType)
	})
	return db
}

// setSignupBonus configures the deferred reward and restores it after the test.
func setSignupBonus(t *testing.T, quota int, minTopUp float64) {
	t.Helper()
	payment := operation_setting.GetPaymentSetting()
	previousQuota := common.QuotaForNewUser
	previousMin := payment.SignupBonusMinTopUp
	common.QuotaForNewUser = quota
	payment.SignupBonusMinTopUp = minTopUp
	t.Cleanup(func() {
		common.QuotaForNewUser = previousQuota
		payment.SignupBonusMinTopUp = previousMin
	})
}

// A new account gets nothing at registration while the reward is deferred, so
// a throwaway signup cannot drain the wallet.
func TestSignupBonusDeferredUntilTopUpThreshold(t *testing.T) {
	db := useTopUpBonusDB(t)
	setSignupBonus(t, 2_500_000, 10)

	user := &User{Username: "deferred", AffCode: "aaaa"}
	require.NoError(t, user.Insert(0))
	assert.Equal(t, 0, user.Quota, "registration alone must not credit the reward")

	// Below the threshold: still nothing.
	order := &TopUp{
		UserId: user.Id, Amount: 4, Money: 4, TradeNo: "VQRBELOW0001",
		PaymentProvider: PaymentProviderVietQR, Status: common.TopUpStatusPending,
		CreateTime: common.GetTimestamp(),
	}
	require.NoError(t, db.Create(order).Error)
	require.NoError(t, ManualCompleteTopUp(order.TradeNo, "127.0.0.1"))
	assert.Zero(t, signupBonusLogCount(t, db, user.Id))
}

// Crossing the threshold pays the reward exactly once, even across several
// top-ups that add up to it.
func TestSignupBonusPaidOnceWhenTotalReachesThreshold(t *testing.T) {
	db := useTopUpBonusDB(t)
	setSignupBonus(t, 2_500_000, 10)

	user := &User{Username: "reaches", AffCode: "bbbb"}
	require.NoError(t, user.Insert(0))

	first := &TopUp{UserId: user.Id, Amount: 6, Money: 6, TradeNo: "VQRFIRST0001",
		PaymentProvider: PaymentProviderVietQR, Status: common.TopUpStatusPending,
		CreateTime: common.GetTimestamp()}
	require.NoError(t, db.Create(first).Error)
	require.NoError(t, ManualCompleteTopUp(first.TradeNo, "127.0.0.1"))
	assert.Zero(t, signupBonusLogCount(t, db, user.Id), "6 is below the 10 threshold")

	second := &TopUp{UserId: user.Id, Amount: 4, Money: 4, TradeNo: "VQRSECOND001",
		PaymentProvider: PaymentProviderVietQR, Status: common.TopUpStatusPending,
		CreateTime: common.GetTimestamp()}
	require.NoError(t, db.Create(second).Error)
	require.NoError(t, ManualCompleteTopUp(second.TradeNo, "127.0.0.1"))
	assert.EqualValues(t, 1, signupBonusLogCount(t, db, user.Id), "10 total crosses the threshold")

	third := &TopUp{UserId: user.Id, Amount: 5, Money: 5, TradeNo: "VQRTHIRD0001",
		PaymentProvider: PaymentProviderVietQR, Status: common.TopUpStatusPending,
		CreateTime: common.GetTimestamp()}
	require.NoError(t, db.Create(third).Error)
	require.NoError(t, ManualCompleteTopUp(third.TradeNo, "127.0.0.1"))
	assert.EqualValues(t, 1, signupBonusLogCount(t, db, user.Id), "the reward is never paid twice")
}

func signupBonusLogCount(t *testing.T, db *gorm.DB, userId int) int64 {
	t.Helper()
	var count int64
	require.NoError(t, db.Model(&Log{}).
		Where("user_id = ? AND type = ? AND content LIKE ?", userId, LogTypeSystem, "%New user sign-up bonus%").
		Count(&count).Error)
	return count
}

// A referred customer's payment pays the inviter a share of the base credit,
// never of the promotional bonus, and only when the rate is enabled.
func TestReferralCommissionOnTopUp(t *testing.T) {
	db := useTopUpBonusDB(t)
	setSignupBonus(t, 0, 0)

	previousPercent := setting.ReferralCommissionPercent
	previousRate := common.QuotaPerUnit
	t.Cleanup(func() {
		setting.ReferralCommissionPercent = previousPercent
		common.QuotaPerUnit = previousRate
	})
	common.QuotaPerUnit = 500000

	inviter := &User{Username: "inviter", AffCode: "inv001"}
	require.NoError(t, inviter.Insert(0))
	invitee := &User{Username: "invitee", AffCode: "inv002", InviterId: inviter.Id}
	require.NoError(t, invitee.Insert(inviter.Id))

	order := &TopUp{
		UserId: invitee.Id, Amount: 10, Money: 10, TradeNo: "VQRCOMM0001",
		PaymentProvider: PaymentProviderVietQR, Status: common.TopUpStatusPending,
		CreateTime: common.GetTimestamp(),
	}
	require.NoError(t, db.Create(order).Error)

	// Disabled by default: no commission.
	setting.ReferralCommissionPercent = 0
	require.NoError(t, ManualCompleteTopUp(order.TradeNo, "127.0.0.1"))
	var afterDisabled User
	require.NoError(t, db.First(&afterDisabled, inviter.Id).Error)
	require.Zero(t, afterDisabled.AffQuota)

	// 10% of a 10 USD top-up is 1 USD of quota.
	setting.ReferralCommissionPercent = 10
	second := &TopUp{
		UserId: invitee.Id, Amount: 10, Money: 10, TradeNo: "VQRCOMM0002",
		PaymentProvider: PaymentProviderVietQR, Status: common.TopUpStatusPending,
		CreateTime: common.GetTimestamp(),
	}
	require.NoError(t, db.Create(second).Error)
	require.NoError(t, ManualCompleteTopUp(second.TradeNo, "127.0.0.1"))

	var afterEnabled User
	require.NoError(t, db.First(&afterEnabled, inviter.Id).Error)
	require.EqualValues(t, common.QuotaPerUnit, afterEnabled.AffQuota)
	require.EqualValues(t, common.QuotaPerUnit, afterEnabled.AffHistoryQuota)

	var logCount int64
	require.NoError(t, db.Model(&Log{}).
		Where("user_id = ? AND type = ? AND content LIKE ?", inviter.Id, LogTypeSystem, "%commission%").
		Count(&logCount).Error)
	require.EqualValues(t, 1, logCount, "the inviter gets one reward event per paid top-up")
}
