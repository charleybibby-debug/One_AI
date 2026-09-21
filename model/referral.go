package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const (
	ReferralEventTopUpRebate = "topup_rebate"
	ReferralEventTransfer    = "transfer"
)

type ReferralTier struct {
	MinInvites int `json:"min_invites"`
	RateBPS    int `json:"rate_bps"`
}

type ReferralConfig struct {
	Tiers []ReferralTier `json:"tiers"`
}

type ReferralOverviewStats struct {
	TotalInvites       int `json:"total_invites"`
	EffectiveInvites   int `json:"effective_invites"`
	InvitesToday       int `json:"invites_today"`
	PendingRewardQuota int `json:"pending_reward_quota"`
	TotalRewardQuota   int `json:"total_reward_quota"`
}

type ReferralOverview struct {
	Code             string                `json:"code"`
	Config           ReferralConfig        `json:"config"`
	Overview         ReferralOverviewStats `json:"overview"`
	CurrentTier      ReferralTier          `json:"current_tier"`
	CurrentTierIndex int                   `json:"current_tier_index"`
	NextTier         *ReferralTier         `json:"next_tier"`
}

type ReferralInvitee struct {
	InviteeId    int    `json:"invitee_id"`
	InviteeName  string `json:"invitee_name"`
	Effective    bool   `json:"effective"`
	RegisteredAt int64  `json:"registered_at"`
	LastPaidAt   int64  `json:"last_paid_at"`
	TopUpQuota   int    `json:"topup_quota"`
	RewardQuota  int    `json:"reward_quota"`
}

type ReferralRecord struct {
	Id          int     `json:"id"`
	InviterId   int     `json:"inviter_id" gorm:"index"`
	InviteeId   int     `json:"invitee_id" gorm:"index"`
	TopUpId     *int    `json:"topup_id,omitempty" gorm:"uniqueIndex"`
	EventType   string  `json:"event_type" gorm:"type:varchar(32);index"`
	BaseQuota   int     `json:"base_quota"`
	RateBPS     int     `json:"rate_bps"`
	RewardQuota int     `json:"reward_quota"`
	RequestId   *string `json:"-" gorm:"type:varchar(64);uniqueIndex"`
	CreatedAt   int64   `json:"created_at" gorm:"autoCreateTime"`
}

var defaultReferralTiers = []ReferralTier{
	{MinInvites: 0, RateBPS: 0},
	{MinInvites: 1, RateBPS: 500},
	{MinInvites: 10, RateBPS: 800},
	{MinInvites: 20, RateBPS: 1000},
}

func referralConfig() ReferralConfig {
	tiers := make([]ReferralTier, len(defaultReferralTiers))
	copy(tiers, defaultReferralTiers)
	return ReferralConfig{Tiers: tiers}
}

func referralTierForInvites(invites int) (ReferralTier, int, *ReferralTier) {
	config := referralConfig()
	currentIndex := 0
	for i := range config.Tiers {
		if config.Tiers[i].MinInvites > invites {
			break
		}
		currentIndex = i
	}

	current := config.Tiers[currentIndex]
	if currentIndex+1 >= len(config.Tiers) {
		return current, currentIndex, nil
	}
	next := config.Tiers[currentIndex+1]
	return current, currentIndex, &next
}

func referralEffectiveInviteQuery(tx *gorm.DB, inviterId int) *gorm.DB {
	paidTopUp := tx.Model(&TopUp{}).
		Select("1").
		Where("top_ups.user_id = users.id AND top_ups.status = ?", common.TopUpStatusSuccess)
	return tx.Model(&User{}).
		Where("inviter_id = ?", inviterId).
		Where("EXISTS (?)", paidTopUp)
}

func countEffectiveReferralInvitees(tx *gorm.DB, inviterId int) (int, error) {
	var count int64
	if err := referralEffectiveInviteQuery(tx, inviterId).Count(&count).Error; err != nil {
		return 0, err
	}
	return int(count), nil
}

func ensureReferralCode(user *User) error {
	if user.AffCode != "" {
		return nil
	}
	for range 5 {
		code := common.GetRandomString(4)
		result := DB.Model(&User{}).
			Where("id = ? AND aff_code = ?", user.Id, "").
			Update("aff_code", code)
		if result.Error == nil && result.RowsAffected == 1 {
			user.AffCode = code
			return nil
		}
		if result.Error != nil {
			var collisionCount int64
			if err := DB.Model(&User{}).Where("aff_code = ?", code).Count(&collisionCount).Error; err != nil {
				return result.Error
			}
			if collisionCount == 0 {
				return result.Error
			}
		}
		if err := DB.Select("aff_code").First(user, user.Id).Error; err != nil {
			return err
		}
		if user.AffCode != "" {
			return nil
		}
		if result.Error == nil {
			return gorm.ErrRecordNotFound
		}
	}
	return errors.New("failed to generate referral code")
}

