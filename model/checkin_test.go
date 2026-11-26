package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
)

// A credit is a tenth of a dollar, and the administrator's range still caps the
// reward at both ends.
func TestCheckinQuotaForCredits(t *testing.T) {
	previous := common.QuotaPerUnit
	common.QuotaPerUnit = 500000
	t.Cleanup(func() { common.QuotaPerUnit = previous })

	tests := []struct {
		name     string
		credits  int
		minQuota int
		maxQuota int
		want     int
	}{
		{"one credit", 1, 0, 0, 50000},
		{"five credits", 5, 0, 0, 250000},
		{"inside configured range", 3, 50000, 250000, 150000},
		{"raised to minimum", 1, 80000, 250000, 80000},
		{"capped at maximum", 5, 50000, 200000, 200000},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, checkinQuotaForCredits(tc.credits, tc.minQuota, tc.maxQuota))
		})
	}
}

// The draw never leaves the 1-5 credit band the product promises.
func TestDrawCheckinCreditsStaysInRange(t *testing.T) {
	for range 500 {
		credits := drawCheckinCredits()
		assert.GreaterOrEqual(t, credits, 1)
		assert.LessOrEqual(t, credits, 5)
	}
}
