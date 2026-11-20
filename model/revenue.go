package model

import (
	"sort"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// RevenueGroupBy selects the reporting dimension of a revenue report.
type RevenueGroupBy string

const (
	RevenueGroupByDay   RevenueGroupBy = "day"
	RevenueGroupByUser  RevenueGroupBy = "user"
	RevenueGroupByModel RevenueGroupBy = "model"
)

// RevenueRow is one line of a revenue report. Cost stays zero and CostKnown
// false when no provider cost is recorded for any (channel, model) behind the
// row, so a report never shows a fabricated margin.
type RevenueRow struct {
	Key              string  `json:"key"`
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

type RevenueReport struct {
	GroupBy        string       `json:"group_by"`
	StartTimestamp int64        `json:"start_timestamp"`
	EndTimestamp   int64        `json:"end_timestamp"`
	Rows           []RevenueRow `json:"rows"`
	Totals         RevenueRow   `json:"totals"`
}

// vietnamUTCOffset keeps day buckets on the operator's local midnight instead of
// UTC, which is what the business actually reports on.
const vietnamUTCOffset = 7 * 3600

type revenueAgg struct {
	Dim              string
	ChannelId        int
	ModelName        string
	Requests         int64
	PromptTokens     int64
	CompletionTokens int64
	Quota            int64
}

// GetRevenueReport aggregates consume logs over [start, end] and prices them
// against provider_costs. The grouping happens in SQL only for the requested
// dimension plus (channel, model) so cost can be resolved; the final rollup and
// the cost join run in Go, which keeps the query portable across SQLite, MySQL
// and PostgreSQL and works when the log database is ClickHouse.
func GetRevenueReport(groupBy RevenueGroupBy, startTimestamp, endTimestamp int64) (*RevenueReport, error) {
	dimExpr, err := revenueDimExpression(groupBy)
	if err != nil {
		return nil, err
	}

	// The day and user dimensions are numeric; MySQL renders division as a
	// decimal, so cast them in the dialect's own integer spelling and group by
	// that same expression. Model names stay text, because casting a name to an
	// integer would collapse every model into one bucket.
	dimSelect := dimExpr
	if groupBy != RevenueGroupByModel {
		castType := "INTEGER"
		switch {
		case common.UsingLogDatabase(common.DatabaseTypeMySQL):
			castType = "SIGNED"
		case common.UsingLogDatabase(common.DatabaseTypePostgreSQL):
			castType = "BIGINT"
		}
		dimSelect = "CAST(" + dimExpr + " AS " + castType + ")"
	}
	var aggs []revenueAgg
	query := LOG_DB.Model(&Log{}).
		Select(dimSelect+" AS dim, channel_id, model_name, "+
			"count(*) AS requests, "+
			"COALESCE(sum(prompt_tokens), 0) AS prompt_tokens, "+
			"COALESCE(sum(completion_tokens), 0) AS completion_tokens, "+
			"COALESCE(sum(quota), 0) AS quota").
		Where("type = ? AND created_at >= ? AND created_at <= ?", LogTypeConsume, startTimestamp, endTimestamp).
		Group(dimSelect + ", channel_id, model_name")
	if err := query.Scan(&aggs).Error; err != nil {
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

	rowsByKey := make(map[string]*RevenueRow, len(aggs))
	for _, agg := range aggs {
		key, err := revenueRowKey(groupBy, agg.Dim)
		if err != nil {
			return nil, err
		}
		row, ok := rowsByKey[key]
		if !ok {
			row = &RevenueRow{Key: key, CostKnown: true}
			rowsByKey[key] = row
		}
		row.Requests += agg.Requests
		row.PromptTokens += agg.PromptTokens
		row.CompletionTokens += agg.CompletionTokens
		row.Quota += agg.Quota

		cost, known := costByKey[costKey{agg.ChannelId, agg.ModelName}]
		if !known {
			// A request that used no tokens cannot carry a per-token cost and is
			// negligible, so it must not hide the margin of the other rows.
			if agg.PromptTokens > 0 || agg.CompletionTokens > 0 {
				row.CostKnown = false
			}
			continue
		}
		// shortcut: prompt_tokens includes cached tokens, which the upstream
		// bills cheaper, so cost is an upper bound until per-cache pricing lands.
		row.Cost += float64(agg.PromptTokens)/1e6*cost.InputPrice +
			float64(agg.CompletionTokens)/1e6*cost.OutputPrice
	}

	report := &RevenueReport{
		GroupBy:        string(groupBy),
		StartTimestamp: startTimestamp,
		EndTimestamp:   endTimestamp,
		Rows:           make([]RevenueRow, 0, len(rowsByKey)),
	}
	// A total margin is only meaningful when every row has a cost; one unpriced
	// row would otherwise hide its spend behind a positive headline.
	report.Totals.CostKnown = true
	for _, row := range rowsByKey {
		finalizeRevenueRow(row)
		report.Rows = append(report.Rows, *row)
		report.Totals.Requests += row.Requests
		report.Totals.PromptTokens += row.PromptTokens
		report.Totals.CompletionTokens += row.CompletionTokens
		report.Totals.Quota += row.Quota
		report.Totals.Cost += row.Cost
		report.Totals.CostKnown = report.Totals.CostKnown && row.CostKnown
	}
	finalizeRevenueRow(&report.Totals)
	report.Totals.Key = "total"

	sort.Slice(report.Rows, func(i, j int) bool {
		if groupBy == RevenueGroupByDay {
			return report.Rows[i].Key < report.Rows[j].Key
		}
		// Money dimensions read best by what actually moved.
		return report.Rows[i].Revenue > report.Rows[j].Revenue
	})
	return report, nil
}

func finalizeRevenueRow(row *RevenueRow) {
	row.Revenue = float64(row.Quota) / common.QuotaPerUnit
	row.Margin = row.Revenue - row.Cost
	if row.Revenue > 0 {
		row.MarginPercent = row.Margin / row.Revenue * 100
	}
}

func revenueDimExpression(groupBy RevenueGroupBy) (string, error) {
	switch groupBy {
	case RevenueGroupByDay:
		// Local midnight of each log, as unix seconds: shift into local time,
		// floor to a day, then shift back.
		return "(created_at + " + strconv.Itoa(vietnamUTCOffset) + ") / 86400 * 86400 - " +
			strconv.Itoa(vietnamUTCOffset), nil
	case RevenueGroupByUser:
		return "user_id", nil
	case RevenueGroupByModel:
		return "model_name", nil
	default:
		return "", common.NewMessage("Unsupported group_by")
	}
}

func revenueRowKey(groupBy RevenueGroupBy, dim string) (string, error) {
	switch groupBy {
	case RevenueGroupByDay:
		seconds, err := strconv.ParseInt(dim, 10, 64)
		if err != nil {
			return "", err
		}
		return formatRevenueDay(seconds), nil
	case RevenueGroupByUser:
		userId, err := strconv.Atoi(dim)
		if err != nil {
			return "", err
		}
		return usernameById(userId), nil
	default:
		return dim, nil
	}
}

func formatRevenueDay(unixSeconds int64) string {
	// The bucket already sits on local midnight; shifting by the offset and
	// formatting in UTC yields the local calendar day it represents.
	return time.Unix(unixSeconds+vietnamUTCOffset, 0).UTC().Format("2006-01-02")
}

// usernameById resolves a user id for the report label. Ids are grouped in the
// log database but names live in the main database, so a missing name falls back
// to the raw id rather than dropping the row.
func usernameById(userId int) string {
	var username string
	if err := DB.Model(&User{}).Where("id = ?", userId).Pluck("username", &username).Error; err != nil || username == "" {
		return "#" + strconv.Itoa(userId)
	}
	return username
}

// UserTokenStat is one user's recorded token usage in a window. TopModel is the
// model that consumed the most of that user's tokens, so the page answers
// "which model does this user use most" without a second request.
type UserTokenStat struct {
	Key              string  `json:"key"`
	Requests         int64   `json:"requests"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	TotalTokens      int64   `json:"total_tokens"`
	TopModel         string  `json:"top_model"`
	TopModelTokens   int64   `json:"top_model_tokens"`
	TopModelShare    float64 `json:"top_model_share"`
	ModelCount       int     `json:"model_count"`
}

type UserTokenStatsReport struct {
	StartTimestamp int64           `json:"start_timestamp"`
	EndTimestamp   int64           `json:"end_timestamp"`
	Rows           []UserTokenStat `json:"rows"`
	Totals         UserTokenStat   `json:"totals"`
}

type userModelAgg struct {
	UserId           int
	ModelName        string
	Requests         int64
	PromptTokens     int64
	CompletionTokens int64
}

// GetUserTokenStats aggregates consume logs by user and model, then keeps each
// user's largest model. Ties break on the model name so a refresh cannot label
// the same data differently.
func GetUserTokenStats(startTimestamp, endTimestamp int64) (*UserTokenStatsReport, error) {
	var aggs []userModelAgg
	err := LOG_DB.Model(&Log{}).
		Select("user_id, model_name, count(*) AS requests, "+
			"COALESCE(sum(prompt_tokens), 0) AS prompt_tokens, "+
			"COALESCE(sum(completion_tokens), 0) AS completion_tokens").
		Where("type = ? AND created_at >= ? AND created_at <= ?", LogTypeConsume, startTimestamp, endTimestamp).
		Group("user_id, model_name").
		Scan(&aggs).Error
	if err != nil {
		return nil, err
	}

	type userAcc struct {
		stat       UserTokenStat
		topTokens  int64
		modelCount int
	}
	byUser := make(map[int]*userAcc, len(aggs))
	for _, agg := range aggs {
		acc := byUser[agg.UserId]
		if acc == nil {
			acc = &userAcc{stat: UserTokenStat{Key: usernameById(agg.UserId)}}
			byUser[agg.UserId] = acc
		}
		tokens := agg.PromptTokens + agg.CompletionTokens
		acc.stat.Requests += agg.Requests
		acc.stat.PromptTokens += agg.PromptTokens
		acc.stat.CompletionTokens += agg.CompletionTokens
		acc.modelCount++
		if tokens > acc.topTokens || (tokens == acc.topTokens && agg.ModelName < acc.stat.TopModel) {
			acc.topTokens = tokens
			acc.stat.TopModel = agg.ModelName
		}
	}

	report := &UserTokenStatsReport{
		StartTimestamp: startTimestamp,
		EndTimestamp:   endTimestamp,
		Rows:           make([]UserTokenStat, 0, len(byUser)),
	}
	for _, acc := range byUser {
		row := acc.stat
		row.TotalTokens = row.PromptTokens + row.CompletionTokens
		row.TopModelTokens = acc.topTokens
		row.ModelCount = acc.modelCount
		if row.TotalTokens > 0 {
			row.TopModelShare = float64(row.TopModelTokens) / float64(row.TotalTokens) * 100
		}
		report.Rows = append(report.Rows, row)
		report.Totals.Requests += row.Requests
		report.Totals.PromptTokens += row.PromptTokens
		report.Totals.CompletionTokens += row.CompletionTokens
	}
	report.Totals.TotalTokens = report.Totals.PromptTokens + report.Totals.CompletionTokens
	report.Totals.Key = "total"
	sort.Slice(report.Rows, func(i, j int) bool {
		if report.Rows[i].TotalTokens != report.Rows[j].TotalTokens {
			return report.Rows[i].TotalTokens > report.Rows[j].TotalTokens
		}
		return report.Rows[i].Key < report.Rows[j].Key
	})
	return report, nil
}
