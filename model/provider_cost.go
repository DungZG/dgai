package model

import (
	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm/clause"
)

// ProviderCost is what the upstream provider charges us per 1M tokens for one
// (channel, model) pair, in USD. Nothing on the relay path reads this table:
// it only feeds margin reporting, so it can be edited while requests flow.
type ProviderCost struct {
	Id          int     `json:"id"`
	ChannelId   int     `json:"channel_id" gorm:"uniqueIndex:idx_provider_cost_channel_model,priority:1"`
	ModelName   string  `json:"model_name" gorm:"type:varchar(128);uniqueIndex:idx_provider_cost_channel_model,priority:2"`
	InputPrice  float64 `json:"input_price"`
	OutputPrice float64 `json:"output_price"`
	UpdatedAt   int64   `json:"updated_at" gorm:"bigint"`
}

// ModelMargin aggregates customer revenue and upstream cost per (channel, model).
// Revenue comes from the consume logs; cost comes from ProviderCost.
type ModelMargin struct {
	ChannelId        int     `json:"channel_id"`
	ModelName        string  `json:"model_name"`
	Requests         int64   `json:"requests"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	Quota            int64   `json:"quota"`
	Revenue          float64 `json:"revenue"`
	Cost             float64 `json:"cost"`
	Margin           float64 `json:"margin"`
	MarginPercent    float64 `json:"margin_percent"`
	CostKnown        bool    `json:"cost_known"`
}

func ListProviderCosts() ([]ProviderCost, error) {
	var rows []ProviderCost
	err := DB.Order("channel_id, model_name").Find(&rows).Error
	return rows, err
}

func SaveProviderCost(row *ProviderCost) error {
	row.UpdatedAt = common.GetTimestamp()
	return DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "channel_id"}, {Name: "model_name"}},
		DoUpdates: clause.AssignmentColumns([]string{"input_price", "output_price", "updated_at"}),
	}).Create(row).Error
}

func DeleteProviderCost(id int) error {
	return DB.Delete(&ProviderCost{}, id).Error
}

// GetModelMargins sums consume logs per (channel, model) in [start, end] and
// prices them against ProviderCost. Rows without a cost entry keep CostKnown
// false so the dashboard never shows a fabricated 100% margin.
func GetModelMargins(startTimestamp, endTimestamp int64) ([]ModelMargin, error) {
	var rows []ModelMargin
	err := LOG_DB.Model(&Log{}).
		Select("channel_id, model_name, count(*) AS requests, "+
			"COALESCE(sum(prompt_tokens), 0) AS prompt_tokens, "+
			"COALESCE(sum(completion_tokens), 0) AS completion_tokens, "+
			"COALESCE(sum(quota), 0) AS quota").
		Where("type = ? AND created_at >= ? AND created_at <= ?", LogTypeConsume, startTimestamp, endTimestamp).
		Group("channel_id, model_name").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	costs, err := ListProviderCosts()
	if err != nil {
		return nil, err
	}
	type costKey struct {
		channel int
		model   string
	}
	costByKey := make(map[costKey]ProviderCost, len(costs))
	for _, cost := range costs {
		costByKey[costKey{cost.ChannelId, cost.ModelName}] = cost
	}

	for i := range rows {
		row := &rows[i]
		row.Revenue = float64(row.Quota) / common.QuotaPerUnit
		// shortcut: prompt_tokens includes cached tokens, which the upstream
		// bills cheaper, so cost is an upper bound until per-cache pricing lands.
		if cost, ok := costByKey[costKey{row.ChannelId, row.ModelName}]; ok {
			row.Cost = float64(row.PromptTokens)/1e6*cost.InputPrice +
				float64(row.CompletionTokens)/1e6*cost.OutputPrice
			row.CostKnown = true
		}
		row.Margin = row.Revenue - row.Cost
		if row.Revenue > 0 {
			row.MarginPercent = row.Margin / row.Revenue * 100
		}
	}
	return rows, nil
}
