package model

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/glebarez/sqlite"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type legacyHierarchyUser struct {
	Id                   int            `gorm:"primaryKey"`
	Username             string         `gorm:"unique;index"`
	Password             string         `gorm:"not null;"`
	DisplayName          string         `gorm:"index"`
	Role                 int            `gorm:"type:int;default:1"`
	Status               int            `gorm:"type:int;default:1"`
	Email                string         `gorm:"index"`
	GitHubId             string         `gorm:"column:github_id;index"`
	DiscordId            string         `gorm:"column:discord_id;index"`
	OidcId               string         `gorm:"column:oidc_id;index"`
	WeChatId             string         `gorm:"column:wechat_id;index"`
	TelegramId           string         `gorm:"column:telegram_id;index"`
	AccessToken          *string        `gorm:"type:char(32);column:access_token;uniqueIndex"`
	AccessTokenCreatedAt *int64         `gorm:"type:bigint;column:access_token_created_at"`
	Quota                int            `gorm:"type:int;default:0"`
	UsedQuota            int            `gorm:"type:int;default:0;column:used_quota"`
	RequestCount         int            `gorm:"type:int;default:0;"`
	Group                string         `gorm:"type:varchar(64);default:'default'"`
	AffCode              string         `gorm:"type:varchar(32);column:aff_code;uniqueIndex"`
	AffCount             int            `gorm:"type:int;default:0;column:aff_count"`
	AffQuota             int            `gorm:"type:int;default:0;column:aff_quota"`
	AffHistoryQuota      int            `gorm:"type:int;default:0;column:aff_history"`
	InviterId            int            `gorm:"type:int;column:inviter_id;index"`
	DeletedAt            gorm.DeletedAt `gorm:"index"`
	LinuxDOId            string         `gorm:"column:linux_do_id;index"`
	Setting              string         `gorm:"type:text;column:setting"`
	Remark               string         `gorm:"type:varchar(255)"`
	StripeCustomer       string         `gorm:"type:varchar(64);column:stripe_customer;index"`
	CreatedAt            int64          `gorm:"autoCreateTime;column:created_at"`
	LastLoginAt          int64          `gorm:"default:0;column:last_login_at"`
	AuthVersion          int64          `gorm:"type:bigint;not null;default:1;column:auth_version"`
}

func setupUserUpdateTestState(t *testing.T) {
	t.Helper()
	truncateTables(t)
	require.NoError(t, DB.Exec("DELETE FROM users").Error)

	oldRedisEnabled := common.RedisEnabled
	oldBatchUpdateEnabled := common.BatchUpdateEnabled
	common.RedisEnabled = false
	common.BatchUpdateEnabled = false
	t.Cleanup(func() {
		common.RedisEnabled = oldRedisEnabled
		common.BatchUpdateEnabled = oldBatchUpdateEnabled
	})
}

func createUserBindTestUser(t *testing.T) User {
	t.Helper()
	user := User{
		Username:    "bind-test-user",
		Password:    "unused-password-hash",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		AuthVersion: 1,
		AffCode:     "bind-test-aff-code",
	}
	require.NoError(t, DB.Create(&user).Error)
	return user
}

func TestUserUpdateDoesNotOverwriteConcurrentAccountingOrTokenChanges(t *testing.T) {
	setupUserUpdateTestState(t)

	user := User{
		Id:              1,
		Username:        "quota-race-user",
		Password:        "password",
		DisplayName:     "before",
		Status:          common.UserStatusEnabled,
		Quota:           1000,
		UsedQuota:       20,
		RequestCount:    3,
		AffCount:        2,
		AffQuota:        800,
		AffHistoryQuota: 1200,
	}
	user.SetAccessToken("old-token")
	require.NoError(t, DB.Create(&user).Error)

	staleUser, err := GetUserById(user.Id, true)
	require.NoError(t, err)

	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).Updates(map[string]any{
		"quota":         gorm.Expr("quota - ?", 400),
		"used_quota":    gorm.Expr("used_quota + ?", 400),
		"request_count": gorm.Expr("request_count + ?", 1),
		"aff_count":     gorm.Expr("aff_count + ?", 1),
		"aff_quota":     gorm.Expr("aff_quota - ?", 500),
		"aff_history":   gorm.Expr("aff_history + ?", 500),
		"access_token":  "rotated-token",
	}).Error)

	staleUser.DisplayName = "after"
	require.NoError(t, staleUser.Update(false))

	var got User
	require.NoError(t, DB.First(&got, user.Id).Error)
	assert.Equal(t, "after", got.DisplayName)
	assert.Equal(t, 600, got.Quota)
	assert.Equal(t, 420, got.UsedQuota)
	assert.Equal(t, 4, got.RequestCount)
	assert.Equal(t, 3, got.AffCount)
	assert.Equal(t, 300, got.AffQuota)
	assert.Equal(t, 1700, got.AffHistoryQuota)
	assert.Equal(t, "rotated-token", got.GetAccessToken())
}

