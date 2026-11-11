package setting

// SePay settings. SePay watches the receiving bank account and pushes every
// balance change to our webhook, so it reconciles the same VietQR orders that
// VietQR itself cannot see: VietQR only links MB Bank and BIDV, while SePay
// covers TPBank and most other Vietnamese banks.
var (
	SePayEnabled    = false
	SePayMerchantID = ""
	SePaySecretKey  = ""

	// SePayToleranceSeconds bounds the replay window for the signed timestamp.
	SePayToleranceSeconds = 300
)
