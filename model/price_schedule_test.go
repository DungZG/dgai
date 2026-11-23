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
)

// TestPriceScheduleDatabaseMatrix proves the new table and its conditional
// updates behave the same on every supported database.
func TestPriceScheduleDatabaseMatrix(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			var driver gorm.Dialector
			switch dialect {
			case "sqlite":
				driver = sqlite.Open("file:price_schedule_matrix?mode=memory&cache=shared")
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
			db, err := gorm.Open(driver, &gorm.Config{})
			require.NoError(t, err)
			require.NoError(t, db.AutoMigrate(&PriceSchedule{}, &Model{}))
			require.NoError(t, db.Unscoped().Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&PriceSchedule{}).Error)
			require.NoError(t, db.Unscoped().Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&Model{}).Error)

			prevDB, prevLogDB := DB, LOG_DB
			prevMainType, prevLogType := common.MainDatabaseType(), common.LogDatabaseType()
			DB, LOG_DB = db, db
			common.SetDatabaseTypes(common.DatabaseType(dialect), common.DatabaseType(dialect))
			InitColumnNames()
			t.Cleanup(func() {
				common.SetDatabaseTypes(prevMainType, prevLogType)
				DB, LOG_DB = prevDB, prevLogDB
			})

			row := &PriceSchedule{
				ModelName: "m", NewRatio: 0.05, NewCompletion: 4, NewCache: 0.3,
				EffectiveAt: common.GetTimestamp() - 1, Status: PriceSchedulePending,
			}
			require.NoError(t, CreatePriceSchedule(row))
			due, err := GetDuePriceSchedules(common.GetTimestamp(), 10)
			require.NoError(t, err)
			require.Len(t, due, 1)
			assert.Equal(t, "m", due[0].ModelName)

			applied, err := MarkPriceScheduleApplied(row.Id, common.GetTimestamp())
			require.NoError(t, err)
			assert.True(t, applied)
			applied, err = MarkPriceScheduleApplied(row.Id, common.GetTimestamp())
			require.NoError(t, err)
			assert.False(t, applied, "a second apply must not win the race")

			cancelledRow := &PriceSchedule{
				ModelName: "n", NewRatio: 0.05, NewCompletion: 4, NewCache: 0.3,
				EffectiveAt: common.GetTimestamp() + 60, Status: PriceSchedulePending,
			}
			require.NoError(t, CreatePriceSchedule(cancelledRow))
			cancelled, err := CancelPriceSchedule(cancelledRow.Id)
			require.NoError(t, err)
			assert.True(t, cancelled)
			cancelled, err = CancelPriceSchedule(cancelledRow.Id)
			require.NoError(t, err)
			assert.False(t, cancelled)

			require.NoError(t, db.Create(&Model{ModelName: "m", Tags: "Reasoning,sale:50"}).Error)
			require.NoError(t, UpdateModelSaleTag("m", "90"))
			var catalog Model
			require.NoError(t, db.Where("model_name = ?", "m").First(&catalog).Error)
			assert.Equal(t, "Reasoning,sale:90", catalog.Tags)
			require.NoError(t, UpdateModelSaleTag("m", "0"))
			require.NoError(t, db.Where("model_name = ?", "m").First(&catalog).Error)
			assert.Equal(t, "Reasoning", catalog.Tags)
		})
	}
}
