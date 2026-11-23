package service

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

const (
	priceScheduleTick     = 15 * time.Second
	priceScheduleBatch    = 20
	priceAnnouncementMark = "GIÁ MỚI: "
)

var vietnamLocation = time.FixedZone("ICT", 7*3600)

// StartPriceScheduler applies due price changes. Only the master node writes,
// and every apply is idempotent, so a restart or a duplicate tick is harmless.
func StartPriceScheduler() {
	if !common.IsMasterNode {
		return
	}
	go func() {
		runDuePriceSchedules()
		ticker := time.NewTicker(priceScheduleTick)
		defer ticker.Stop()
		for range ticker.C {
			runDuePriceSchedules()
		}
	}()
}

func runDuePriceSchedules() {
	rows, err := model.GetDuePriceSchedules(common.GetTimestamp(), priceScheduleBatch)
	if err != nil {
		common.SysError(common.LogText("reading due price schedules failed: %s", err.Error()))
		return
	}
	for i := range rows {
		applyPriceSchedule(&rows[i])
	}
}

// ApplyPriceSchedule applies one stored change now. It is used by the bot's
// confirmation and by the admin API for immediate changes.
func ApplyPriceSchedule(id int) error {
	row, err := model.GetPriceSchedule(id)
	if err != nil {
		return err
	}
	if row.Status != model.PriceScheduleDraft && row.Status != model.PriceSchedulePending {
		return fmt.Errorf("price change %d is already %s", id, row.Status)
	}
	return applyPriceSchedule(row)
}

func applyPriceSchedule(row *model.PriceSchedule) error {
	// The API applies an immediate change while the ticker may already have the
	// same row in hand; re-read so only one of them writes.
	fresh, err := model.GetPriceSchedule(row.Id)
	if err == nil {
		if fresh.Status != model.PriceSchedulePending && fresh.Status != model.PriceScheduleDraft {
			return nil
		}
		row = fresh
	}
	if row.NewRatio <= 0 || row.NewCompletion <= 0 || row.NewCache <= 0 {
		_ = model.MarkPriceScheduleFailed(row.Id, "price values must be positive")
		return fmt.Errorf("price change %d has invalid values", row.Id)
	}
	ratios, err := readRatioMap("ModelRatio")
	if err != nil {
		return failPriceSchedule(row, err)
	}
	completions, err := readRatioMap("CompletionRatio")
	if err != nil {
		return failPriceSchedule(row, err)
	}
	caches, err := readRatioMap("CacheRatio")
	if err != nil {
		return failPriceSchedule(row, err)
	}

	// Snapshot the live values right before writing so undo restores what was
	// actually replaced, even if another edit landed after the change was
	// scheduled.
	row.OldRatio = ratios[row.ModelName]
	row.OldCompletion = completions[row.ModelName]
	row.OldCache = caches[row.ModelName]
	_ = model.DB.Model(&model.PriceSchedule{}).Where("id = ?", row.Id).Updates(map[string]any{
		"old_ratio": row.OldRatio, "old_completion": row.OldCompletion, "old_cache": row.OldCache,
	})

	ratios[row.ModelName] = row.NewRatio
	completions[row.ModelName] = row.NewCompletion
	caches[row.ModelName] = row.NewCache
	updates := make(map[string]string, 3)
	for key, values := range map[string]map[string]float64{
		"ModelRatio": ratios, "CompletionRatio": completions, "CacheRatio": caches,
	} {
		encoded, err := common.Marshal(values)
		if err != nil {
			return failPriceSchedule(row, err)
		}
		updates[key] = string(encoded)
	}
	if err := model.UpdateModelPricingOptions(updates); err != nil {
		return failPriceSchedule(row, err)
	}

	var warnings []string
	if err := model.UpdateModelSaleTag(row.ModelName, row.SaleTag); err != nil {
		warnings = append(warnings, "sale tag: "+err.Error())
	}
	if row.Announce {
		if err := publishPriceAnnouncement(row); err != nil {
			warnings = append(warnings, "announcement: "+err.Error())
		}
	}
	if applied, err := model.MarkPriceScheduleApplied(row.Id, common.GetTimestamp()); err != nil {
		warnings = append(warnings, "mark applied: "+err.Error())
	} else if !applied {
		warnings = append(warnings, "the change was cancelled while it was being applied")
	}
	if len(warnings) > 0 {
		_ = model.SetPriceScheduleNote(row.Id, strings.Join(warnings, "; "))
	}
	text := "✅ Đã áp dụng giá mới cho <b>" + row.ModelName + "</b>."
	if len(warnings) > 0 {
		text += "\n⚠️ " + strings.Join(warnings, "; ")
	}
	notifyPriceSchedule(row, text)
	return nil
}

