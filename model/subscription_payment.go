package model

import "github.com/QuantumNous/new-api/common"

// GetPendingCryptoSubscriptionOrders returns crypto-paid plan orders old enough
// that the provider should already report a final status for them.
func GetPendingCryptoSubscriptionOrders(createdBefore int64, limit int) ([]SubscriptionOrder, error) {
	var orders []SubscriptionOrder
	err := DB.
		Where("status = ? AND payment_provider = ? AND create_time <= ?", common.TopUpStatusPending, PaymentProviderCryptomus, createdBefore).
		Order("create_time, trade_no").
		Limit(min(max(limit, 1), 100)).
		Find(&orders).Error
	return orders, err
}
