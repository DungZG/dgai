package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupPriceScheduleDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.PriceSchedule{}, &model.Option{}, &model.Model{}, &model.ProviderCost{}))

	prevDB, prevLogDB := model.DB, model.LOG_DB
	prevMainType, prevLogType := common.MainDatabaseType(), common.LogDatabaseType()
	model.DB, model.LOG_DB = db, db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	model.InitColumnNames()
	common.OptionMapRWMutex.Lock()
	prevOptionMap := common.OptionMap
	common.OptionMap = map[string]string{}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = prevOptionMap
		common.OptionMapRWMutex.Unlock()
		common.SetDatabaseTypes(prevMainType, prevLogType)
		model.DB, model.LOG_DB = prevDB, prevLogDB
	})
	return db
}

func seedPricingOption(t *testing.T, db *gorm.DB, key, value string) {
	t.Helper()
	require.NoError(t, db.Create(&model.Option{Key: key, Value: value}).Error)
}

func readOptionMap(t *testing.T, db *gorm.DB, key string) map[string]float64 {
	t.Helper()
	var option model.Option
	require.NoError(t, db.Where("key = ?", key).First(&option).Error)
	values := make(map[string]float64)
	require.NoError(t, common.UnmarshalJsonStr(option.Value, &values))
	return values
}

func TestApplyPriceScheduleMergesModelPricing(t *testing.T) {
	db := setupPriceScheduleDB(t)
	seedPricingOption(t, db, "ModelRatio", `{"other":0.1}`)
	seedPricingOption(t, db, "CompletionRatio", `{"other":5}`)
	seedPricingOption(t, db, "CacheRatio", `{"other":0.2}`)
	require.NoError(t, db.Create(&model.Model{ModelName: "m", Tags: "Reasoning,sale:50"}).Error)

	row := &model.PriceSchedule{
		ModelName: "m", NewRatio: 0.055, NewCompletion: 6, NewCache: 0.3,
		SaleTag: "90", EffectiveAt: common.GetTimestamp(), Status: model.PriceSchedulePending,
	}
	require.NoError(t, model.CreatePriceSchedule(row))
	require.NoError(t, ApplyPriceSchedule(row.Id))

	assert.Equal(t, 0.1, readOptionMap(t, db, "ModelRatio")["other"], "other models keep their price")
	assert.Equal(t, 0.055, readOptionMap(t, db, "ModelRatio")["m"])
	assert.Equal(t, 6.0, readOptionMap(t, db, "CompletionRatio")["m"])
	assert.Equal(t, 0.3, readOptionMap(t, db, "CacheRatio")["m"])

	stored, err := model.GetPriceSchedule(row.Id)
	require.NoError(t, err)
	assert.Equal(t, model.PriceScheduleApplied, stored.Status)
	assert.InDelta(t, 0, stored.OldRatio, 1e-9, "a model without a previous ratio has no old value")

	var catalog model.Model
	require.NoError(t, db.Where("model_name = ?", "m").First(&catalog).Error)
	assert.Equal(t, "Reasoning,sale:90", catalog.Tags, "the old sale marker is replaced")
}

func TestApplyPriceScheduleReplacesAnnouncement(t *testing.T) {
	db := setupPriceScheduleDB(t)
	seedPricingOption(t, db, "ModelRatio", `{"m":0.1}`)
	seedPricingOption(t, db, "CompletionRatio", `{"m":5}`)
	seedPricingOption(t, db, "CacheRatio", `{"m":0.3}`)
	seedPricingOption(t, db, "console_setting.announcements",
		`[{"id":1,"publishDate":"2026-01-01T00:00:00+07:00","type":"ongoing","content":"GIÁ MỚI: m — giá cũ"},{"id":2,"publishDate":"2026-01-01T00:00:00+07:00","type":"ongoing","content":"Thông báo khác"}]`)

	row := &model.PriceSchedule{
		ModelName: "m", NewRatio: 0.05, NewCompletion: 4, NewCache: 0.3,
		EffectiveAt: common.GetTimestamp(), Status: model.PriceSchedulePending, Announce: true,
	}
	require.NoError(t, model.CreatePriceSchedule(row))
	require.NoError(t, ApplyPriceSchedule(row.Id))

	entries, err := readAnnouncements()
	require.NoError(t, err)
	require.Len(t, entries, 2, "the old note for the same model is replaced, not appended")
	kept := map[string]bool{}
	for _, entry := range entries {
		kept[entry["content"].(string)] = true
	}
	assert.True(t, kept["Thông báo khác"])
	assert.False(t, kept["GIÁ MỚI: m — giá cũ"])
}

