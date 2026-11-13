package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSpendVelocityExceeded(t *testing.T) {
	const minQuota = 500000
	for _, tc := range []struct {
		name     string
		recent   int64
		baseline int64
		want     bool
	}{
		{"below floor", 100, 7_000_000, false},
		{"no history", 5_000_000, 0, false},
		{"normal day", 1_000_000, 7_000_000, false},
		{"three times average", 3_000_000, 7_000_000, true},
		{"just under", 2_900_000, 7_000_000, false},
		{"huge spike", 50_000_000, 7_000_000, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, spendVelocityExceeded(tc.recent, tc.baseline, 3.0, minQuota))
		})
	}
}