func TestUsageAccountingSupportsSignedDirectAndBatchDeltas(t *testing.T) {
	setupUserUpdateTestState(t)
	resetBatchUpdateTestState(t)

	user := User{
		Id:           10,
		Username:     "usage-adjustment-user",
		Password:     "password",
		Status:       common.UserStatusEnabled,
		UsedQuota:    1000,
		RequestCount: 3,
	}
	channel := Channel{
		Id:        10,
		Name:      "usage-adjustment-channel",
		Key:       "sk-test",
		Status:    common.ChannelStatusEnabled,
		UsedQuota: 1000,
	}
	require.NoError(t, DB.Create(&user).Error)
	require.NoError(t, DB.Create(&channel).Error)

	UpdateUserUsedQuota(user.Id, -200)
	UpdateUserUsedQuota(user.Id, 50)
	UpdateChannelUsedQuota(channel.Id, -200)
	UpdateChannelUsedQuota(channel.Id, 50)

	var got User
	require.NoError(t, DB.Select("used_quota", "request_count").First(&got, user.Id).Error)
	assert.Equal(t, 850, got.UsedQuota)
	assert.Equal(t, 3, got.RequestCount)
	var gotChannel Channel
	require.NoError(t, DB.Select("used_quota").First(&gotChannel, channel.Id).Error)
	assert.Equal(t, int64(850), gotChannel.UsedQuota)

	common.BatchUpdateEnabled = true
	UpdateUserUsedQuota(user.Id, 400)
	UpdateUserUsedQuota(user.Id, -100)
	UpdateChannelUsedQuota(channel.Id, 400)
	UpdateChannelUsedQuota(channel.Id, -100)

	require.NoError(t, DB.Select("used_quota", "request_count").First(&got, user.Id).Error)
	assert.Equal(t, 850, got.UsedQuota, "batch deltas must remain queued until flush")
	assert.Equal(t, 3, got.RequestCount)
	require.NoError(t, DB.Select("used_quota").First(&gotChannel, channel.Id).Error)
	assert.Equal(t, int64(850), gotChannel.UsedQuota, "batch deltas must remain queued until flush")

	batchUpdate()
	require.NoError(t, DB.Select("used_quota", "request_count").First(&got, user.Id).Error)
	assert.Equal(t, 1150, got.UsedQuota)
	assert.Equal(t, 3, got.RequestCount)
	require.NoError(t, DB.Select("used_quota").First(&gotChannel, channel.Id).Error)
	assert.Equal(t, int64(1150), gotChannel.UsedQuota)
}

func TestUserAccountHierarchyOnlyAllowsDirectMembersOfActiveChannelAccounts(t *testing.T) {
	setupUserUpdateTestState(t)
	channel := User{
		Username: "hierarchy-channel", Password: "unused", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, Group: "default", AffCode: "hierarchy-channel-aff", AccountType: UserAccountTypeChannel,
	}
	require.NoError(t, DB.Create(&channel).Error)

	assert.NoError(t, ValidateUserAccountHierarchy(DB, 0, UserAccountTypeStandard, channel.Id))
	assert.Error(t, ValidateUserAccountHierarchy(DB, channel.Id, UserAccountTypeStandard, channel.Id))
	assert.Error(t, ValidateUserAccountHierarchy(DB, 0, UserAccountTypeChannel, channel.Id))

	disabledChannel := User{
		Username: "hierarchy-disabled-channel", Password: "unused", Role: common.RoleCommonUser,
		Status: common.UserStatusDisabled, Group: "default", AffCode: "hierarchy-disabled-aff", AccountType: UserAccountTypeChannel,
	}
	require.NoError(t, DB.Create(&disabledChannel).Error)
	assert.Error(t, ValidateUserAccountHierarchy(DB, 0, UserAccountTypeStandard, disabledChannel.Id))

	admin := User{
		Username: "hierarchy-admin", Password: "unused", Role: common.RoleAdminUser,
		Status: common.UserStatusEnabled, Group: "default", AffCode: "hierarchy-admin-aff", AccountType: UserAccountTypeChannel,
	}
	require.NoError(t, DB.Create(&admin).Error)
	assert.Error(t, ValidateUserAccountHierarchy(DB, 0, UserAccountTypeStandard, admin.Id))
}

