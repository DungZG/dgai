package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// PriceSchedule statuses. A draft waits for the administrator to confirm it in
// the bot; pending is armed and applied by the scheduler; the rest are final.
const (
	PriceScheduleDraft     = "draft"
	PriceSchedulePending   = "pending"
	PriceScheduleApplied   = "applied"
	PriceScheduleCancelled = "cancelled"
	PriceScheduleFailed    = "failed"
)

// PriceSchedule is one planned price change for a model. Old values are kept so
// an administrator can undo the change without reconstructing them by hand.
type PriceSchedule struct {
	Id            int     `json:"id" gorm:"primaryKey"`
	ModelName     string  `json:"model_name" gorm:"type:varchar(128);index"`
	OldRatio      float64 `json:"old_ratio"`
	OldCompletion float64 `json:"old_completion"`
	OldCache      float64 `json:"old_cache"`
	NewRatio      float64 `json:"new_ratio"`
	NewCompletion float64 `json:"new_completion"`
	NewCache      float64 `json:"new_cache"`
	// SaleTag is "90" for sale:90, empty to leave tags untouched, and "0" to
	// remove an existing sale tag.
	SaleTag     string `json:"sale_tag" gorm:"type:varchar(16)"`
	EffectiveAt int64  `json:"effective_at" gorm:"index"`
	Status      string `json:"status" gorm:"type:varchar(16);index"`
	Announce    bool   `json:"announce"`
	Note        string `json:"note" gorm:"type:varchar(255)"`
	ChatID      int64  `json:"chat_id"`
	CreatedBy   int    `json:"created_by"`
	CreatedAt   int64  `json:"created_at"`
	AppliedAt   int64  `json:"applied_at"`
	Error       string `json:"error" gorm:"type:text"`
}

func CreatePriceSchedule(row *PriceSchedule) error {
	if row.CreatedAt == 0 {
		row.CreatedAt = common.GetTimestamp()
	}
	if row.Status == "" {
		row.Status = PriceSchedulePending
	}
	return DB.Create(row).Error
}

func GetPriceSchedule(id int) (*PriceSchedule, error) {
	var row PriceSchedule
	if err := DB.First(&row, id).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// GetDuePriceSchedules returns armed rows whose time has come, oldest first.
func GetDuePriceSchedules(now int64, limit int) ([]PriceSchedule, error) {
	var rows []PriceSchedule
	err := DB.Where("status = ? AND effective_at <= ?", PriceSchedulePending, now).
		Order("effective_at asc, id asc").Limit(limit).Find(&rows).Error
	return rows, err
}

// GetOpenPriceSchedules lists drafts and armed rows for the bot's list command.
func GetOpenPriceSchedules(limit int) ([]PriceSchedule, error) {
	var rows []PriceSchedule
	err := DB.Where("status IN ?", []string{PriceScheduleDraft, PriceSchedulePending}).
		Order("effective_at asc, id asc").Limit(limit).Find(&rows).Error
	return rows, err
}

// ArmPriceSchedule moves a confirmed draft into the scheduler's queue.
func ArmPriceSchedule(id int) (bool, error) {
	result := DB.Model(&PriceSchedule{}).
		Where("id = ? AND status = ?", id, PriceScheduleDraft).
		Update("status", PriceSchedulePending)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

// CancelPriceSchedule only cancels rows that have not been applied yet.
func CancelPriceSchedule(id int) (bool, error) {
	result := DB.Model(&PriceSchedule{}).
		Where("id = ? AND status IN ?", id, []string{PriceScheduleDraft, PriceSchedulePending}).
		Update("status", PriceScheduleCancelled)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

// MarkPriceScheduleApplied flips an open row to applied. The conditional update
// is what keeps a cancelled or already-applied row from being applied twice.
func MarkPriceScheduleApplied(id int, appliedAt int64) (bool, error) {
	result := DB.Model(&PriceSchedule{}).
		Where("id = ? AND status IN ?", id, []string{PriceScheduleDraft, PriceSchedulePending}).
		Updates(map[string]any{"status": PriceScheduleApplied, "applied_at": appliedAt})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func MarkPriceScheduleFailed(id int, message string) error {
	return DB.Model(&PriceSchedule{}).Where("id = ?", id).
		Updates(map[string]any{"status": PriceScheduleFailed, "error": message}).Error
}

// SetPriceScheduleNote records a non-fatal problem (for example a failed
// announcement) without hiding that the price itself was applied.
func SetPriceScheduleNote(id int, message string) error {
	return DB.Model(&PriceSchedule{}).Where("id = ?", id).Update("error", message).Error
}

// UpdateModelSaleTag rewrites the sale marker of a model's catalog tags.
// An empty saleTag leaves the tags untouched.
func UpdateModelSaleTag(modelName, saleTag string) error {
	saleTag = strings.TrimSpace(saleTag)
	if saleTag == "" {
		return nil
	}
	var row Model
	err := DB.Where("model_name = ?", modelName).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	tags := make([]string, 0, 8)
	for _, tag := range strings.Split(row.Tags, ",") {
		tag = strings.TrimSpace(tag)
		if tag == "" || strings.HasPrefix(tag, "sale:") || strings.HasPrefix(tag, "sale-") {
			continue
		}
		tags = append(tags, tag)
	}
	if saleTag != "0" {
		tags = append(tags, "sale:"+saleTag)
	}
	return DB.Model(&Model{}).Where("id = ?", row.Id).Update("tags", strings.Join(tags, ",")).Error
}
