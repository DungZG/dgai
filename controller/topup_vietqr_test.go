package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The memo round-trips through bank apps, which upper-case it, add spaces and
// sometimes append their own note. Matching must survive all of that.
func TestVietQRNormalizeMemo(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"plain", "DGAIVQR11XF4LRX", "DGAIVQR11XF4LRX"},
		{"bank spaces", "DGAI VQR11 XF4LRX", "DGAIVQR11XF4LRX"},
		{"lowercase", "dgai vqr11xf4lrx", "DGAIVQR11XF4LRX"},
		{"appended note", "DGAIVQR11XF4LRX chuyen tien", "DGAIVQR11XF4LRXCHUYENTIEN"},
		{"dashes", "DGAI-VQR11-XF4LRX", "DGAIVQR11XF4LRX"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, vietqrNormalizeMemo(tc.content))
		})
	}
}

// The callback recovers the trade number from the memo by fixed width, so the
// memo layout and the constant have to agree.
func TestVietQRTradeNoFromMemo(t *testing.T) {
	tradeNo := "VQR11XF4LRX"
	assert.Len(t, tradeNo, vietqrTradeNoLen)
	assert.Equal(t, "DGAIVQR11XF4LRX", vietqrMemo(tradeNo))

	for _, content := range []string{
		"DGAIVQR11XF4LRX",
		"DGAI VQR11 XF4LRX",
		"DGAIVQR11XF4LRX chuyen tien mua goi",
	} {
		normalized := vietqrNormalizeMemo(content)
		start := len("DGAI")
		assert.Equal(t, tradeNo, normalized[start:start+vietqrTradeNoLen], content)
	}
}