func GetReferralOverview(userId int) (*ReferralOverview, error) {
	var user User
	if err := DB.Select("id", "aff_code", "aff_quota", "aff_history").First(&user, userId).Error; err != nil {
		return nil, err
	}
	if err := ensureReferralCode(&user); err != nil {
		return nil, err
	}

	var totalInvites int64
	if err := DB.Model(&User{}).Where("inviter_id = ?", userId).Count(&totalInvites).Error; err != nil {
		return nil, err
	}
	effectiveInvites, err := countEffectiveReferralInvitees(DB, userId)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()
	var invitesToday int64
	if err := DB.Model(&User{}).
		Where("inviter_id = ? AND created_at >= ?", userId, startOfDay).
		Count(&invitesToday).Error; err != nil {
		return nil, err
	}

	currentTier, currentTierIndex, nextTier := referralTierForInvites(effectiveInvites)
	return &ReferralOverview{
		Code:             user.AffCode,
		Config:           referralConfig(),
		CurrentTier:      currentTier,
		CurrentTierIndex: currentTierIndex,
		NextTier:         nextTier,
		Overview: ReferralOverviewStats{
			TotalInvites:       int(totalInvites),
			EffectiveInvites:   effectiveInvites,
			InvitesToday:       int(invitesToday),
			PendingRewardQuota: user.AffQuota,
			TotalRewardQuota:   user.AffHistoryQuota,
		},
	}, nil
}

func GetReferralInvitees(inviterId int, pageInfo *common.PageInfo) ([]ReferralInvitee, int64, error) {
	query := DB.Model(&User{}).Where("inviter_id = ?", inviterId)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var users []User
	if err := query.Select("id", "username", "display_name", "created_at").
		Order("id desc").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Find(&users).Error; err != nil {
		return nil, 0, err
	}
	if len(users) == 0 {
		return []ReferralInvitee{}, total, nil
	}

	userIds := make([]int, len(users))
	for i := range users {
		userIds[i] = users[i].Id
	}
	type topUpSummary struct {
		UserId     int
		LastPaidAt int64
	}
	var topUpSummaries []topUpSummary
	if err := DB.Model(&TopUp{}).
		Select("user_id, MAX(complete_time) AS last_paid_at").
		Where("user_id IN ? AND status = ?", userIds, common.TopUpStatusSuccess).
		Group("user_id").
		Scan(&topUpSummaries).Error; err != nil {
		return nil, 0, err
	}
	type rewardSummary struct {
		InviteeId   int
		TopUpQuota  int
		RewardQuota int
	}
	var rewardSummaries []rewardSummary
	if err := DB.Model(&ReferralRecord{}).
		Select("invitee_id, SUM(base_quota) AS top_up_quota, SUM(reward_quota) AS reward_quota").
		Where("inviter_id = ? AND invitee_id IN ? AND event_type = ?", inviterId, userIds, ReferralEventTopUpRebate).
		Group("invitee_id").
		Scan(&rewardSummaries).Error; err != nil {
		return nil, 0, err
	}

	lastPaidByUser := make(map[int]int64, len(topUpSummaries))
	for i := range topUpSummaries {
		lastPaidByUser[topUpSummaries[i].UserId] = topUpSummaries[i].LastPaidAt
	}
	rewardByUser := make(map[int]rewardSummary, len(rewardSummaries))
	for i := range rewardSummaries {
		rewardByUser[rewardSummaries[i].InviteeId] = rewardSummaries[i]
	}

	invitees := make([]ReferralInvitee, 0, len(users))
	for i := range users {
		name := users[i].DisplayName
		if name == "" {
			name = users[i].Username
		}
		reward := rewardByUser[users[i].Id]
		lastPaidAt := lastPaidByUser[users[i].Id]
		invitees = append(invitees, ReferralInvitee{
			InviteeId:    users[i].Id,
			InviteeName:  name,
			Effective:    lastPaidAt > 0,
			RegisteredAt: users[i].CreatedAt,
			LastPaidAt:   lastPaidAt,
			TopUpQuota:   reward.TopUpQuota,
			RewardQuota:  reward.RewardQuota,
		})
	}
	return invitees, total, nil
}

