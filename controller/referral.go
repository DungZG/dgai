package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"

	"github.com/gin-gonic/gin"
)

// referralEventLimit bounds the notification feed shown in the bell and on the
// referral card; the full history stays in the usage log.
const referralEventLimit = 20

// GetReferralInfo returns everything the referral card and the notification
// bell need in one call: the invite code, the reward totals, the commission
// rate and the most recent reward events.
func GetReferralInfo(c *gin.Context) {
	id := c.GetInt("id")
	user, err := model.GetUserById(id, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if user.AffCode == "" {
		user.AffCode = common.GetRandomString(4)
		if err := user.Update(false); err != nil {
			common.ApiError(c, err)
			return
		}
	}

	// Reward events are system logs: both the signup bonus for an invite and
	// the commission on a referred customer's top-up land there.
	events, _, err := model.GetUserLogs(id, model.LogFilter{Type: model.LogTypeSystem}, 0, referralEventLimit)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"aff_code":           user.AffCode,
			"aff_count":          user.AffCount,
			"aff_quota":          user.AffQuota,
			"aff_history_quota":  user.AffHistoryQuota,
			"commission_percent": setting.ReferralCommissionPercent,
			"events":             events,
		},
	})
}
