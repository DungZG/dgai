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

			prevDB, prevLogDB, prevQuota := DB, LOG_DB, common.QuotaPerUnit
			DB, LOG_DB = db, db
			common.QuotaPerUnit = 500000
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

			byUser, err := GetRevenueReport(RevenueGroupByUser, base, base+2*day)
			require.NoError(t, err)
			require.Len(t, byUser.Rows, 1)
			assert.Equal(t, "alice", byUser.Rows[0].Key)

			byModel, err := GetRevenueReport(RevenueGroupByModel, base, base+2*day)
			require.NoError(t, err)
			require.Len(t, byModel.Rows, 2)
			assert.Equal(t, "priced", byModel.Rows[0].Key, "sorted by revenue")
		})
	}
}