func GetReferralRecords(inviterId int, pageInfo *common.PageInfo) ([]ReferralRecord, int64, error) {
	query := DB.Model(&ReferralRecord{}).
		Where("inviter_id = ? AND event_type = ?", inviterId, ReferralEventTopUpRebate)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var records []ReferralRecord
	if err := query.Order("id desc").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Find(&records).Error; err != nil {
		return nil, 0, err
	}
	return records, total, nil
}

func creditPaidTopUp(tx *gorm.DB, topUp *TopUp, creditedQuota int, updates map[string]any) error {
	if err := creditTopUpQuota(tx, topUp.UserId, creditedQuota, updates); err != nil {
		return err
	}
	if !operation_setting.IsPaymentComplianceConfirmed() {
		return nil
	}

	var invitee User
	if err := tx.Select("id", "inviter_id").First(&invitee, topUp.UserId).Error; err != nil {
		return err
	}
	if invitee.InviterId <= 0 {
		return nil
	}
	var inviter User
	if err := lockForUpdate(tx).Select("id").First(&inviter, invitee.InviterId).Error; err != nil {
		return err
	}
	effectiveInvites, err := countEffectiveReferralInvitees(tx, invitee.InviterId)
	if err != nil {
		return err
	}
	tier, _, _ := referralTierForInvites(effectiveInvites)
	if tier.RateBPS <= 0 {
		return nil
	}
	rewardQuota, err := common.WalletQuotaFromDecimalStrict(
		decimal.NewFromInt(int64(creditedQuota)).
			Mul(decimal.NewFromInt(int64(tier.RateBPS))).
			Div(decimal.NewFromInt(10_000)),
	)
	if err != nil {
		return err
	}
	if rewardQuota <= 0 {
		return nil
	}

	maxCurrentQuota := common.MaxWalletQuota - rewardQuota
	result := tx.Model(&User{}).
		Where("id = ? AND aff_quota <= ? AND aff_history <= ?", invitee.InviterId, maxCurrentQuota, maxCurrentQuota).
		Updates(map[string]any{
			"aff_quota":   gorm.Expr("aff_quota + ?", rewardQuota),
			"aff_history": gorm.Expr("aff_history + ?", rewardQuota),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrWalletQuotaLimitExceeded
	}

	topUpId := topUp.Id
	return tx.Create(&ReferralRecord{
		InviterId:   invitee.InviterId,
		InviteeId:   invitee.Id,
		TopUpId:     &topUpId,
		EventType:   ReferralEventTopUpRebate,
		BaseQuota:   creditedQuota,
		RateBPS:     tier.RateBPS,
		RewardQuota: rewardQuota,
	}).Error
}

func (user *User) TransferAffQuotaToQuotaIdempotent(quota int, requestId string) error {
	if float64(quota) < common.QuotaPerUnit {
		return fmt.Errorf("转移额度最小为%s！", logger.LogQuota(common.QuotaFromFloat(common.QuotaPerUnit)))
	}
	requestId = strings.TrimSpace(requestId)
	if len(requestId) > 64 {
		return errors.New("request_id 过长")
	}

	credited := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := lockForUpdate(tx).First(user, user.Id).Error; err != nil {
			return err
		}
		if requestId != "" {
			var existing ReferralRecord
			err := tx.Where("request_id = ?", requestId).First(&existing).Error
			if err == nil {
				if existing.InviterId == user.Id && existing.EventType == ReferralEventTransfer && existing.RewardQuota == -quota {
					return nil
				}
				return errors.New("request_id 已被使用")
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
		if user.AffQuota < quota {
			return errors.New("邀请额度不足！")
		}
		if user.Quota > common.MaxWalletQuota-quota {
			return ErrWalletQuotaLimitExceeded
		}

		if err := tx.Model(user).Updates(map[string]any{
			"aff_quota": gorm.Expr("aff_quota - ?", quota),
			"quota":     gorm.Expr("quota + ?", quota),
		}).Error; err != nil {
			return err
		}
		var requestIdValue *string
		if requestId != "" {
			requestIdValue = &requestId
		}
		if err := tx.Create(&ReferralRecord{
			InviterId:   user.Id,
			EventType:   ReferralEventTransfer,
			BaseQuota:   quota,
			RewardQuota: -quota,
			RequestId:   requestIdValue,
		}).Error; err != nil {
			return err
		}
		user.AffQuota -= quota
		user.Quota += quota
		credited = true
		return nil
	})
	if err != nil {
		return err
	}
	if credited {
		syncCreditUserQuotaCache(user.Id, quota, "referral transfer")
	}
	return nil
}
