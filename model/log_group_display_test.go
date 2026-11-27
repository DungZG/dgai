package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A customer must never read the internal group name or its multiplier in their
// own usage log, while an administrator keeps the real figures.
func TestFormatUserLogsHidesInternalGroup(t *testing.T) {
	logs := []*Log{{
		Group: "plus",
		Other: `{"group":"plus","group_ratio":3,"user_group_ratio":3,"cache_ratio":0.3}`,
	}}
	formatUserLogs(logs, 0)

	assert.Equal(t, "default", logs[0].Group)
	var other map[string]any
	require.NoError(t, json.Unmarshal([]byte(logs[0].Other), &other))
	assert.Equal(t, "default", other["group"])
	assert.Equal(t, float64(1), other["group_ratio"])
	assert.Equal(t, float64(1), other["user_group_ratio"])
	assert.Equal(t, 0.3, other["cache_ratio"], "unrelated ratios stay untouched")
}

// A public group is reported as-is.
func TestFormatUserLogsKeepsPublicGroup(t *testing.T) {
	logs := []*Log{{
		Group: "vip",
		Other: `{"group":"vip","group_ratio":2}`,
	}}
	formatUserLogs(logs, 0)

	assert.Equal(t, "vip", logs[0].Group)
	var other map[string]any
	require.NoError(t, json.Unmarshal([]byte(logs[0].Other), &other))
	assert.Equal(t, "vip", other["group"])
	assert.Equal(t, float64(2), other["group_ratio"])
}
