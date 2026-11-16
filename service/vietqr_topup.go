package service

import "context"

// VietQRTopUpResult is the order data both the HTTP wallet and the Telegram bot
// need in order to show the same QR and memo.
type VietQRTopUpResult struct {
	TradeNo       string
	Amount        int64
	PayMoney      float64
	BaseQuota     int
	BonusQuota    int
	CreditedQuota int
	Memo          string
	TransferMemo  string
	QRImageURL    string
	QRContent     string
	VNDAmount     int64
	VNDRate       float64
	BankCode      string
	BankAccount   string
	AccountName   string
	ExpiresAt     int64
}

// VietQRTopUpError carries a stable code for the bot while the HTTP handler
// still returns the message as an API error.
type VietQRTopUpError struct {
	Code    string
	Message string
}

func (e *VietQRTopUpError) Error() string { return e.Message }

const (
	VietQRTopUpErrorNotConfigured = "not_configured"
	VietQRTopUpErrorAmountTooLow  = "amount_too_low"
	VietQRTopUpErrorInvalidAmount = "invalid_amount"
	VietQRTopUpErrorQuotaLimit    = "quota_limit"
	VietQRTopUpErrorUser          = "user"
	VietQRTopUpErrorPaymentLow    = "payment_too_low"
	VietQRTopUpErrorCreate        = "create_failed"
)

// The controller owns the VietQR implementation; it installs this hook at
// startup so the Telegram service does not import the HTTP layer.
var createVietQRTopUp func(context.Context, int, int64) (*VietQRTopUpResult, error)

func SetVietQRTopUpCreator(creator func(context.Context, int, int64) (*VietQRTopUpResult, error)) {
	createVietQRTopUp = creator
}

func CreateVietQRTopUp(ctx context.Context, userId int, amount int64) (*VietQRTopUpResult, error) {
	if createVietQRTopUp == nil {
		return nil, &VietQRTopUpError{Code: VietQRTopUpErrorNotConfigured, Message: "VietQR top-up is not configured"}
	}
	return createVietQRTopUp(ctx, userId, amount)
}
