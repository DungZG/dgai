package model

import (
	"gorm.io/gorm"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/setting"
)

// CreditReferralCommission pays the inviter a share of the quota a referred
// customer just received from a top-up. It runs inside the top-up transaction
// so the commission and the credit commit together: a rollback returns the
// inviter's balance too.
//
// paidQuota is the base credit for the payment, excluding promotional bonuses,
// so the inviter never earns from a bonus that was not real money.
func CreditReferralCommission(tx *gorm.DB, inviteeId int, paidQuota int) (int, error) {
	percent := setting.ReferralCommissionPercent
	if percent <= 0 || paidQuota <= 0 {
		return 0, nil
	}
	if percent > 100 {
		percent = 100
	}

	var invitee User
	if err := tx.Select("id", "inviter_id", "username").First(&invitee, inviteeId).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return 0, nil
		}
		return 0, err
	}
	if invitee.InviterId == 0 || invitee.InviterId == invitee.Id {
		return 0, nil
	}

	commission := common.QuotaFromFloat(float64(paidQuota) * percent / 100)
	if commission <= 0 {
		return 0, nil
	}

	result := tx.Model(&User{}).Where("id = ?", invitee.InviterId).Updates(map[string]any{
		"aff_quota":   gorm.Expr("aff_quota + ?", commission),
		"aff_history": gorm.Expr("aff_history + ?", commission),
	})
	if result.Error != nil {
		return 0, result.Error
	}
	if result.RowsAffected == 0 {
		return 0, gorm.ErrRecordNotFound
	}

	// The inviter sees this in the notification bell, so record it in the same
	// transaction as the balance change.
	other := NewLogOther()
	log := &Log{
		UserId:    invitee.InviterId,
		CreatedAt: common.GetTimestamp(),
		Type:      LogTypeSystem,
		Content: other.setContent([]*common.Message{common.NewMessage(
			"Referral top-up commission {{quota}} from {{username}}",
			map[string]any{
				"quota":    logger.FormatQuota(commission),
				"username": invitee.Username,
			},
		)}),
		Other: other.JSONString(),
	}
	if err := tx.Create(log).Error; err != nil {
		return 0, err
	}
	return commission, nil
}