func TestUserAccountHierarchyPreventsDemotingChannelWithAssignedUsers(t *testing.T) {
	setupUserUpdateTestState(t)
	channel := User{
		Username: "hierarchy-demotion-channel", Password: "unused", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, Group: "default", AffCode: "hierarchy-demotion-aff", AccountType: UserAccountTypeChannel,
	}
	require.NoError(t, DB.Create(&channel).Error)
	member := User{
		Username: "hierarchy-member", Password: "unused", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, Group: "default", AffCode: "hierarchy-member-aff", AccountType: UserAccountTypeStandard,
		ChannelOwnerId: channel.Id,
	}
	require.NoError(t, DB.Create(&member).Error)

	assert.Error(t, ValidateUserAccountHierarchy(DB, channel.Id, UserAccountTypeStandard, 0))
}

func TestUserAccountHierarchyPreventsDeletingChannelWithAssignedUsers(t *testing.T) {
	setupUserUpdateTestState(t)
	channel := User{
		Username: "hierarchy-delete-channel", Password: "unused", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, Group: "default", AffCode: "hierarchy-delete-channel-aff", AccountType: UserAccountTypeChannel,
	}
	require.NoError(t, DB.Create(&channel).Error)
	member := User{
		Username: "hierarchy-delete-member", Password: "unused", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, Group: "default", AffCode: "hierarchy-delete-member-aff", AccountType: UserAccountTypeStandard,
		ChannelOwnerId: channel.Id,
	}
	require.NoError(t, DB.Create(&member).Error)

	assert.Error(t, validateChannelAccountDeletion(DB, channel.Id))

	require.NoError(t, DB.Model(&member).Update("channel_owner_id", 0).Error)
	assert.NoError(t, validateChannelAccountDeletion(DB, channel.Id))
}

func TestUserAccountHierarchyMigrationPreservesLegacyUsers(t *testing.T) {
	tests := []struct {
		name string
		dsn  string
		open func(string) gorm.Dialector
	}{
		{
			name: "sqlite",
			dsn:  filepath.Join(t.TempDir(), "account-hierarchy.sqlite"),
			open: sqlite.Open,
		},
		{
			name: "mysql",
			dsn:  strings.TrimSpace(os.Getenv("TEST_MYSQL_DSN")),
			open: mysql.Open,
		},
		{
			name: "postgres",
			dsn:  strings.TrimSpace(os.Getenv("TEST_POSTGRES_DSN")),
			open: func(dsn string) gorm.Dialector {
				return postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true})
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.dsn == "" {
				t.Skip("database DSN is not configured")
			}
			db, err := gorm.Open(test.open(test.dsn), &gorm.Config{})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			t.Cleanup(func() { _ = sqlDB.Close() })

			versionQuery := "SELECT VERSION()"
			if test.name == "sqlite" {
				versionQuery = "SELECT sqlite_version()"
			} else if test.name == "postgres" {
				versionQuery = "SHOW server_version"
			}
			var version string
			require.NoError(t, db.Raw(versionQuery).Scan(&version).Error)
			t.Logf("database: %s %s", test.name, version)

			const upgradeTable = "user_account_hierarchy_upgrade_test"
			const freshTable = "user_account_hierarchy_fresh_test"
			for _, table := range []string{upgradeTable, freshTable} {
				require.NoError(t, db.Migrator().DropTable(table))
				t.Cleanup(func() { _ = db.Migrator().DropTable(table) })
			}

			require.NoError(t, db.Table(upgradeTable).AutoMigrate(&legacyHierarchyUser{}))
			legacy := legacyHierarchyUser{Id: 1, Username: "existing-user", Password: "hash", AffCode: "existing-aff"}
			require.NoError(t, db.Table(upgradeTable).Create(&legacy).Error)
			require.NoError(t, db.Table(upgradeTable).AutoMigrate(&User{}))
			require.NoError(t, db.Table(upgradeTable).AutoMigrate(&User{}))

			var migrated User
			require.NoError(t, db.Table(upgradeTable).First(&migrated, legacy.Id).Error)
			assert.Equal(t, UserAccountTypeStandard, migrated.AccountType)
			assert.Zero(t, migrated.ChannelOwnerId)

			require.NoError(t, db.Table(freshTable).AutoMigrate(&User{}))
			require.NoError(t, db.Table(freshTable).AutoMigrate(&User{}))
			fresh := User{Username: "fresh-user", Password: "hash", Status: common.UserStatusEnabled, Group: "default"}
			require.NoError(t, db.Table(freshTable).Create(&fresh).Error)
			assert.Equal(t, UserAccountTypeStandard, fresh.AccountType)
			assert.Zero(t, fresh.ChannelOwnerId)
		})
	}
}

