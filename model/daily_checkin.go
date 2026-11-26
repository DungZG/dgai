package model

import (
	"crypto/rand"
	"errors"
	"math/big"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// Daily check-in pays a small quota reward once per calendar day. The reward is
// quoted in credits where 10 credits equal one dollar, so a credit is worth
// QuotaPerUnit/10.
const (
	DailyCheckinPaymentMethod    = "checkin"
	dailyCheckinCreditsPerDollar = 10
	dailyCheckinTradeNoPrefix    = "checkin-"
)

// ErrDailyCheckinAlreadyClaimed is returned when today's reward was already paid.
var ErrDailyCheckinAlreadyClaimed = errors.New("daily check-in already claimed")

// dailyCheckinWeights is a basis-point table: 60% one credit, 30% two, 9.98%
// three, and 0.01% each for four and five.
var dailyCheckinWeights = []struct {
	credits     int
	basisPoints int
}{
	{1, 6000},
	{2, 3000},
	{3, 998},
	{4, 1},
	{5, 1},
}

// The day boundary follows Vietnam time regardless of the server clock.
var dailyCheckinZone = time.FixedZone("ICT", 7*3600)

type DailyCheckinStatus struct {
	ClaimedToday   bool  `json:"claimed_today"`
	CreditsToday   int   `json:"credits_today"`
	TotalCredits   int   `json:"total_credits"`
	NextAt         int64 `json:"next_at"`
	QuotaPerCredit int   `json:"quota_per_credit"`
}

func dailyCheckinDayKey(now time.Time) string {
	return now.In(dailyCheckinZone).Format("20060102")
}

func dailyCheckinTradeNo(userId int, dayKey string) string {
	return dailyCheckinTradeNoPrefix + strconv.Itoa(userId) + "-" + dayKey
}

func dailyCheckinNextAt(now time.Time) int64 {
	local := now.In(dailyCheckinZone)
	next := time.Date(local.Year(), local.Month(), local.Day()+1, 0, 0, 0, 0, dailyCheckinZone)
	return next.Unix()
}

// DailyCheckinQuotaPerCredit converts one credit into wallet quota.
func DailyCheckinQuotaPerCredit() int {
	if common.QuotaPerUnit <= 0 {
		return 0
	}
	return int(common.QuotaPerUnit) / dailyCheckinCreditsPerDollar
}

// drawDailyCheckinCredits picks today's reward. crypto/rand keeps the rare
// outcomes unpredictable, so nobody can time the draw.
func drawDailyCheckinCredits() int {
	n, err := rand.Int(rand.Reader, big.NewInt(10000))
	if err != nil {
		return 1
	}
	roll := int(n.Int64())
	cumulative := 0
	for _, weight := range dailyCheckinWeights {
		cumulative += weight.basisPoints
		if roll < cumulative {
			return weight.credits
		}
	}
	return 1
}

func GetDailyCheckinStatus(userId int) (*DailyCheckinStatus, error) {
	now := time.Now()
	status := &DailyCheckinStatus{
		NextAt:         dailyCheckinNextAt(now),
		QuotaPerCredit: DailyCheckinQuotaPerCredit(),
	}
	var today TopUp
	err := DB.
		Select("amount").
		Where("trade_no = ?", dailyCheckinTradeNo(userId, dailyCheckinDayKey(now))).
		First(&today).Error
	if err == nil {
		status.ClaimedToday = true
		status.CreditsToday = creditsFromQuota(int(today.Amount))
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	var total int64
	if err := DB.Model(&TopUp{}).
		Select("coalesce(sum(amount), 0)").
		Where("user_id = ? AND payment_method = ?", userId, DailyCheckinPaymentMethod).
		Scan(&total).Error; err != nil {
		return nil, err
	}
	status.TotalCredits = creditsFromQuota(int(total))
	return status, nil
}

// ClaimDailyCheckin pays today's reward. The unique trade number is what keeps
// it to one reward per account per day, including concurrent requests.
func ClaimDailyCheckin(userId int) (int, error) {
	if userId <= 0 {
		return 0, errors.New("invalid user id")
	}
	quotaPerCredit := DailyCheckinQuotaPerCredit()
	if quotaPerCredit <= 0 {
		return 0, errors.New("quota per unit is not configured")
	}
	now := time.Now()
	credits := drawDailyCheckinCredits()
	quota := credits * quotaPerCredit
	entry := &TopUp{
		UserId:          userId,
		Amount:          int64(quota),
		Money:           0,
		TradeNo:         dailyCheckinTradeNo(userId, dailyCheckinDayKey(now)),
		PaymentMethod:   DailyCheckinPaymentMethod,
		PaymentProvider: DailyCheckinPaymentMethod,
		CreateTime:      now.Unix(),
		CompleteTime:    now.Unix(),
		Status:          common.TopUpStatusSuccess,
	}
	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).Select("id").Where("id = ?", userId).First(&user).Error; err != nil {
			return err
		}
		if err := tx.Create(entry).Error; err != nil {
			// The unique trade number rejects a second claim in the same day.
			return ErrDailyCheckinAlreadyClaimed
		}
		return creditTopUpQuota(tx, userId, quota, nil)
	})
	if err != nil {
		return 0, err
	}
	syncCreditUserQuotaCache(userId, quota, common.LogText("daily check-in"))
	RecordLog(userId, LogTypeTopup, common.NewMessage("Daily check-in reward: {{credits}} credits", map[string]any{
		"credits": credits,
	}))
	return credits, nil
}

func creditsFromQuota(quota int) int {
	perCredit := DailyCheckinQuotaPerCredit()
	if perCredit <= 0 || quota <= 0 {
		return 0
	}
	return quota / perCredit
}