func TestPriceScheduleRespectsTimeAndCancellation(t *testing.T) {
	db := setupPriceScheduleDB(t)
	seedPricingOption(t, db, "ModelRatio", `{"m":0.1}`)
	seedPricingOption(t, db, "CompletionRatio", `{"m":5}`)
	seedPricingOption(t, db, "CacheRatio", `{"m":0.3}`)

	future := &model.PriceSchedule{
		ModelName: "m", NewRatio: 0.05, NewCompletion: 4, NewCache: 0.3,
		EffectiveAt: common.GetTimestamp() + 3600, Status: model.PriceSchedulePending,
	}
	require.NoError(t, model.CreatePriceSchedule(future))
	due, err := model.GetDuePriceSchedules(common.GetTimestamp(), 10)
	require.NoError(t, err)
	assert.Empty(t, due, "a future change is not due")

	cancelled, err := model.CancelPriceSchedule(future.Id)
	require.NoError(t, err)
	assert.True(t, cancelled)
	require.Error(t, ApplyPriceSchedule(future.Id), "a cancelled change cannot be applied")
	assert.Equal(t, 0.1, readOptionMap(t, db, "ModelRatio")["m"])

	require.NoError(t, db.Create(&model.Model{ModelName: "m"}).Error)
	clearTag := &model.PriceSchedule{
		ModelName: "m", NewRatio: 0.05, NewCompletion: 4, NewCache: 0.3, SaleTag: "0",
		EffectiveAt: common.GetTimestamp(), Status: model.PriceSchedulePending,
	}
	require.NoError(t, db.Model(&model.Model{}).Where("model_name = ?", "m").Update("tags", "Reasoning,sale:90").Error)
	require.NoError(t, model.CreatePriceSchedule(clearTag))
	require.NoError(t, ApplyPriceSchedule(clearTag.Id))
	var catalog model.Model
	require.NoError(t, db.Where("model_name = ?", "m").First(&catalog).Error)
	assert.Equal(t, "Reasoning", catalog.Tags, "sale:0 removes the marker")
}

func TestParsePriceCommand(t *testing.T) {
	now := int64(1795000000)
	command, err := parsePriceCommand("gpt-6.2-sol 0.11 0.66", now)
	require.NoError(t, err)
	assert.Equal(t, "gpt-6.2-sol", command.modelName)
	assert.InDelta(t, 0.11, command.inputPrice, 1e-9)
	assert.InDelta(t, 0.033, command.cachePrice, 1e-9, "cache defaults to 30% of input")
	assert.True(t, command.announce)
	assert.Equal(t, now, command.effectiveAt)

	command, err = parsePriceCommand("m $0,05 0,3 0,02 --after 90m --sale 90 --no-announce", now)
	require.NoError(t, err)
	assert.InDelta(t, 0.05, command.inputPrice, 1e-9)
	assert.InDelta(t, 0.3, command.outputPrice, 1e-9)
	assert.InDelta(t, 0.02, command.cachePrice, 1e-9)
	assert.Equal(t, "90", command.saleTag)
	assert.False(t, command.announce)
	assert.Equal(t, now+90*60, command.effectiveAt)

	_, err = parsePriceCommand("m 0.1", now)
	assert.Error(t, err)
	_, err = parsePriceCommand("m -1 0.5", now)
	assert.Error(t, err)
	_, err = parsePriceCommand("m 0.1 0.5 --sale 200", now)
	assert.Error(t, err)

	at, err := parsePriceAt("23:59", now)
	require.NoError(t, err)
	assert.Equal(t, "23:59", time.Unix(at, 0).In(vietnamLocation).Format("15:04"))
	assert.True(t, at > now)
}
