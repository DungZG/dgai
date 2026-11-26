package controller

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func GetDailyCheckin(c *gin.Context) {
	status, err := model.GetDailyCheckinStatus(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, status)
}

func PostDailyCheckin(c *gin.Context) {
	credits, err := model.ClaimDailyCheckin(c.GetInt("id"))
	if err != nil {
		if errors.Is(err, model.ErrDailyCheckinAlreadyClaimed) {
			common.ApiErrorT(c, "You have already checked in today")
			return
		}
		common.ApiError(c, err)
		return
	}
	quotaPerCredit := model.DailyCheckinQuotaPerCredit()
	common.ApiSuccess(c, gin.H{
		"credits":          credits,
		"quota":            credits * quotaPerCredit,
		"quota_per_credit": quotaPerCredit,
	})
}
