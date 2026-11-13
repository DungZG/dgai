package service

import (
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting"
)

// shortcut: cooldown is per process, so several instances would each notify.
var spendVelocityAlertedAt sync.Map

// CheckSpendVelocity notifies administrators about users whose spend today is
// far above their own trailing 7-day average, and returns how many notices it
// sent.
func CheckSpendVelocity() int {
	now := time.Now()
	recentSince := now.Add(-24 * time.Hour).Unix()
	baselineSince := now.Add(-8 * 24 * time.Hour).Unix()

	rows, err := model.GetUserSpendWindows(recentSince, baselineSince)
	if err != nil {
		common.SysLog(common.LogText("failed to read spend windows: %s", err.Error()))
		return 0
	}

	sent := 0
	for _, row := range rows {
		if !spendVelocityExceeded(row.RecentQuota, row.BaselineQuota, setting.SpendVelocityMultiplier, setting.SpendVelocityMinQuota) {
			continue
		}
		dailyBaseline := float64(row.BaselineQuota) / 7
		if !spendVelocityAlertDue(row.UserId) {
			continue
		}

		ratio := float64(row.RecentQuota) / dailyBaseline
		params := map[string]any{
			"Id":    row.UserId,
			"Today": formatQuotaUSD(row.RecentQuota),
			"Avg":   formatQuotaUSD(int64(dailyBaseline)),
			"Ratio": fmt.Sprintf("%.1f", ratio),
		}
		sent += notifyAdmins(dto.NotifyTypeQuotaExceed, func(_ int, _ dto.UserSetting) bool { return true },
			func(lang string) (string, string) {
				return i18n.Translate(lang, i18n.MsgSpendVelocitySubject, params),
					i18n.Translate(lang, i18n.MsgSpendVelocityContent, params)
			})
	}
	return sent
}

// spendVelocityExceeded reports whether the recent window ran at least
// multiplier times the user's own daily average. A user without meaningful
// history is never flagged, because there is nothing to compare against.
func spendVelocityExceeded(recent, baseline int64, multiplier float64, minQuota int64) bool {
	if recent < minQuota {
		return false
	}
	dailyBaseline := float64(baseline) / 7
	if dailyBaseline < float64(minQuota)/4 {
		return false
	}
	return float64(recent) >= dailyBaseline*multiplier
}

func spendVelocityAlertDue(userId int) bool {
	now := time.Now()
	if last, ok := spendVelocityAlertedAt.Load(userId); ok {
		if now.Sub(last.(time.Time)) < 6*time.Hour {
			return false
		}
	}
	spendVelocityAlertedAt.Store(userId, now)
	return true
}

func formatQuotaUSD(quota int64) string {
	return fmt.Sprintf("$%.2f", float64(quota)/common.QuotaPerUnit)
}