func TestRevokeUserAccessTokenOnlyClearsLegacyToken(t *testing.T) {
	setupUserUpdateTestState(t)

	createdAt := int64(100)
	user := User{
		Id:                   2,
		Username:             "token-revoke-user",
		Password:             "password",
		DisplayName:          "before",
		Status:               common.UserStatusEnabled,
		Quota:                1000,
		AffQuota:             800,
		AffHistoryQuota:      1200,
		AccessTokenCreatedAt: &createdAt,
	}
	user.SetAccessToken("legacy-token")
	require.NoError(t, DB.Create(&user).Error)

	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).Updates(map[string]any{
		"quota":        gorm.Expr("quota + ?", 500),
		"aff_quota":    gorm.Expr("aff_quota - ?", 500),
		"display_name": "concurrent-update",
	}).Error)

	tokenRef, err := RevokeUserAccessToken(user.Id)
	require.NoError(t, err)
	assert.Equal(t, AccessTokenFingerprint("legacy-token"), tokenRef)

	var got User
	require.NoError(t, DB.First(&got, user.Id).Error)
	assert.Empty(t, got.GetAccessToken())
	assert.Nil(t, got.AccessTokenCreatedAt)
	assert.Equal(t, "concurrent-update", got.DisplayName)
	assert.Equal(t, 1500, got.Quota)
	assert.Equal(t, 300, got.AffQuota)
	assert.Equal(t, 1200, got.AffHistoryQuota)
}

func TestRevokeUserAccessTokenRejectsSoftDeletedUser(t *testing.T) {
	setupUserUpdateTestState(t)

	user := User{
		Id:       3,
		Username: "deleted-token-revoke-user",
		Password: "password",
		Status:   common.UserStatusEnabled,
	}
	user.SetAccessToken("old-token")
	require.NoError(t, DB.Create(&user).Error)
	require.NoError(t, DB.Delete(&user).Error)

	_, err := RevokeUserAccessToken(user.Id)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)

	var got User
	require.NoError(t, DB.Unscoped().First(&got, user.Id).Error)
	assert.Equal(t, "old-token", got.GetAccessToken())
}

func TestUpdateUserSettingOnlyUpdatesSetting(t *testing.T) {
	setupUserUpdateTestState(t)

	user := User{
		Id:           2,
		Username:     "setting-user",
		Password:     "password",
		Status:       common.UserStatusEnabled,
		Quota:        1000,
		UsedQuota:    20,
		RequestCount: 3,
	}
	require.NoError(t, DB.Create(&user).Error)

	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).Updates(map[string]any{
		"quota":         gorm.Expr("quota - ?", 250),
		"used_quota":    gorm.Expr("used_quota + ?", 250),
		"request_count": gorm.Expr("request_count + ?", 1),
	}).Error)

	require.NoError(t, UpdateUserSetting(user.Id, dto.UserSetting{Language: "zh"}))

	var got User
	require.NoError(t, DB.First(&got, user.Id).Error)
	assert.Equal(t, 750, got.Quota)
	assert.Equal(t, 270, got.UsedQuota)
	assert.Equal(t, 4, got.RequestCount)
	assert.Equal(t, "zh", got.GetSetting().Language)
}

