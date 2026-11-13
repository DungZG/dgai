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

func TestModelMarginsDatabaseMatrix(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			var driver gorm.Dialector
			switch dialect {
			case "sqlite":
				driver = sqlite.Open("file:provider_cost_test?mode=memory&cache=shared")
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
			db, err := gorm.Open(driver, &gorm.Config{NamingStrategy: schema.NamingStrategy{TablePrefix: "provider_cost_test_"}})
			require.NoError(t, err)
			require.NoError(t, db.AutoMigrate(&ProviderCost{}, &Log{}))

			prevDB, prevLogDB, prevQuotaPerUnit := DB, LOG_DB, common.QuotaPerUnit
			DB, LOG_DB = db, db
			common.QuotaPerUnit = 500000
			t.Cleanup(func() {
				DB, LOG_DB = prevDB, prevLogDB
				common.QuotaPerUnit = prevQuotaPerUnit
			})

			require.NoError(t, SaveProviderCost(&ProviderCost{ChannelId: 1, ModelName: "priced", InputPrice: 2, OutputPrice: 6}))
			for _, log := range []Log{
				{Type: LogTypeConsume, ChannelId: 1, ModelName: "priced", PromptTokens: 1_000_000, CompletionTokens: 500_000, Quota: 500_000, CreatedAt: 100},
				{Type: LogTypeConsume, ChannelId: 2, ModelName: "unpriced", PromptTokens: 10, CompletionTokens: 10, Quota: 250_000, CreatedAt: 100},
				{Type: LogTypeTopup, ChannelId: 1, ModelName: "priced", PromptTokens: 1_000_000, Quota: 999_999, CreatedAt: 100},
			} {
				log := log
				require.NoError(t, db.Create(&log).Error)
			}

			rows, err := GetModelMargins(0, 1000)
			require.NoError(t, err)
			require.Len(t, rows, 2)

			byModel := map[string]ModelMargin{}
			for _, row := range rows {
				byModel[row.ModelName] = row
			}

			priced := byModel["priced"]
			assert.Equal(t, int64(1), priced.Requests)
			assert.Equal(t, 1.0, priced.Revenue)
			assert.Equal(t, 5.0, priced.Cost)
			assert.Equal(t, -4.0, priced.Margin)
			assert.Equal(t, -400.0, priced.MarginPercent)
			assert.True(t, priced.CostKnown)

			unpriced := byModel["unpriced"]
			assert.Equal(t, 0.5, unpriced.Revenue)
			assert.Equal(t, 0.0, unpriced.Cost)
			assert.False(t, unpriced.CostKnown)
		})
	}
}
