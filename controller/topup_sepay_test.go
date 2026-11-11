package controller

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/setting"
)

func signSePay(secret string, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestSePaySignatureValid(t *testing.T) {
	previous := setting.SePaySecretKey
	previousTolerance := setting.SePayToleranceSeconds
	t.Cleanup(func() {
		setting.SePaySecretKey = previous
		setting.SePayToleranceSeconds = previousTolerance
	})

	setting.SePaySecretKey = "spsk_live_test"
	setting.SePayToleranceSeconds = 300
	body := []byte(`{"id":1,"transferType":"in","transferAmount":1000}`)
	now := strconv.FormatInt(time.Now().Unix(), 10)
	valid := signSePay("spsk_live_test", now, body)

	cases := []struct {
		name      string
		timestamp string
		signature string
		body      []byte
		want      bool
	}{
		{"current window", now, valid, body, true},
		{"tampered body", now, valid, []byte(`{"id":1,"transferType":"in","transferAmount":9999}`), false},
		{"wrong secret", now, signSePay("spsk_live_other", now, body), body, false},
		{"stale timestamp", strconv.FormatInt(time.Now().Add(-time.Hour).Unix(), 10), valid, body, false},
		{"missing signature", now, "", body, false},
		{"non-numeric timestamp", "abc", valid, body, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sePaySignatureValid(tc.timestamp, tc.signature, tc.body); got != tc.want {
				t.Fatalf("sePaySignatureValid() = %v, want %v", got, tc.want)
			}
		})
	}
}