func TestEnsureEmailAvailableRejectsExistingEmailCaseInsensitive(t *testing.T) {
	setupUserUpdateTestState(t)

	require.NoError(t, DB.Create(&User{
		Username: "existing",
		Password: "old-password",
		Email:    "Taken@Example.com",
		Status:   common.UserStatusEnabled,
	}).Error)

	err := EnsureEmailAvailable(" taken@example.COM ", 0)
	require.ErrorIs(t, err, ErrEmailAlreadyTaken)

	user, err := GetUniqueUserByEmail("TAKEN@example.com")
	require.NoError(t, err)
	assert.Equal(t, "existing", user.Username)

	require.NoError(t, EnsureEmailAvailable("taken@example.com", user.Id))
}

func TestInsertRejectsDuplicateEmailWithoutUniqueIndex(t *testing.T) {
	setupUserUpdateTestState(t)

	require.NoError(t, DB.Create(&User{
		Username: "existing",
		Password: "old-password",
		Email:    "taken@example.com",
		Status:   common.UserStatusEnabled,
	}).Error)

	user := &User{
		Username: "oauth-user",
		Email:    "TAKEN@example.com",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	}

	err := user.Insert(0)
	require.ErrorIs(t, err, ErrEmailAlreadyTaken)

	var count int64
	require.NoError(t, DB.Model(&User{}).Where("username = ?", "oauth-user").Count(&count).Error)
	assert.Zero(t, count)
}

func TestInsertKeepsBlankPasswordForPasswordlessUser(t *testing.T) {
	setupUserUpdateTestState(t)

	user := &User{
		Username: "passwordless-user",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	}

	require.NoError(t, user.Insert(0))

	var stored User
	require.NoError(t, DB.Where("username = ?", user.Username).First(&stored).Error)
	assert.Empty(t, stored.Password)
}

func TestUpdateUserBindColumnOnlyTouchesTheBindingColumn(t *testing.T) {
	truncateTables(t)

	user := createUserBindTestUser(t)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).Updates(map[string]any{
		"role":   common.RoleAdminUser,
		"status": common.UserStatusEnabled,
		"group":  "vip",
	}).Error)

	require.NoError(t, UpdateUserBindColumn(user.Id, "github_id", "gh-12345"))

	reloaded, err := GetUserById(user.Id, true)
	require.NoError(t, err)
	assert.Equal(t, "gh-12345", reloaded.GitHubId)
	assert.Equal(t, common.RoleAdminUser, reloaded.Role)
	assert.Equal(t, common.UserStatusEnabled, reloaded.Status)
	assert.Equal(t, "vip", reloaded.Group)
}

func TestUpdateUserBindColumnPreservesRestrictiveChange(t *testing.T) {
	truncateTables(t)

	user := createUserBindTestUser(t)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).
		Update("status", common.UserStatusDisabled).Error)
	require.NoError(t, UpdateUserBindColumn(user.Id, "wechat_id", "wx-open-id"))

	reloaded, err := GetUserById(user.Id, true)
	require.NoError(t, err)
	assert.Equal(t, "wx-open-id", reloaded.WeChatId)
	assert.Equal(t, common.UserStatusDisabled, reloaded.Status)
}

func TestUpdateUserBindColumnRejectsNonWhitelistedColumns(t *testing.T) {
	truncateTables(t)

	user := createUserBindTestUser(t)
	for _, column := range []string{"role", "status", "group", "quota", "username", "password", "id"} {
		assert.Error(t, UpdateUserBindColumn(user.Id, column, "1"), "column %s must be rejected", column)
	}
	assert.Error(t, UpdateUserBindColumn(user.Id, "github_id; DROP TABLE users", "x"))
	assert.Error(t, UpdateUserBindColumn(0, "github_id", "x"))
}