func failPriceSchedule(row *model.PriceSchedule, err error) error {
	_ = model.MarkPriceScheduleFailed(row.Id, err.Error())
	notifyPriceSchedule(row, "⚠️ Đổi giá <b>"+row.ModelName+"</b> thất bại: "+err.Error())
	return err
}

func readRatioMap(key string) (map[string]float64, error) {
	var option model.Option
	err := model.DB.Where("key = ?", key).First(&option).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return map[string]float64{}, nil
	}
	if err != nil {
		return nil, err
	}
	values := make(map[string]float64)
	if strings.TrimSpace(option.Value) == "" {
		return values, nil
	}
	if err := common.UnmarshalJsonStr(option.Value, &values); err != nil {
		return nil, err
	}
	return values, nil
}

// publishPriceAnnouncement replaces the previous note for the same model so the
// console shows one current line per model instead of a growing history.
func publishPriceAnnouncement(row *model.PriceSchedule) error {
	announcements, err := readAnnouncements()
	if err != nil {
		return err
	}
	kept := announcements[:0]
	for _, entry := range announcements {
		content, _ := entry["content"].(string)
		if strings.HasPrefix(content, priceAnnouncementMark+row.ModelName+" ") {
			continue
		}
		kept = append(kept, entry)
	}
	kept = append(kept, map[string]any{
		"id":          time.Now().UnixMilli(),
		"publishDate": time.Unix(row.EffectiveAt, 0).In(vietnamLocation).Format(time.RFC3339),
		"type":        "ongoing",
		"content":     priceAnnouncementText(row),
	})
	encoded, err := common.Marshal(kept)
	if err != nil {
		return err
	}
	return model.UpdateOption("console_setting.announcements", string(encoded))
}

func readAnnouncements() ([]map[string]any, error) {
	var option model.Option
	err := model.DB.Where("key = ?", "console_setting.announcements").First(&option).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(option.Value) == "" {
		return nil, nil
	}
	var entries []map[string]any
	if err := common.UnmarshalJsonStr(option.Value, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func priceAnnouncementText(row *model.PriceSchedule) string {
	input, output, cache := priceSchedulePrices(row)
	when := time.Unix(row.EffectiveAt, 0).In(vietnamLocation).Format("02/01/2006 15:04")
	return fmt.Sprintf("%s%s — $%s/1M vào, $%s/1M ra, cache $%s/1M. Áp dụng từ %s.",
		priceAnnouncementMark, row.ModelName, formatPrice(input), formatPrice(output), formatPrice(cache), when)
}

func priceSchedulePrices(row *model.PriceSchedule) (input, output, cache float64) {
	input = row.NewRatio * 2
	output = input * row.NewCompletion
	cache = input * row.NewCache
	return input, output, cache
}

func formatPrice(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

// notifyPriceSchedule reports the result in the chat that created the change.
func notifyPriceSchedule(row *model.PriceSchedule, text string) {
	bot := activeTelegramBot
	if bot == nil || row.ChatID == 0 {
		return
	}
	_ = bot.sendCard(row.ChatID, text, nil)
}
