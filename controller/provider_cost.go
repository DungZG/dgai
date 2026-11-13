package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func GetProviderCosts(c *gin.Context) {
	rows, err := model.ListProviderCosts()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, rows)
}

func SaveProviderCost(c *gin.Context) {
	var row model.ProviderCost
	if err := c.ShouldBindJSON(&row); err != nil {
		common.ApiError(c, err)
		return
	}
	if row.ChannelId <= 0 || row.ModelName == "" {
		common.ApiErrorMsg(c, "channel_id and model_name are required")
		return
	}
	if row.InputPrice < 0 || row.OutputPrice < 0 {
		common.ApiErrorMsg(c, "prices must not be negative")
		return
	}
	if err := model.SaveProviderCost(&row); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, row)
}

func DeleteProviderCost(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "invalid id")
		return
	}
	if err := model.DeleteProviderCost(id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func GetModelMargins(c *gin.Context) {
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
	rows, err := model.GetModelMargins(startTimestamp, endTimestamp)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"start_timestamp": startTimestamp,
		"end_timestamp":   endTimestamp,
		"quota_per_unit":  common.QuotaPerUnit,
		"items":           rows,
	})
}