func TestValidateAndFillRejectsPasswordlessUser(t *testing.T) {
	setupUserUpdateTestState(t)

	require.NoError(t, DB.Create(&User{
		Username: "passwordless-user",
		Password: "",
		Status:   common.UserStatusEnabled,
	}).Error)

	loginUser := User{
		Username: "passwordless-user",
		Password: "NewPassword123",
	}
	err := loginUser.ValidateAndFill()
	require.ErrorIs(t, err, ErrInvalidCredentials)

	var stored User
	require.NoError(t, DB.Where("username = ?", "passwordless-user").First(&stored).Error)
	assert.Empty(t, stored.Password)
}

func TestResetUserPasswordByEmailRequiresSingleActiveMatch(t *testing.T) {
	setupUserUpdateTestState(t)

	require.NoError(t, DB.Create(&User{
		Username: "duplicate-1",
		Password: "old-1",
		Email:    "legacy@example.com",
		AffCode:  "dupe1",
		Status:   common.UserStatusEnabled,
	}).Error)
	require.NoError(t, DB.Create(&User{
		Username: "duplicate-2",
		Password: "old-2",
		Email:    "LEGACY@example.com",
		AffCode:  "dupe2",
		Status:   common.UserStatusEnabled,
	}).Error)

	err := ResetUserPasswordByEmail("legacy@example.com", "NewPassword123")
	require.ErrorIs(t, err, ErrEmailAmbiguous)

	var duplicates []User
	require.NoError(t, DB.Where("LOWER(email) = ?", "legacy@example.com").Order("username asc").Find(&duplicates).Error)
	require.Len(t, duplicates, 2)
	assert.Equal(t, "old-1", duplicates[0].Password)
	assert.Equal(t, "old-2", duplicates[1].Password)

	require.NoError(t, DB.Create(&User{
		Username: "unique",
		Password: "old",
		Email:    "unique@example.com",
		AffCode:  "unique",
		Status:   common.UserStatusEnabled,
	}).Error)

	require.NoError(t, ResetUserPasswordByEmail("UNIQUE@example.com", "NewPassword123"))

	var unique User
	require.NoError(t, DB.Where("username = ?", "unique").First(&unique).Error)
	assert.True(t, common.ValidatePasswordAndHash("NewPassword123", unique.Password))

	err = ResetUserPasswordByEmail("missing@example.com", "NewPassword123")
	require.True(t, errors.Is(err, ErrEmailNotFound))
}

func createAccessTokenTestUser(t *testing.T, username string) User {
	t.Helper()
	user := User{
		Username:    username,
		Password:    "unused-password-hash",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		AuthVersion: 1,
		AffCode:     username + "-aff",
	}
	require.NoError(t, DB.Create(&user).Error)
	return user
}

func createTestUserAccessToken(t *testing.T, userID int, raw string, expiresAt int64) UserAccessToken {
	t.Helper()
	token := UserAccessToken{Name: raw, TokenHash: AccessTokenFingerprint(raw), TokenHint: AccessTokenHint(raw), ExpiresAt: expiresAt}
	require.NoError(t, token.SetScopes([]string{"profile:read"}))
	require.NoError(t, CreateUserAccessToken(userID, &token, 20))
	return token
}

func TestLegacyAccessTokenRetireAtIsWrittenOnceAndReadOnly(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Option{}))
	previous := legacyAccessTokenRetireAt.Load()
	originalMap := common.OptionMap
	common.OptionMap = map[string]string{}
	require.NoError(t, DB.Where(&Option{Key: legacyAccessTokenRetireAtKey}).Delete(&Option{}).Error)
	t.Cleanup(func() {
		legacyAccessTokenRetireAt.Store(previous)
		common.OptionMap = originalMap
		DB.Where(&Option{Key: legacyAccessTokenRetireAtKey}).Delete(&Option{})
	})

	const firstStart = int64(1_800_000_000)
	want := firstStart + 30*24*60*60
	require.NoError(t, EnsureLegacyAccessTokenRetireAt(firstStart))
	assert.Equal(t, want, LegacyAccessTokenRetireAt())

	require.NoError(t, EnsureLegacyAccessTokenRetireAt(firstStart+24*60*60))
	assert.Equal(t, want, LegacyAccessTokenRetireAt())
	var stored Option
	require.NoError(t, DB.Where(&Option{Key: legacyAccessTokenRetireAtKey}).First(&stored).Error)
	assert.Equal(t, strconv.FormatInt(want, 10), stored.Value)

	require.ErrorIs(t, UpdateOption(legacyAccessTokenRetireAtKey, "1"), errLegacyRetireAtReadOnly)
	require.ErrorIs(t, UpdateOptionsBulk(map[string]string{legacyAccessTokenRetireAtKey: "1"}), errLegacyRetireAtReadOnly)
	require.NoError(t, DB.Where(&Option{Key: legacyAccessTokenRetireAtKey}).First(&stored).Error)
	assert.Equal(t, strconv.FormatInt(want, 10), stored.Value)

	require.NoError(t, updateOptionMap(legacyAccessTokenRetireAtKey, stored.Value))
	assert.NotContains(t, common.OptionMap, legacyAccessTokenRetireAtKey)
}

