package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// revenueReportRange reads the shared [start, end] window of the reporting
// endpoints: the last 30 days unless the caller passes unix seconds.
func revenueReportRange(c *gin.Context) (int64, int64, bool) {
	endTimestamp := common.GetTimestamp()
	startTimestamp := endTimestamp - 30*24*3600
	if raw := c.Query("start_timestamp"); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			common.ApiErrorMsg(c, "invalid start_timestamp")
			return 0, 0, false
		}
		startTimestamp = value
	}
	if raw := c.Query("end_timestamp"); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			common.ApiErrorMsg(c, "invalid end_timestamp")
			return 0, 0, false
		}
		endTimestamp = value
	}
	if startTimestamp > endTimestamp {
		common.ApiErrorMsg(c, "start_timestamp must not be after end_timestamp")
		return 0, 0, false
	}
	return startTimestamp, endTimestamp, true
}

// GetRevenueReport returns revenue, upstream cost and margin grouped by day, user
// or model. It is a read-only report over consume logs; it never touches quota.
func GetRevenueReport(c *gin.Context) {
	groupBy := model.RevenueGroupBy(c.DefaultQuery("group_by", string(model.RevenueGroupByDay)))

	startTimestamp, endTimestamp, ok := revenueReportRange(c)
	if !ok {
		return
	}

	report, err := model.GetRevenueReport(groupBy, startTimestamp, endTimestamp)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"group_by":        report.GroupBy,
		"start_timestamp": report.StartTimestamp,
		"end_timestamp":   report.EndTimestamp,
		"quota_per_unit":  common.QuotaPerUnit,
		"rows":            report.Rows,
		"totals":          report.Totals,
	})
}

// GetUserTokenStats returns per-user token totals and the model that consumed
// the most of each user's tokens. Read-only over consume logs.
func GetUserTokenStats(c *gin.Context) {
	startTimestamp, endTimestamp, ok := revenueReportRange(c)
	if !ok {
		return
	}
	report, err := model.GetUserTokenStats(startTimestamp, endTimestamp)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, report)
}
