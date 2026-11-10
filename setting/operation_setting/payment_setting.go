package operation_setting

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

type PaymentSetting struct {
	AmountOptions  []int           `json:"amount_options"`
	AmountDiscount map[int]float64 `json:"amount_discount"` // 充值金额对应的折扣，例如 100 元 0.9 表示 100 元充值享受 9 折优惠

	// TopUpBonusPercent credits extra quota on top of a top-up, so paying 10
	// dollars at 10 percent yields 11 dollars of credit. TopUpBonusMinAmount is
	// the smallest top-up that earns it; zero disables the bonus entirely.
	TopUpBonusPercent   float64 `json:"topup_bonus_percent"`
	TopUpBonusMinAmount int     `json:"topup_bonus_min_amount"`

	// SignupBonusMinTopUp defers the new-user bonus until the customer has paid
	// this much in successful top-ups. Zero grants it at registration instead.
	SignupBonusMinTopUp float64 `json:"signup_bonus_min_top_up"`

	ComplianceConfirmed    bool   `json:"compliance_confirmed"`
	ComplianceTermsVersion string `json:"compliance_terms_version"`
	ComplianceConfirmedAt  int64  `json:"compliance_confirmed_at"`
	ComplianceConfirmedBy  int    `json:"compliance_confirmed_by"`
	ComplianceConfirmedIP  string `json:"compliance_confirmed_ip"`
}

const CurrentComplianceTermsVersion = "v1"

// 默认配置
var paymentSetting = PaymentSetting{
	AmountOptions:  []int{10, 20, 50, 100, 200, 500},
	AmountDiscount: map[int]float64{},
}

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("payment_setting", &paymentSetting)
}

func GetPaymentSetting() *PaymentSetting {
	return &paymentSetting
}

// TopUpBonus applies the configured promotional bonus to a paid amount and
// returns the extra quota granted on top of the base credit. The base credit is
// passed in so the caller keeps its own rounding rules; the bonus is derived
// from the same amount, never from user input alone.
func TopUpBonus(amountUSD float64, baseQuota int) int {
	setting := GetPaymentSetting()
	if setting.TopUpBonusPercent <= 0 || amountUSD < float64(setting.TopUpBonusMinAmount) {
		return 0
	}
	if baseQuota <= 0 {
		return 0
	}
	bonus := float64(baseQuota) * setting.TopUpBonusPercent / 100
	if bonus <= 0 {
		return 0
	}
	return common.QuotaFromFloat(bonus)
}

func IsPaymentComplianceConfirmed() bool {
	return paymentSetting.ComplianceConfirmed &&
		paymentSetting.ComplianceTermsVersion == CurrentComplianceTermsVersion
}
