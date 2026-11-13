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

// ChannelHealthAlertWindowMinutes is the lookback used to judge a channel.
var ChannelHealthAlertWindowMinutes = 30

// ChannelHealthAlertCooldown suppresses repeat notices for a channel that stays
// degraded, so one broken provider does not page every window.
var ChannelHealthAlertCooldown = 2 * time.Hour

// shortcut: the cooldown lives in process memory, so several instances would
// each send their own notice. One instance is the current deployment shape;
// move this to Redis if the gateway is scaled out.
var channelHealthAlertedAt sync.Map

// CheckChannelHealth notifies administrators about channels whose recent
// success rate fell below the threshold, and returns how many notices it sent.
func CheckChannelHealth(windowMinutes int) int {
	if windowMinutes <= 0 {
		windowMinutes = ChannelHealthAlertWindowMinutes
	}
	since := time.Now().Add(-time.Duration(windowMinutes) * time.Minute).Unix()
	rows, err := model.GetChannelSuccessRates(since)
	if err != nil {
		common.SysLog(common.LogText("failed to read channel success rates: %s", err.Error()))
		return 0
	}

	names, err := channelNamesById()
	if err != nil {
		common.SysLog(common.LogText("failed to read channel names: %s", err.Error()))
		return 0
	}

	sent := 0
	for _, row := range degradedChannels(rows, setting.ChannelHealthAlertMinRequests, setting.ChannelHealthAlertSuccessRate) {
		if !channelHealthAlertDue(row.ChannelId) {
			continue
		}

		name := names[row.ChannelId]
		rate := float64(row.SuccessCount) / float64(row.RequestCount) * 100
		if name == "" {
			name = fmt.Sprintf("#%d", row.ChannelId)
		}
		params := map[string]any{
			"Name":      name,
			"Id":        row.ChannelId,
			"Requests":  row.RequestCount,
			"Rate":      fmt.Sprintf("%.1f", rate),
			"Threshold": fmt.Sprintf("%.1f", setting.ChannelHealthAlertSuccessRate),
			"Minutes":   windowMinutes,
		}
		sent += notifyAdmins(dto.NotifyTypeChannelUpdate, func(_ int, setting dto.UserSetting) bool {
			if setting.ChannelNotifyEnabled != nil {
				return *setting.ChannelNotifyEnabled
			}
			return true
		}, func(lang string) (string, string) {
			return i18n.Translate(lang, i18n.MsgChannelHealthDegradedSubject, params),
				i18n.Translate(lang, i18n.MsgChannelHealthDegradedContent, params)
		})
	}
	return sent
}

// degradedChannels filters the aggregate down to channels that served enough
// traffic and still fell below the threshold, so a single failure on a quiet
// channel never pages anyone.
func degradedChannels(rows []model.ChannelSuccessRate, minRequests int64, threshold float64) []model.ChannelSuccessRate {
	degraded := make([]model.ChannelSuccessRate, 0, len(rows))
	for _, row := range rows {
		if row.RequestCount < minRequests {
			continue
		}
		rate := float64(row.SuccessCount) / float64(row.RequestCount) * 100
		if rate >= threshold {
			continue
		}
		degraded = append(degraded, row)
	}
	return degraded
}

func channelHealthAlertDue(channelId int) bool {
	now := time.Now()
	if last, ok := channelHealthAlertedAt.Load(channelId); ok {
		if now.Sub(last.(time.Time)) < ChannelHealthAlertCooldown {
			return false
		}
	}
	channelHealthAlertedAt.Store(channelId, now)
	return true
}

func channelNamesById() (map[int]string, error) {
	var channels []model.Channel
	if err := model.DB.Select("id", "name").Find(&channels).Error; err != nil {
		return nil, err
	}
	names := make(map[int]string, len(channels))
	for _, ch := range channels {
		names[ch.Id] = ch.Name
	}
	return names, nil
}
