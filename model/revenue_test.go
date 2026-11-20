package model

import (
	"os"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestRevenueReportDatabaseMatrix(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			var driver gorm.Dialector
			switch dialect {
			case "sqlite":
				driver = sqlite.Open("file:revenue_test?mode=memory&cache=shared")
			case "mysql":
				dsn := os.Getenv("TEST_MYSQL_DSN")
				if dsn == "" {
					t.Skip("TEST_MYSQL_DSN is not configured")
				}
				driver = mysql.Open(dsn)
			case "postgres":
				dsn := os.Getenv("TEST_POSTGRES_DSN")
				if dsn == "" {
					t.Skip("TEST_POSTGRES_DSN is not configured")
				}
				driver = postgres.Open(dsn)
			}
			db, err := gorm.Open(driver, &gorm.Config{NamingStrategy: schema.NamingStrategy{TablePrefix: "revenue_test_"}})
			require.NoError(t, err)
			require.NoError(t, db.AutoMigrate(&ProviderCost{}, &Log{}, &User{}))
			// The matrix may reuse a database between runs; clear the prefixed
			// tables so fixed ids and log rows cannot collide with earlier data.
			for _, model := range []any{&Log{}, &ProviderCost{}, &User{}} {
				require.NoError(t, db.Unscoped().Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(model).Error)
			}

			prevDB, prevLogDB, prevQuota := DB, LOG_DB, common.QuotaPerUnit
			prevMainType, prevLogType := common.MainDatabaseType(), common.LogDatabaseType()
			DB, LOG_DB = db, db
			common.SetDatabaseTypes(common.DatabaseType(dialect), common.DatabaseType(dialect))
			common.QuotaPerUnit = 500000
			t.Cleanup(func() {
				common.SetDatabaseTypes(prevMainType, prevLogType)
			})
			t.Cleanup(func() {
				DB, LOG_DB = prevDB, prevLogDB
				common.QuotaPerUnit = prevQuota
			})

			require.NoError(t, db.Create(&User{Id: 1, Username: "alice"}).Error)
			require.NoError(t, SaveProviderCost(&ProviderCost{ChannelId: 1, ModelName: "priced", InputPrice: 2, OutputPrice: 6}))

			// Two local days, both inside the report window.
			const day = int64(86400)
			base := int64(1794000000) - (int64(1794000000)+vietnamUTCOffset)%day
			for _, row := range []Log{
				{Type: LogTypeConsume, UserId: 1, ChannelId: 1, ModelName: "priced", PromptTokens: 1_000_000, CompletionTokens: 500_000, Quota: 500_000, CreatedAt: base + 100},
				{Type: LogTypeConsume, UserId: 1, ChannelId: 1, ModelName: "priced", PromptTokens: 1_000_000, CompletionTokens: 500_000, Quota: 500_000, CreatedAt: base + day + 100},
				{Type: LogTypeConsume, UserId: 1, ChannelId: 2, ModelName: "unpriced", PromptTokens: 10, CompletionTokens: 10, Quota: 250_000, CreatedAt: base + 200},
				// An empty request on the priced day must not mark that day's cost unknown.
				{Type: LogTypeConsume, UserId: 1, ChannelId: 2, ModelName: "ghost", CreatedAt: base + day + 200},
				{Type: LogTypeTopup, UserId: 1, ChannelId: 1, ModelName: "priced", Quota: 999_999, CreatedAt: base + 300},
			} {
				row := row
				require.NoError(t, db.Create(&row).Error)
			}

			report, err := GetRevenueReport(RevenueGroupByDay, base, base+2*day)
			require.NoError(t, err)
			require.Len(t, report.Rows, 2)
			assert.Less(t, report.Rows[0].Key, report.Rows[1].Key)

			byDay := map[string]RevenueRow{}
			for _, row := range report.Rows {
				byDay[row.Key] = row
			}
			first := byDay[formatRevenueDay(base)]
			assert.Equal(t, int64(2), first.Requests)
			assert.Equal(t, 1.5, first.Revenue) // 750000 quota / 500000
			assert.Equal(t, 5.0, first.Cost)    // 1M*2 + 500k*6 per million
			assert.False(t, first.CostKnown)    // "unpriced" has no cost row

			second := byDay[formatRevenueDay(base+day)]
			assert.Equal(t, 1.0, second.Revenue)
			assert.True(t, second.CostKnown)

			assert.Equal(t, int64(3), report.Totals.Requests)
			assert.Equal(t, 2.5, report.Totals.Revenue)
			assert.False(t, report.Totals.CostKnown, "one unpriced row hides the total margin")

			byUser, err := GetRevenueReport(RevenueGroupByUser, base, base+2*day)
			require.NoError(t, err)
			require.Len(t, byUser.Rows, 1)
			assert.Equal(t, "alice", byUser.Rows[0].Key)

			byModel, err := GetRevenueReport(RevenueGroupByModel, base, base+2*day)
			require.NoError(t, err)
			require.Len(t, byModel.Rows, 2)
			assert.Equal(t, "priced", byModel.Rows[0].Key, "sorted by revenue")

			// A second user with a different top model proves the pick is per
			// user, not one global maximum.
			require.NoError(t, db.Create(&User{Id: 2, Username: "bob", AffCode: "bob"}).Error)
			for _, row := range []Log{
				{Type: LogTypeConsume, UserId: 2, ChannelId: 1, ModelName: "priced", PromptTokens: 10, CompletionTokens: 10, Quota: 1, CreatedAt: base + 400},
				{Type: LogTypeConsume, UserId: 2, ChannelId: 2, ModelName: "unpriced", PromptTokens: 1_000_000, CompletionTokens: 0, Quota: 1, CreatedAt: base + 401},
				{Type: LogTypeTopup, UserId: 2, ChannelId: 1, ModelName: "priced", Quota: 999_999, CreatedAt: base + 402},
			} {
				row := row
				require.NoError(t, db.Create(&row).Error)
			}

			stats, err := GetUserTokenStats(base, base+2*day)
			require.NoError(t, err)
			require.Len(t, stats.Rows, 2)
			assert.Equal(t, "alice", stats.Rows[0].Key, "sorted by total tokens")
			alice := stats.Rows[0]
			assert.Equal(t, int64(3), alice.Requests)
			assert.Equal(t, int64(2_000_010), alice.PromptTokens)
			assert.Equal(t, int64(1_000_010), alice.CompletionTokens)
			assert.Equal(t, int64(3_000_020), alice.TotalTokens)
			assert.Equal(t, "priced", alice.TopModel)
			assert.Equal(t, int64(3_000_000), alice.TopModelTokens)
			assert.Equal(t, 2, alice.ModelCount)
			assert.InDelta(t, 99.9993, alice.TopModelShare, 0.001)

			bob := stats.Rows[1]
			assert.Equal(t, "bob", bob.Key)
			assert.Equal(t, "unpriced", bob.TopModel, "each user keeps their own largest model")
			assert.Equal(t, int64(1_000_020), bob.TotalTokens)

			assert.Equal(t, int64(5), stats.Totals.Requests, "top-up logs are not usage")
			assert.Equal(t, int64(4_000_040), stats.Totals.TotalTokens)
		})
	}
}
