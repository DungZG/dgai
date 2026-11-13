package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// GetRevenueReport returns revenue, upstream cost and margin grouped by day, user
// or model. It is a read-only report over consume logs; it never touches quota.
func GetRevenueReport(c *gin.Context) {
	groupBy := model.RevenueGroupBy(c.DefaultQuery("group_by", string(model.RevenueGroupByDay)))

	endTimestamp := common.GetTimestamp()
	startTimestamp := endTimestamp - 30*24*3600
	if raw := c.Query("start_timestamp"); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			common.ApiErrorMsg(c, "invalid start_timestamp")
			return
		}
		startTimestamp = value
	}
	if raw := c.Query("end_timestamp"); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			common.ApiErrorMsg(c, "invalid end_timestamp")
			return
		}
		endTimestamp = value
	}
	if startTimestamp > endTimestamp {
		common.ApiErrorMsg(c, "start_timestamp must not be after end_timestamp")
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
