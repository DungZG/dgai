package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
)

func TestDegradedChannels(t *testing.T) {
	rows := []model.ChannelSuccessRate{
		{ChannelId: 1, RequestCount: 100, SuccessCount: 50}, // 50% -> alert
		{ChannelId: 2, RequestCount: 100, SuccessCount: 90}, // 90% -> ok
		{ChannelId: 3, RequestCount: 5, SuccessCount: 0},    // too little traffic
		{ChannelId: 4, RequestCount: 20, SuccessCount: 16},  // 80% -> exactly at threshold, ok
		{ChannelId: 5, RequestCount: 20, SuccessCount: 15},  // 75% -> alert
	}

	degraded := degradedChannels(rows, 20, 80)
	assert.Len(t, degraded, 2)
	assert.Equal(t, 1, degraded[0].ChannelId)
	assert.Equal(t, 5, degraded[1].ChannelId)
}

func TestDegradedChannelsEmpty(t *testing.T) {
	assert.Empty(t, degradedChannels(nil, 20, 80))
	assert.Empty(t, degradedChannels([]model.ChannelSuccessRate{{ChannelId: 1, RequestCount: 0}}, 20, 80))
}
