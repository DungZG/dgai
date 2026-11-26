package model

import (
	"os"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func useTopUpBonusDB(t *testing.T) *gorm.DB {
	t.Helper()
	return openTopUpBonusDB(t, sqlite.Open(":memory:"), common.DatabaseTypeSQLite)
}

func openTopUpBonusDB(t *testing.T, driver gorm.Dialector, databaseType common.DatabaseType) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(driver, &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { assert.NoError(t, sqlDB.Close()) })
	if databaseType != common.DatabaseTypeSQLite {
		var databaseName string
		query := "SELECT DATABASE()"
		if databaseType == common.DatabaseTypePostgreSQL {
			query = "SELECT current_database()"
		}
		require.NoError(t, db.Raw(query).Scan(&databaseName).Error)
		require.Equal(t, "dgai_bonus_check", databaseName, "use the dedicated disposable test database only")
		tables, err := db.Migrator().GetTables()
		require.NoError(t, err)
		require.Empty(t, tables, "the disposable database must be empty")
		t.Cleanup(func() { assert.NoError(t, db.Migrator().DropTable(&TopUp{}, &Log{}, &User{})) })
	}
	require.NoError(t, db.AutoMigrate(&TopUp{}, &User{}, &Log{}))
	previousDB, previousLogDB := DB, LOG_DB
	previousMainType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
	previousQuotaPerUnit := common.QuotaPerUnit
	previousRedis, previousMemory := common.RedisEnabled, common.MemoryCacheEnabled
	DB, LOG_DB = db, db
	common.SetDatabaseTypes(databaseType, databaseType)
	initCol()
	common.QuotaPerUnit = 500000
	common.RedisEnabled, common.MemoryCacheEnabled = false, false
	t.Cleanup(func() {
		DB, LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
		initCol()
		common.QuotaPerUnit = previousQuotaPerUnit
		common.RedisEnabled, common.MemoryCacheEnabled = previousRedis, previousMemory
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
	setTopUpBonusTiers(t)

	previousPercent := setting.ReferralCommissionPercent
	previousRate := common.QuotaPerUnit
	t.Cleanup(func() {
		setting.ReferralCommissionPercent = previousPercent
		common.QuotaPerUnit = previousRate
	})
	common.QuotaPerUnit = 500000

	inviter := &User{Username: "inviter", AffCode: "inv001"}
	require.NoError(t, db.Create(inviter).Error)
	invitee := &User{Username: "invitee", AffCode: "inv002", InviterId: inviter.Id}
	require.NoError(t, db.Create(invitee).Error)

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
	assert.Equal(t, 10200000, getUserQuotaForPaymentGuardTest(t, invitee.Id))
	require.NoError(t, ManualCompleteTopUp(second.TradeNo, "127.0.0.1"))
	require.NoError(t, db.First(&afterEnabled, inviter.Id).Error)
	assert.Equal(t, 500000, afterEnabled.AffQuota, "replay must not pay commission again")

	var logCount int64
	require.NoError(t, db.Model(&Log{}).
		Where("user_id = ? AND type = ? AND content LIKE ?", inviter.Id, LogTypeSystem, "%commission%").
		Count(&logCount).Error)
	require.EqualValues(t, 1, logCount, "the inviter gets one reward event per paid top-up")
}

func setTopUpBonusTiers(t *testing.T) {
	t.Helper()
	payment := operation_setting.GetPaymentSetting()
	previousTiers, previousQuotaPerUnit := payment.TopUpBonusTiers, common.QuotaPerUnit
	common.QuotaPerUnit = 500000
	payment.TopUpBonusTiers = []operation_setting.TopUpBonusTier{
		{MinAmountUSD: 10, Percent: 2},
		{MinAmountUSD: 20, Percent: 5},
		{MinAmountUSD: 50, Percent: 12},
		{MinAmountUSD: 100, Percent: 25},
	}
	t.Cleanup(func() {
		payment.TopUpBonusTiers = previousTiers
		common.QuotaPerUnit = previousQuotaPerUnit
	})
}

func TestTopUpTierBoundariesAndWalletCapacityIncludeBonus(t *testing.T) {
	db := useTopUpBonusDB(t)
	setTopUpBonusTiers(t)

	for _, test := range []struct {
		baseQuota int
		bonus     int
	}{
		{4995000, 0},
		{5000000, 100000},
		{9995000, 199900},
		{10000000, 500000},
		{24995000, 1249750},
		{25000000, 3000000},
		{49995000, 5999400},
		{50000000, 12500000},
	} {
		bonus, err := operation_setting.TopUpBonus(test.baseQuota)
		require.NoError(t, err)
		assert.Equal(t, test.bonus, bonus, "base quota %d", test.baseQuota)
	}

	total, err := operation_setting.TopUpQuotaWithBonus(10000000)
	require.NoError(t, err)
	require.Equal(t, 10500000, total)
	user := &User{Username: "bonus-cap", AffCode: "bonuscap", Quota: common.MaxWalletQuota - 10500000}
	require.NoError(t, db.Create(user).Error)
	assert.NoError(t, ValidateTopUpQuotaCapacity(user.Id, total))
	require.NoError(t, db.Model(user).Update("quota", common.MaxWalletQuota-10500000+1).Error)
	assert.ErrorIs(t, ValidateTopUpQuotaCapacity(user.Id, total), ErrTopUpQuotaLimitExceeded)

	operation_setting.GetPaymentSetting().TopUpBonusTiers = []operation_setting.TopUpBonusTier{}
	bonus, err := operation_setting.TopUpBonus(50000000)
	require.NoError(t, err)
	assert.Zero(t, bonus, "empty tiers disable bonuses")
}

func TestValidateTopUpBonusTiersJSONRejectsInvalidTiers(t *testing.T) {
	setTopUpBonusTiers(t)
	for _, value := range []string{
		`{}`,
		`[{}]`,
		`null`,
		`[{"min_amount_usd":-10,"percent":2}]`,
		`[{"min_amount_usd":10,"percent":-1}]`,
		`[{"min_amount_usd":20,"percent":5},{"min_amount_usd":10,"percent":2}]`,
		`[{"min_amount_usd":10,"percent":101}]`,
		`[{"min_amount_usd":10,"percent":2},{"min_amount_usd":10,"percent":5}]`,
	} {
		assert.Error(t, operation_setting.ValidateTopUpBonusTiersJSON(value), value)
	}
	assert.NoError(t, operation_setting.ValidateTopUpBonusTiersJSON(`[]`))
	assert.NoError(t, operation_setting.ValidateTopUpBonusTiersJSON(`[{"min_amount_usd":10,"percent":0},{"min_amount_usd":20,"percent":100}]`))
}

func TestTopUpBonusFinalizersDatabaseMatrix(t *testing.T) {
	for _, dialect := range []struct {
		name string
		env  string
	}{
		{"sqlite", ""},
		{"mysql", "TEST_MYSQL_DSN"},
		{"postgres", "TEST_POSTGRES_DSN"},
	} {
		t.Run(dialect.name, func(t *testing.T) {
			var db *gorm.DB
			if dialect.env == "" {
				db = useTopUpBonusDB(t)
			} else {
				dsn := os.Getenv(dialect.env)
				if dsn == "" {
					t.Skip(dialect.env + " is not configured")
				}
				if dialect.name == "mysql" {
					db = openTopUpBonusDB(t, mysql.Open(dsn), common.DatabaseTypeMySQL)
				} else {
					db = openTopUpBonusDB(t, postgres.Open(dsn), common.DatabaseTypePostgreSQL)
				}
			}
			setTopUpBonusTiers(t)
			setSignupBonus(t, 0, 0)
			previousCommission := setting.ReferralCommissionPercent
			setting.ReferralCommissionPercent = 10
			t.Cleanup(func() { setting.ReferralCommissionPercent = previousCommission })

			for _, provider := range []struct {
				name       string
				amount     int64
				replayErr  bool
				commission bool
				finalize   func(string) error
			}{
				{PaymentProviderVietQR, 20, false, true, func(tradeNo string) error {
					return ManualCompleteTopUp(tradeNo, "127.0.0.1")
				}},
				{PaymentProviderEpay, 20, false, true, func(tradeNo string) error {
					_, err := RechargeEpay(tradeNo, "alipay", "127.0.0.1")
					return err
				}},
				{PaymentProviderStripe, 20, true, false, func(tradeNo string) error {
					return Recharge(tradeNo, "cus_bonus", "127.0.0.1")
				}},
				{PaymentProviderCreem, 10000000, true, false, func(tradeNo string) error {
					return RechargeCreem(tradeNo, "bonus@example.com", "Bonus", "127.0.0.1")
				}},
				{PaymentProviderWaffo, 20, false, false, func(tradeNo string) error {
					return RechargeWaffo(tradeNo, "127.0.0.1")
				}},
				{PaymentProviderWaffoPancake, 20, false, false, RechargeWaffoPancake},
			} {
				t.Run(provider.name, func(t *testing.T) {
					inviter := &User{Username: provider.name + "-ref", AffCode: provider.name + "-ref"}
					require.NoError(t, db.Create(inviter).Error)
					user := &User{Username: provider.name, AffCode: provider.name, InviterId: inviter.Id}
					require.NoError(t, db.Create(user).Error)
					order := &TopUp{
						UserId: user.Id, Amount: provider.amount, Money: 20, TradeNo: provider.name + "-bonus",
						PaymentMethod: provider.name, PaymentProvider: provider.name,
						Status: common.TopUpStatusPending, CreateTime: common.GetTimestamp(),
					}
					require.NoError(t, db.Create(order).Error)
					require.NoError(t, provider.finalize(order.TradeNo))
					assert.Equal(t, 10500000, getUserQuotaForPaymentGuardTest(t, user.Id))
					paid := GetTopUpByTradeNo(order.TradeNo)
					require.NotNil(t, paid)
					assert.Equal(t, common.TopUpStatusSuccess, paid.Status)
					assert.NotZero(t, paid.CompleteTime)
					replayErr := provider.finalize(order.TradeNo)
					if provider.replayErr {
						assert.Error(t, replayErr)
					} else {
						assert.NoError(t, replayErr)
					}
					assert.Equal(t, 10500000, getUserQuotaForPaymentGuardTest(t, user.Id), "replay must leave the wallet unchanged")
					require.NoError(t, db.First(inviter, inviter.Id).Error)
					if provider.commission {
						assert.Equal(t, 1000000, inviter.AffQuota, "commission uses the $20 base, not the $21 wallet credit")
						assert.Equal(t, 1000000, inviter.AffHistoryQuota)
					}
					previousAffQuota, previousAffHistory := inviter.AffQuota, inviter.AffHistoryQuota

					// The base fits, but the promotional bonus exceeds the remaining wallet capacity.
					startingQuota := common.MaxWalletQuota - 10000000
					require.NoError(t, db.Model(user).Update("quota", startingQuota).Error)
					capped := &TopUp{
						UserId: user.Id, Amount: provider.amount, Money: 20, TradeNo: provider.name + "-cap",
						PaymentMethod: provider.name, PaymentProvider: provider.name,
						Status: common.TopUpStatusPending, CreateTime: common.GetTimestamp(),
					}
					require.NoError(t, db.Create(capped).Error)
					require.Error(t, provider.finalize(capped.TradeNo))
					assert.Equal(t, startingQuota, getUserQuotaForPaymentGuardTest(t, user.Id))
					pending := GetTopUpByTradeNo(capped.TradeNo)
					require.NotNil(t, pending)
					assert.Equal(t, common.TopUpStatusPending, pending.Status)
					assert.Zero(t, pending.CompleteTime)
					require.NoError(t, db.First(inviter, inviter.Id).Error)
					assert.Equal(t, previousAffQuota, inviter.AffQuota)
					assert.Equal(t, previousAffHistory, inviter.AffHistoryQuota)
				})
			}
		})
	}
}

// A published promotion code pays once per account and only after the paid
// total reaches its minimum, and the ledger entry blocks a second grant.
func TestRedeemPromoCodeOnceAfterMinimum(t *testing.T) {
	db := useTopUpBonusDB(t)
	previousOptionMap := common.OptionMap
	common.OptionMap = map[string]string{
		PromoCodeRulesOptionKey: `[{"code":"DGAIHSTX","quota":2500000,"min_top_up_usd":10,"expires_at":0},{"code":"OLD","quota":2500000,"min_top_up_usd":0,"expires_at":1}]`,
	}
	t.Cleanup(func() { common.OptionMap = previousOptionMap })

	user := &User{Id: 501, Username: "promo-user", Password: "unused-password", Status: common.UserStatusEnabled, Group: "default"}
	require.NoError(t, db.Create(user).Error)

	granted, err := RedeemPromoCode("DGAIHSTX", user.Id)
	require.Error(t, err, "no top-up yet, so the minimum is not reached")
	assert.Zero(t, granted)

	require.NoError(t, db.Create(&TopUp{
		UserId: user.Id, Amount: 5000000, Money: 10, TradeNo: "VQRPROMO01",
		PaymentMethod: PaymentMethodVietQR, PaymentProvider: PaymentProviderVietQR,
		Status: common.TopUpStatusSuccess, CreateTime: 1, CompleteTime: 1,
	}).Error)

	granted, err = RedeemPromoCode("dgaihstx", user.Id)
	require.NoError(t, err, "the code is matched case-insensitively")
	assert.Equal(t, 2500000, granted)
	var stored User
	require.NoError(t, db.First(&stored, user.Id).Error)
	assert.Equal(t, 2500000, stored.Quota)

	granted, err = RedeemPromoCode("DGAIHSTX", user.Id)
	require.Error(t, err, "the same account cannot redeem twice")
	assert.Zero(t, granted)

	_, err = RedeemPromoCode("UNKNOWN", user.Id)
	require.Error(t, err)

	_, err = RedeemPromoCode("OLD", user.Id)
	require.Error(t, err, "an expired code grants nothing")
}

// A daily check-in pays one credit-denominated reward per day and never twice.
func TestClaimDailyCheckinOncePerDay(t *testing.T) {
	db := useTopUpBonusDB(t)
	user := &User{Id: 601, Username: "checkin-user", Password: "unused-password", Status: common.UserStatusEnabled, Group: "default"}
	require.NoError(t, db.Create(user).Error)

	credits, err := ClaimDailyCheckin(user.Id)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, credits, 1)
	assert.LessOrEqual(t, credits, 5)

	var stored User
	require.NoError(t, db.First(&stored, user.Id).Error)
	assert.Equal(t, credits*DailyCheckinQuotaPerCredit(), stored.Quota)

	_, err = ClaimDailyCheckin(user.Id)
	require.ErrorIs(t, err, ErrDailyCheckinAlreadyClaimed)

	var after User
	require.NoError(t, db.First(&after, user.Id).Error)
	assert.Equal(t, stored.Quota, after.Quota, "a second claim must not pay again")

	status, err := GetDailyCheckinStatus(user.Id)
	require.NoError(t, err)
	assert.True(t, status.ClaimedToday)
	assert.Equal(t, credits, status.CreditsToday)
	assert.Equal(t, credits, status.TotalCredits)
	assert.Greater(t, status.NextAt, common.GetTimestamp())
}
