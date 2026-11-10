package setting

// VietQR payment settings. The QR image flow needs only the bank account;
// the callback flow additionally needs the merchant credentials issued by
// VietQR (username/password for their token endpoint, plus the Basic auth
// pair VietQR uses to call our transaction-sync endpoint).
var (
	VietQREnabled   = false
	VietQRBankCode  = "TCB"
	VietQRBankAccount = "0395185726"
	VietQRAccountName = ""

	// Merchant credentials for the VietQR Callback API (api.vietqr.org).
	// Leave empty to stay in QR-image mode.
	VietQRUsername = ""
	VietQRPassword = ""
	VietQRSecretKey = ""
	VietQRSandbox   = true

	// Credentials VietQR uses to fetch our bearer token and push transactions.
	VietQRSyncUsername = ""
	VietQRSyncPassword = ""
	VietQRSyncSecretKey = ""

	// VietQRVNDRate converts the gateway's billing currency (USD) to VND for
	// the bank transfer. Update it when the exchange rate moves.
	VietQRVNDRate = 26500.0

	VietQRMinTopUp = 1

	// VietQRAPIBaseURL is the VietQR host used for the QR generate and
	// check-order APIs. Sandbox and production are different hosts.
	VietQRAPIBaseURL = "https://dev.vietqr.org"
)
