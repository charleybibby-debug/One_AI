package model

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestReferralMigrationPreservesExistingWalletData(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "referral-migration.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}, &TopUp{}))

	user := User{Username: "referral-migration", Password: "password123", AffCode: "ref-migration", Quota: 1234}
	require.NoError(t, db.Create(&user).Error)
	topUp := TopUp{UserId: user.Id, TradeNo: "referral-migration-topup", Status: common.TopUpStatusSuccess}
	require.NoError(t, db.Create(&topUp).Error)

	require.NoError(t, db.AutoMigrate(&ReferralRecord{}))
	require.NoError(t, db.AutoMigrate(&ReferralRecord{}))
	require.True(t, db.Migrator().HasTable(&ReferralRecord{}))

	var migratedUser User
	require.NoError(t, db.First(&migratedUser, user.Id).Error)
	assert.Equal(t, 1234, migratedUser.Quota)
	var migratedTopUp TopUp
	require.NoError(t, db.First(&migratedTopUp, topUp.Id).Error)
	assert.Equal(t, common.TopUpStatusSuccess, migratedTopUp.Status)
}

func enableReferralPaymentsForTest(t *testing.T) {
	t.Helper()
	setting := operation_setting.GetPaymentSetting()
	previousConfirmed := setting.ComplianceConfirmed
	previousVersion := setting.ComplianceTermsVersion
	setting.ComplianceConfirmed = true
	setting.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion
	t.Cleanup(func() {
		setting.ComplianceConfirmed = previousConfirmed
		setting.ComplianceTermsVersion = previousVersion
	})
}

func TestReferralRewardsAndTransferAreIdempotent(t *testing.T) {
	truncateTables(t)
	enableReferralPaymentsForTest(t)

	inviter := User{Username: "referral-inviter", Password: "password123", AffCode: "ref-inviter"}
	require.NoError(t, DB.Create(&inviter).Error)
	invitee := User{Username: "referral-invitee", Password: "password123", AffCode: "ref-invitee", InviterId: inviter.Id}
	require.NoError(t, DB.Create(&invitee).Error)
	topUp := TopUp{
		UserId:       invitee.Id,
		Amount:       10,
		Money:        10,
		TradeNo:      "referral-topup",
		Status:       common.TopUpStatusSuccess,
		CompleteTime: time.Now().Unix(),
	}
	require.NoError(t, DB.Create(&topUp).Error)

	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return creditPaidTopUp(tx, &topUp, 10_000_000, nil)
	}))
	require.Error(t, DB.Transaction(func(tx *gorm.DB) error {
		return creditPaidTopUp(tx, &topUp, 10_000_000, nil)
	}))

	require.NoError(t, DB.First(&inviter, inviter.Id).Error)
	assert.Equal(t, 500_000, inviter.AffQuota)
	assert.Equal(t, 500_000, inviter.AffHistoryQuota)
	require.NoError(t, DB.First(&invitee, invitee.Id).Error)
	assert.Equal(t, 10_000_000, invitee.Quota)

	overview, err := GetReferralOverview(inviter.Id)
	require.NoError(t, err)
	assert.Equal(t, 1, overview.Overview.EffectiveInvites)
	assert.Equal(t, 500, overview.CurrentTier.RateBPS)
	assert.Equal(t, 500_000, overview.Overview.PendingRewardQuota)

	invitees, total, err := GetReferralInvitees(inviter.Id, &common.PageInfo{Page: 1, PageSize: 5})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, invitees, 1)
	assert.True(t, invitees[0].Effective)
	assert.Equal(t, 10_000_000, invitees[0].TopUpQuota)
	assert.Equal(t, 500_000, invitees[0].RewardQuota)

	requestID := "referral-transfer-request"
	require.NoError(t, inviter.TransferAffQuotaToQuotaIdempotent(500_000, requestID))
	require.NoError(t, inviter.TransferAffQuotaToQuotaIdempotent(500_000, requestID))
	require.NoError(t, DB.First(&inviter, inviter.Id).Error)
	assert.Equal(t, 0, inviter.AffQuota)
	assert.Equal(t, 500_000, inviter.Quota)

	records, total, err := GetReferralRecords(inviter.Id, &common.PageInfo{Page: 1, PageSize: 5})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, records, 1)
	assert.Equal(t, ReferralEventTopUpRebate, records[0].EventType)

	var transferCount int64
	require.NoError(t, DB.Model(&ReferralRecord{}).
		Where("event_type = ?", ReferralEventTransfer).
		Count(&transferCount).Error)
	assert.Equal(t, int64(1), transferCount)
}

func TestReferralRewardSkipsUsersWithoutInviter(t *testing.T) {
	truncateTables(t)
	enableReferralPaymentsForTest(t)

	user := User{Username: "referral-solo", Password: "password123", AffCode: "ref-solo"}
	require.NoError(t, DB.Create(&user).Error)
	topUp := TopUp{UserId: user.Id, TradeNo: "referral-solo-topup", Status: common.TopUpStatusSuccess}
	require.NoError(t, DB.Create(&topUp).Error)
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return creditPaidTopUp(tx, &topUp, 1000, nil)
	}))

	var count int64
	require.NoError(t, DB.Model(&ReferralRecord{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestReferralTransferRejectsWalletOverflow(t *testing.T) {
	truncateTables(t)
	minimumQuota := int(common.QuotaPerUnit)
	user := User{
		Username: "referral-overflow",
		Password: "password123",
		AffCode:  "ref-overflow",
		AffQuota: minimumQuota,
		Quota:    common.MaxWalletQuota,
	}
	require.NoError(t, DB.Create(&user).Error)

	err := user.TransferAffQuotaToQuotaIdempotent(minimumQuota, "referral-overflow-request")
	require.ErrorIs(t, err, ErrWalletQuotaLimitExceeded)
	require.NoError(t, DB.First(&user, user.Id).Error)
	assert.Equal(t, minimumQuota, user.AffQuota)
	assert.Equal(t, common.MaxWalletQuota, user.Quota)
}
