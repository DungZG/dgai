package cryptomus

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A settled invoice can sit a cent under the quote because the provider applies
// its own conversion rate; that must still count as paid.
func TestCoversAmountAllowsProviderRounding(t *testing.T) {
	tests := []struct {
		name     string
		paid     string
		expected string
		want     bool
	}{
		{"exact", "100.00", "100.00", true},
		{"one cent short", "99.99", "100.00", true},
		{"overpaid", "100.50", "100.00", true},
		{"underpaid beyond tolerance", "98.00", "100.00", false},
		{"empty paid", "", "100.00", false},
		{"zero expected", "5.00", "0", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, CoversAmount(tc.paid, tc.expected))
		})
	}
}