func TestValidateAccessTokenStopsAtLegacyDeadline(t *testing.T) {
	setupUserUpdateTestState(t)
	previous := legacyAccessTokenRetireAt.Load()
	t.Cleanup(func() { legacyAccessTokenRetireAt.Store(previous) })

	user := createAccessTokenTestUser(t, "legacy-deadline-user")
	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).Update("access_token", "legacy-deadline-token").Error)

	legacyAccessTokenRetireAt.Store(time.Now().Add(time.Hour).Unix())
	found, err := ValidateAccessToken("legacy-deadline-token")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, user.Id, found.Id)

	var userQueries int
	const callbackName = "test:count_legacy_token_queries"
	require.NoError(t, DB.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Table == "users" {
			userQueries++
		}
	}))
	t.Cleanup(func() { _ = DB.Callback().Query().Remove(callbackName) })

	for _, retireAt := range []int64{time.Now().Add(-time.Second).Unix(), 0} {
		legacyAccessTokenRetireAt.Store(retireAt)
		found, err = ValidateAccessToken("legacy-deadline-token")
		require.ErrorIs(t, err, ErrLegacyAccessTokenRetired)
		assert.Nil(t, found)
	}
	assert.Zero(t, userQueries)
}

func TestUserAccessTokenLimitAndOwnership(t *testing.T) {
	setupUserUpdateTestState(t)
	owner := createAccessTokenTestUser(t, "token-owner")
	other := createAccessTokenTestUser(t, "token-other")

	var first UserAccessToken
	for i := range 20 {
		token := createTestUserAccessToken(t, owner.Id, "nap_limit_"+strconv.Itoa(i), 0)
		if i == 0 {
			first = token
		}
	}
	extra := UserAccessToken{TokenHash: AccessTokenFingerprint("nap_limit_extra")}
	require.ErrorIs(t, CreateUserAccessToken(owner.Id, &extra, 20), ErrAccessTokenLimit)
	var count int64
	require.NoError(t, DB.Model(&UserAccessToken{}).Where("user_id = ?", owner.Id).Count(&count).Error)
	assert.EqualValues(t, 20, count)

	_, err := UpdateUserAccessToken(other.Id, first.Id, "stolen", []string{"profile:read"})
	require.ErrorIs(t, err, ErrAccessTokenNotFound)
	_, err = DeleteUserAccessToken(other.Id, first.Id)
	require.ErrorIs(t, err, ErrAccessTokenNotFound)
	stored, err := GetUserAccessToken(owner.Id, first.Id)
	require.NoError(t, err)
	assert.Equal(t, first.Name, stored.Name)

	assert.Equal(t, first.Scopes, stored.Scopes)

	renamed, err := UpdateUserAccessToken(owner.Id, first.Id, "renamed", nil)
	require.NoError(t, err)
	assert.Equal(t, "renamed", renamed.Name)
	assert.Equal(t, first.Scopes, renamed.Scopes, "a rename keeps the grant")
	regranted, err := UpdateUserAccessToken(owner.Id, first.Id, "regranted", []string{"usage:read"})
	require.NoError(t, err)
	assert.Equal(t, "regranted", regranted.Name)
	assert.Equal(t, []string{"usage:read"}, regranted.GetScopes())
	deleted, err := DeleteUserAccessToken(owner.Id, first.Id)
	require.NoError(t, err)
	assert.Equal(t, first.TokenHash, deleted.TokenHash)
	found, err := FindUserAccessTokenByHash(first.TokenHash)
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestDisablingUserKeepsAccessTokensAndDeletingRemovesThem(t *testing.T) {
	setupUserUpdateTestState(t)
	user := createAccessTokenTestUser(t, "token-lifecycle-user")
	createTestUserAccessToken(t, user.Id, "nap_lifecycle_one", 0)
	createTestUserAccessToken(t, user.Id, "nap_lifecycle_two", time.Now().Add(time.Hour).Unix())
	countTokens := func() int64 {
		var count int64
		require.NoError(t, DB.Model(&UserAccessToken{}).Where("user_id = ?", user.Id).Count(&count).Error)
		return count
	}

	user.Status = common.UserStatusDisabled
	require.NoError(t, user.Update(false))
	assert.EqualValues(t, 2, countTokens())

	revoked, err := user.Delete()
	require.NoError(t, err)
	assert.EqualValues(t, 2, revoked)
	assert.Zero(t, countTokens())

	hardDeleted := createAccessTokenTestUser(t, "token-hard-delete-user")
	createTestUserAccessToken(t, hardDeleted.Id, "nap_hard_delete", 0)
	revoked, err = hardDeleted.HardDelete()
	require.NoError(t, err)
	assert.EqualValues(t, 1, revoked)
	var remaining int64
	require.NoError(t, DB.Model(&UserAccessToken{}).Where("user_id = ?", hardDeleted.Id).Count(&remaining).Error)
	assert.Zero(t, remaining)
}

func TestValidateAuthSessionWithTxChecksAccessTokenIdentity(t *testing.T) {
	setupUserUpdateTestState(t)
	now := time.Now()

	tests := []struct {
		name    string
		prepare func(user *User, token *UserAccessToken, identity *AuthSessionIdentity)
		wantErr bool
	}{
		{name: "active token", prepare: func(*User, *UserAccessToken, *AuthSessionIdentity) {}},
		{name: "expired token", wantErr: true, prepare: func(_ *User, token *UserAccessToken, _ *AuthSessionIdentity) {
			require.NoError(t, DB.Model(token).Update("expires_at", now.Add(-time.Second).Unix()).Error)
		}},
		{name: "deleted token", wantErr: true, prepare: func(user *User, token *UserAccessToken, _ *AuthSessionIdentity) {
			_, err := DeleteUserAccessToken(user.Id, token.Id)
			require.NoError(t, err)
		}},
		{name: "another user's token", wantErr: true, prepare: func(_ *User, _ *UserAccessToken, identity *AuthSessionIdentity) {
			other := createAccessTokenTestUser(t, "token-identity-other")
			identity.UserID = other.Id
		}},
		{name: "stale auth version", wantErr: true, prepare: func(user *User, _ *UserAccessToken, _ *AuthSessionIdentity) {
			require.NoError(t, DB.Model(user).Update("auth_version", 2).Error)
		}},
		{name: "disabled user", wantErr: true, prepare: func(user *User, _ *UserAccessToken, _ *AuthSessionIdentity) {
			require.NoError(t, DB.Model(user).Update("status", common.UserStatusDisabled).Error)
		}},
		{name: "unexpected session version", wantErr: true, prepare: func(_ *User, _ *UserAccessToken, identity *AuthSessionIdentity) {
			identity.SessionVersion = 2
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, DB.Exec("DELETE FROM user_access_tokens").Error)
			require.NoError(t, DB.Exec("DELETE FROM users").Error)
			user := createAccessTokenTestUser(t, "token-identity-user")
			token := createTestUserAccessToken(t, user.Id, "nap_identity_token", now.Add(time.Hour).Unix())
			identity := AuthSessionIdentity{
				UserID: user.Id, SessionID: AccessTokenSessionID(token.Id),
				UserAuthVersion: 1, SessionVersion: AccessTokenSessionVersion,
			}
			tt.prepare(&user, &token, &identity)

			txErr := DB.Transaction(func(tx *gorm.DB) error { return ValidateAuthSessionWithTx(tx, identity) })
			plainErr := ValidateAccessTokenIdentity(identity)
			if !tt.wantErr {
				assert.NoError(t, txErr)
				assert.NoError(t, plainErr)
				return
			}
			assert.ErrorIs(t, txErr, ErrUserSessionInactive)
			assert.ErrorIs(t, plainErr, ErrUserSessionInactive)
		})
	}
}
