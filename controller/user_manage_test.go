package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/oauth"
	"github.com/QuantumNous/new-api/service/authz"
	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupManageUserTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	require.NoError(t, i18n.Init())
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousRedisEnabled := common.RedisEnabled
	previousMainDatabaseType, previousLogDatabaseType := common.MainDatabaseType(), common.LogDatabaseType()
	dialect := os.Getenv("TEST_MANAGE_USER_DIALECT")
	if dialect == "" {
		dialect = "sqlite"
	}
	databaseTypes := map[string]common.DatabaseType{
		"sqlite": common.DatabaseTypeSQLite, "mysql": common.DatabaseTypeMySQL, "postgres": common.DatabaseTypePostgreSQL,
	}
	require.Contains(t, databaseTypes, dialect)
	dsn := os.Getenv("TEST_" + strings.ToUpper(dialect) + "_DSN")
	db, _ := newAuditTestDatabase(t, dialect, dsn)
	logDB := db
	if os.Getenv("TEST_MANAGE_USER_SEPARATE_LOG_DB") == "1" {
		logDB, _ = newAuditTestDatabase(t, dialect, dsn)
	}
	model.DB, model.LOG_DB = db, logDB
	common.RedisEnabled = false
	common.SetDatabaseTypes(databaseTypes[dialect], databaseTypes[dialect])

	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.RedisEnabled = previousRedisEnabled
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		if logDB != db {
			sqlLogDB, err := logDB.DB()
			if err == nil {
				_ = sqlLogDB.Close()
			}
		}
	})
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.UserSession{}, &model.CasbinRule{}, &model.AuthzRole{}))
	require.NoError(t, logDB.AutoMigrate(&model.Log{}, &model.AuditLog{}))
	versionQuery := "SELECT version()"
	if dialect == "sqlite" {
		versionQuery = "SELECT sqlite_version()"
	}
	var version string
	require.NoError(t, db.Raw(versionQuery).Scan(&version).Error)
	t.Logf("database: %s %s, separate log database: %v", dialect, version, logDB != db)
	return db
}

func performManageUserRequest(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/user/manage", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("id", 9999)
	c.Set("role", common.RoleRootUser)
	c.Set("username", "root-operator")
	c.Set(common.RequestIdKey, "quota-test-request")
	ManageUser(c)
	return recorder
}

func performAdminUserReadRequest(t *testing.T, role int, method, path string, handler gin.HandlerFunc, targetID ...int) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(method, path, nil)
	c.Set("id", 9999)
	c.Set("role", role)
	c.Set("username", "user-read-operator")
	if len(targetID) > 0 {
		c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(targetID[0])}}
	}
	handler(c)
	return recorder
}

func performChannelUserRequest(t *testing.T, manager model.User, method, path, body string, handler gin.HandlerFunc, targetID ...int) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(method, path, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("id", manager.Id)
	c.Set("role", manager.Role)
	c.Set("username", manager.Username)
	c.Set(common.RequestIdKey, "channel-user-test-request")
	if len(targetID) > 0 {
		c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(targetID[0])}}
	}
	handler(c)
	return recorder
}

func createChannelManager(t *testing.T, db *gorm.DB, username, group string) model.User {
	t.Helper()
	manager := model.User{
		Username: username, Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
		Group: group, AccountType: model.UserAccountTypeChannel, AuthVersion: 1, AffCode: username + "-aff",
	}
	require.NoError(t, db.Create(&manager).Error)
	return manager
}

func createChannelChild(t *testing.T, db *gorm.DB, manager model.User, username string) model.User {
	t.Helper()
	accessToken := username + "-token"
	child := model.User{
		Username: username, Password: "stored-password", DisplayName: username + " display",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: manager.Group,
		AccountType: model.UserAccountTypeStandard, ChannelOwnerId: manager.Id, AuthVersion: 1,
		Email: username + "@example.com", AccessToken: &accessToken, AffCode: username + "-aff",
	}
	require.NoError(t, db.Create(&child).Error)
	return child
}

func configureRegistrationInvitationTest(t *testing.T) {
	t.Helper()
	previousRegisterEnabled := common.RegisterEnabled
	previousPasswordRegisterEnabled := common.PasswordRegisterEnabled
	previousEmailVerificationEnabled := common.EmailVerificationEnabled
	previousGenerateDefaultToken := constant.GenerateDefaultToken
	common.RegisterEnabled = true
	common.PasswordRegisterEnabled = true
	common.EmailVerificationEnabled = false
	constant.GenerateDefaultToken = false
	t.Cleanup(func() {
		common.RegisterEnabled = previousRegisterEnabled
		common.PasswordRegisterEnabled = previousPasswordRegisterEnabled
		common.EmailVerificationEnabled = previousEmailVerificationEnabled
		constant.GenerateDefaultToken = previousGenerateDefaultToken
	})
}

func performRegistrationRequest(t *testing.T, username, affCode string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := common.Marshal(map[string]string{
		"username": username,
		"password": "password123",
		"aff_code": affCode,
	})
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/user/register", strings.NewReader(string(body)))
	c.Request.Header.Set("Content-Type", "application/json")
	Register(c)
	return recorder
}

type registrationInvitationOAuthProvider struct{}

func (*registrationInvitationOAuthProvider) GetName() string { return "Registration Invitation" }
func (*registrationInvitationOAuthProvider) IsEnabled() bool { return true }
func (*registrationInvitationOAuthProvider) ExchangeToken(context.Context, string, *gin.Context) (*oauth.OAuthToken, error) {
	return &oauth.OAuthToken{}, nil
}
func (*registrationInvitationOAuthProvider) GetUserInfo(context.Context, *oauth.OAuthToken) (*oauth.OAuthUser, error) {
	return nil, errors.New("not used")
}
func (*registrationInvitationOAuthProvider) IsUserIDTaken(providerUserID string) bool {
	return model.IsGitHubIdAlreadyTaken(providerUserID)
}
func (*registrationInvitationOAuthProvider) FillUserByProviderID(user *model.User, providerUserID string) error {
	user.GitHubId = providerUserID
	return user.FillUserByGitHubId()
}
func (*registrationInvitationOAuthProvider) SetProviderUserID(user *model.User, providerUserID string) {
	user.GitHubId = providerUserID
}
func (*registrationInvitationOAuthProvider) GetProviderPrefix() string { return "invite_oauth_" }
func (*registrationInvitationOAuthProvider) ProviderUserIDColumn() string {
	return "github_id"
}

func TestRegistrationInvitationAssignsOnlyActiveTopLevelChannelAccounts(t *testing.T) {
	tests := []struct {
		name        string
		role        int
		accountType string
		status      int
		ownerID     int
		deleted     bool
		wantOwner   bool
		wantInviter bool
	}{
		{name: "active channel", role: common.RoleCommonUser, accountType: model.UserAccountTypeChannel, status: common.UserStatusEnabled, wantOwner: true, wantInviter: true},
		{name: "ordinary user", role: common.RoleCommonUser, accountType: model.UserAccountTypeStandard, status: common.UserStatusEnabled, wantInviter: true},
		{name: "disabled channel", role: common.RoleCommonUser, accountType: model.UserAccountTypeChannel, status: common.UserStatusDisabled, wantInviter: true},
		{name: "nested channel", role: common.RoleCommonUser, accountType: model.UserAccountTypeChannel, status: common.UserStatusEnabled, ownerID: 999, wantInviter: true},
		{name: "admin channel", role: common.RoleAdminUser, accountType: model.UserAccountTypeChannel, status: common.UserStatusEnabled, wantInviter: true},
		{name: "deleted channel", role: common.RoleCommonUser, accountType: model.UserAccountTypeChannel, status: common.UserStatusEnabled, deleted: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := setupManageUserTestDB(t)
			configureRegistrationInvitationTest(t)
			inviter := model.User{
				Username: "registration-inviter", Role: test.role, Status: test.status,
				Group: "inviter-group", AccountType: test.accountType, ChannelOwnerId: test.ownerID,
				AuthVersion: 1, AffCode: "registration-invite-code",
			}
			require.NoError(t, db.Create(&inviter).Error)
			if test.deleted {
				require.NoError(t, db.Delete(&inviter).Error)
			}

			recorder := performRegistrationRequest(t, "registration-invitee", inviter.AffCode)
			assert.Equal(t, http.StatusOK, recorder.Code)
			assert.Contains(t, recorder.Body.String(), `"success":true`)

			var invitee model.User
			require.NoError(t, db.Where("username = ?", "registration-invitee").First(&invitee).Error)
			assert.Equal(t, model.UserAccountTypeStandard, invitee.AccountType)
			if test.wantInviter {
				assert.Equal(t, inviter.Id, invitee.InviterId)
			} else {
				assert.Zero(t, invitee.InviterId)
			}
			if test.wantOwner {
				assert.Equal(t, inviter.Id, invitee.ChannelOwnerId)
				assert.Equal(t, inviter.Group, invitee.Group)
			} else {
				assert.Zero(t, invitee.ChannelOwnerId)
				assert.Equal(t, "default", invitee.Group)
			}
		})
	}
}

func TestOAuthRegistrationInvitationAssignsNewUserWithoutRebindingExistingUser(t *testing.T) {
	db := setupManageUserTestDB(t)
	configureRegistrationInvitationTest(t)
	channel := createChannelManager(t, db, "oauth-registration-channel", "oauth-channel-group")
	provider := &registrationInvitationOAuthProvider{}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/oauth/registration-invitation", nil)

	created, migration, err := findOrCreateOAuthUser(c, provider, &oauth.OAuthUser{
		ProviderUserID: "new-oauth-user", Username: "oauth-channel-invitee",
	}, &oauth.OAuthToken{}, channel.AffCode)
	require.NoError(t, err)
	assert.Nil(t, migration)
	assert.Equal(t, channel.Id, created.InviterId)
	assert.Equal(t, channel.Id, created.ChannelOwnerId)
	assert.Equal(t, channel.Group, created.Group)
	assert.Equal(t, model.UserAccountTypeStandard, created.AccountType)

	existing := model.User{
		Username: "existing-oauth-user", GitHubId: "existing-provider-id",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "existing-group",
		AccountType: model.UserAccountTypeStandard, AuthVersion: 1, AffCode: "existing-oauth-aff",
	}
	require.NoError(t, db.Create(&existing).Error)

	returned, migration, err := findOrCreateOAuthUser(c, provider, &oauth.OAuthUser{
		ProviderUserID: existing.GitHubId, Username: existing.Username,
	}, &oauth.OAuthToken{}, channel.AffCode)
	require.NoError(t, err)
	assert.Nil(t, migration)
	assert.Equal(t, existing.Id, returned.Id)
	var unchanged model.User
	require.NoError(t, db.First(&unchanged, existing.Id).Error)
	assert.Zero(t, unchanged.InviterId)
	assert.Zero(t, unchanged.ChannelOwnerId)
	assert.Equal(t, "existing-group", unchanged.Group)
}

func TestManageUserDisableAdvancesAuthVersionOnceAndRevokesSession(t *testing.T) {
	db := setupManageUserTestDB(t)
	now := time.Now().Unix()
	user := model.User{
		Username: "managed-disable-user", Password: "password", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1,
	}
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, db.Create(&model.UserSession{
		SID: "managed-disable-session", UserID: user.Id, Version: 1, UserAuthVersion: 1,
		Status: model.UserSessionStatusActive, RefreshHash: "refresh-hash", LoginMethod: "password",
		LastActiveAt: now, ExpiresAt: now + 3600,
	}).Error)

	recorder := performManageUserRequest(t, fmt.Sprintf(`{"id":%d,"action":"disable"}`, user.Id))
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"success":true`)

	var updated model.User
	require.NoError(t, db.First(&updated, user.Id).Error)
	assert.Equal(t, common.UserStatusDisabled, updated.Status)
	assert.EqualValues(t, 2, updated.AuthVersion)
	var session model.UserSession
	require.NoError(t, db.First(&session, "sid = ?", "managed-disable-session").Error)
	assert.Equal(t, model.UserSessionStatusRevoked, session.Status)
}

func TestManageUserDemoteAdvancesAuthVersionAndRevokesSessionsOnce(t *testing.T) {
	db := setupManageUserTestDB(t)
	previousMaster := common.IsMasterNode
	common.IsMasterNode = false
	t.Cleanup(func() { common.IsMasterNode = previousMaster })
	require.NoError(t, authz.Init(db))

	now := time.Now().Unix()
	user := model.User{
		Username: "managed-demote-user", Password: "password", Role: common.RoleAdminUser,
		Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1,
	}
	require.NoError(t, db.Create(&user).Error)
	for _, sid := range []string{"managed-demote-session-one", "managed-demote-session-two"} {
		require.NoError(t, db.Create(&model.UserSession{
			SID: sid, UserID: user.Id, Version: 1, UserAuthVersion: 1,
			Status: model.UserSessionStatusActive, RefreshHash: "refresh-" + sid, LoginMethod: "password",
			LastActiveAt: now, ExpiresAt: now + 3600,
		}).Error)
	}

	sessionUpdateCount := 0
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("test:count_demote_session_updates", func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "user_sessions" {
			sessionUpdateCount++
		}
	}))

	recorder := performManageUserRequest(t, fmt.Sprintf(`{"id":%d,"action":"demote"}`, user.Id))
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"success":true`)

	var updated model.User
	require.NoError(t, db.First(&updated, user.Id).Error)
	assert.Equal(t, common.RoleCommonUser, updated.Role)
	assert.EqualValues(t, 2, updated.AuthVersion)
	var sessions []model.UserSession
	require.NoError(t, db.Where("user_id = ?", user.Id).Order("sid asc").Find(&sessions).Error)
	require.Len(t, sessions, 2)
	for _, session := range sessions {
		assert.Equal(t, model.UserSessionStatusRevoked, session.Status)
		assert.Equal(t, "admin_demote", session.RevokedReason)
	}
	assert.Equal(t, 1, sessionUpdateCount)
}

func TestManageUserDeleteReturnsImmediatelyAndUnknownActionFails(t *testing.T) {
	db := setupManageUserTestDB(t)
	deleted := model.User{
		Username: "managed-delete-user", Password: "password", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1, AffCode: "delete-aff",
	}
	require.NoError(t, db.Create(&deleted).Error)

	recorder := performManageUserRequest(t, fmt.Sprintf(`{"id":%d,"action":"delete"}`, deleted.Id))
	assert.Contains(t, recorder.Body.String(), `"success":true`)
	var deletedCount int64
	require.NoError(t, db.Unscoped().Model(&model.User{}).Where("id = ? AND deleted_at IS NOT NULL", deleted.Id).Count(&deletedCount).Error)
	assert.EqualValues(t, 1, deletedCount)

	unchanged := model.User{
		Username: "managed-unknown-user", Password: "password", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1, AffCode: "unknown-aff",
	}
	require.NoError(t, db.Create(&unchanged).Error)
	recorder = performManageUserRequest(t, fmt.Sprintf(`{"id":%d,"action":"unknown"}`, unchanged.Id))
	assert.Contains(t, recorder.Body.String(), `"success":false`)
	require.NoError(t, db.First(&unchanged, unchanged.Id).Error)
	assert.EqualValues(t, 1, unchanged.AuthVersion)
	assert.Equal(t, common.UserStatusEnabled, unchanged.Status)
}

func TestAdminUserReadEndpointsEnforceRoleBoundaryAndRedactSecrets(t *testing.T) {
	db := setupManageUserTestDB(t)
	accessToken := "management-access-token"
	commonUser := model.User{
		Username: "visible-user", Password: "stored-password", DisplayName: "Visible User",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default",
		Email: "visible-user@example.com", GitHubId: "github-visible", DiscordId: "discord-visible",
		OidcId: "oidc-visible", WeChatId: "wechat-visible", TelegramId: "telegram-visible",
		LinuxDOId: "linuxdo-visible", AccessToken: &accessToken, AffCode: "visible-aff-code",
		AffQuota: 123, Setting: `{"webhook_secret":"secret-value","gotify_token":"gotify-secret"}`,
		StripeCustomer: "cus_secret", AuthVersion: 7,
	}
	peerAdmin := model.User{
		Username: "peer-admin", Password: "peer-admin-password", Role: common.RoleAdminUser,
		Status: common.UserStatusEnabled, Group: "default", Email: "peer-admin@example.com",
		AffCode: "peer-admin-aff", Setting: `{"webhook_secret":"peer-secret"}`,
	}
	root := model.User{
		Username: "root-user", Password: "root-password", Role: common.RoleRootUser,
		Status: common.UserStatusEnabled, Group: "default", Email: "root-user@example.com",
		AffCode: "root-user-aff", Setting: `{"webhook_secret":"root-secret"}`,
	}
	require.NoError(t, db.Create(&commonUser).Error)
	require.NoError(t, db.Create(&peerAdmin).Error)
	require.NoError(t, db.Create(&root).Error)

	readPage := func(t *testing.T, recorder *httptest.ResponseRecorder) (bool, int, []map[string]any) {
		t.Helper()
		var response struct {
			Success bool `json:"success"`
			Data    struct {
				Items []map[string]any `json:"items"`
				Total int              `json:"total"`
			} `json:"data"`
		}
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
		return response.Success, response.Data.Total, response.Data.Items
	}

	recorder := performAdminUserReadRequest(t, common.RoleAdminUser, http.MethodGet, "/api/user/?p=1&page_size=20", GetAllUsers)
	success, total, items := readPage(t, recorder)
	require.True(t, success)
	assert.Equal(t, 1, total)
	require.Len(t, items, 1)
	assert.Equal(t, commonUser.Username, items[0]["username"])
	for _, field := range []string{
		"password", "access_token", "access_token_created_at", "email", "github_id", "discord_id",
		"oidc_id", "wechat_id", "telegram_id", "linux_do_id", "setting", "stripe_customer",
		"aff_code", "aff_quota", "auth_version", "verification_code", "original_password",
	} {
		assert.NotContains(t, items[0], field)
	}

	recorder = performAdminUserReadRequest(t, common.RoleAdminUser, http.MethodGet, "/api/user/search?keyword=root-user%40example.com", SearchUsers)
	success, total, items = readPage(t, recorder)
	require.True(t, success)
	assert.Zero(t, total)
	assert.Empty(t, items)

	recorder = performAdminUserReadRequest(t, common.RoleRootUser, http.MethodGet, "/api/user/?p=1&page_size=20", GetAllUsers)
	success, total, items = readPage(t, recorder)
	require.True(t, success)
	assert.Equal(t, 3, total)
	require.Len(t, items, 3)
	roles := make([]int, 0, len(items))
	for _, item := range items {
		roles = append(roles, int(item["role"].(float64)))
	}
	sort.Ints(roles)
	assert.Equal(t, []int{common.RoleCommonUser, common.RoleAdminUser, common.RoleRootUser}, roles)

	recorder = performAdminUserReadRequest(t, common.RoleAdminUser, http.MethodGet, "/api/user/id", GetUser, commonUser.Id)
	var detailResponse struct {
		Success bool           `json:"success"`
		Data    map[string]any `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &detailResponse))
	require.True(t, detailResponse.Success)
	assert.Equal(t, commonUser.Email, detailResponse.Data["email"])
	assert.Equal(t, commonUser.GitHubId, detailResponse.Data["github_id"])
	assert.Equal(t, commonUser.DiscordId, detailResponse.Data["discord_id"])
	assert.Equal(t, commonUser.OidcId, detailResponse.Data["oidc_id"])
	assert.Equal(t, commonUser.WeChatId, detailResponse.Data["wechat_id"])
	assert.Equal(t, commonUser.TelegramId, detailResponse.Data["telegram_id"])
	assert.Equal(t, commonUser.LinuxDOId, detailResponse.Data["linux_do_id"])
	for _, field := range []string{
		"password", "access_token", "access_token_created_at", "setting", "stripe_customer",
		"aff_code", "aff_quota", "auth_version", "verification_code", "original_password",
	} {
		assert.NotContains(t, detailResponse.Data, field)
	}

	recorder = performAdminUserReadRequest(t, common.RoleAdminUser, http.MethodGet, "/api/user/id", GetUser, peerAdmin.Id)
	detailResponse = struct {
		Success bool           `json:"success"`
		Data    map[string]any `json:"data"`
	}{}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &detailResponse))
	assert.False(t, detailResponse.Success)
	assert.Empty(t, detailResponse.Data)
	assert.NotContains(t, recorder.Body.String(), peerAdmin.Email)
	assert.NotContains(t, recorder.Body.String(), "peer-secret")
}

func TestChannelManagerListsOnlyDirectUsersWithoutCredentialFields(t *testing.T) {
	db := setupManageUserTestDB(t)
	manager := createChannelManager(t, db, "channel-manager-a", "channel-a")
	otherManager := createChannelManager(t, db, "channel-manager-b", "channel-b")
	visible := createChannelChild(t, db, manager, "visible-child")
	createChannelChild(t, db, otherManager, "other-child")
	require.NoError(t, db.Create(&model.User{
		Username: "owned-admin", Role: common.RoleAdminUser, Status: common.UserStatusEnabled,
		Group: manager.Group, AccountType: model.UserAccountTypeStandard, ChannelOwnerId: manager.Id,
		AffCode: "owned-admin-aff",
	}).Error)

	recorder := performChannelUserRequest(t, manager, http.MethodGet, "/api/user/channel/members/?p=1&page_size=20", "", GetChannelManagedUsers)
	assert.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Items []map[string]any `json:"items"`
			Total int              `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	assert.Equal(t, 1, response.Data.Total)
	require.Len(t, response.Data.Items, 1)
	item := response.Data.Items[0]
	assert.Equal(t, visible.Username, item["username"])
	for _, field := range []string{"password", "access_token", "email", "github_id", "oidc_id", "wechat_id", "telegram_id", "setting", "auth_version"} {
		assert.NotContains(t, item, field)
	}

	recorder = performChannelUserRequest(t, manager, http.MethodGet, "/api/user/channel/members/search?keyword=visible", "", GetChannelManagedUsers)
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Len(t, response.Data.Items, 1)
	assert.Equal(t, visible.Username, response.Data.Items[0]["username"])
}

func TestChannelManagerCreateAndUpdateIgnorePrivilegedFields(t *testing.T) {
	db := setupManageUserTestDB(t)
	manager := createChannelManager(t, db, "channel-create-manager", "inherited-group")
	otherManager := createChannelManager(t, db, "other-channel-manager", "other-group")
	body := fmt.Sprintf(`{
		"username":"channel-created-user",
		"password":"12344321",
		"display_name":"Created User",
		"remark":"managed note",
		"role":100,
		"group":"forged-group",
		"status":2,
		"channel_owner_id":%d,
		"account_type":"channel",
		"quota":999999
	}`, otherManager.Id)

	recorder := performChannelUserRequest(t, manager, http.MethodPost, "/api/user/channel/members/", body, CreateChannelManagedUser)
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"success":true`)

	var created model.User
	require.NoError(t, db.Where("username = ?", "channel-created-user").First(&created).Error)
	assert.Equal(t, common.RoleCommonUser, created.Role)
	assert.Equal(t, model.UserAccountTypeStandard, created.AccountType)
	assert.Equal(t, manager.Id, created.ChannelOwnerId)
	assert.Equal(t, manager.Group, created.Group)
	assert.Equal(t, common.UserStatusEnabled, created.Status)
	assert.Equal(t, common.QuotaForNewUser, created.Quota)
	assert.NotEqual(t, "12344321", created.Password)

	createdPassword := created.Password
	updateBody := fmt.Sprintf(`{
		"display_name":"Updated User",
		"remark":"updated note",
		"password":"forged-password",
		"role":100,
		"group":"forged-group",
		"status":2,
		"channel_owner_id":%d,
		"account_type":"channel",
		"quota":999999
	}`, otherManager.Id)
	recorder = performChannelUserRequest(t, manager, http.MethodPut, "/api/user/channel/members/id", updateBody, UpdateChannelManagedUser, created.Id)
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"success":true`)
	require.NoError(t, db.First(&created, created.Id).Error)
	assert.Equal(t, "Updated User", created.DisplayName)
	assert.Equal(t, "updated note", created.Remark)
	assert.Equal(t, createdPassword, created.Password)
	assert.Equal(t, common.RoleCommonUser, created.Role)
	assert.Equal(t, model.UserAccountTypeStandard, created.AccountType)
	assert.Equal(t, manager.Id, created.ChannelOwnerId)
	assert.Equal(t, manager.Group, created.Group)
	assert.Equal(t, common.UserStatusEnabled, created.Status)
	assert.Equal(t, common.QuotaForNewUser, created.Quota)
}

func TestChannelManagerCannotAccessAnotherChannelsUser(t *testing.T) {
	db := setupManageUserTestDB(t)
	manager := createChannelManager(t, db, "channel-owner-a", "channel-a")
	otherManager := createChannelManager(t, db, "channel-owner-b", "channel-b")
	target := createChannelChild(t, db, otherManager, "other-managed-user")
	originalDisplayName := target.DisplayName

	requests := []struct {
		name, method, path, body string
		handler                  gin.HandlerFunc
		withTarget               bool
	}{
		{name: "get", method: http.MethodGet, path: "/api/user/channel/members/id", handler: GetChannelManagedUser, withTarget: true},
		{name: "update", method: http.MethodPut, path: "/api/user/channel/members/id", body: `{"display_name":"forged","remark":"forged"}`, handler: UpdateChannelManagedUser, withTarget: true},
		{name: "status", method: http.MethodPost, path: "/api/user/channel/members/manage", body: fmt.Sprintf(`{"id":%d,"action":"disable"}`, target.Id), handler: ManageChannelManagedUser},
		{name: "delete", method: http.MethodDelete, path: "/api/user/channel/members/id", handler: DeleteChannelManagedUser, withTarget: true},
	}
	for _, tc := range requests {
		t.Run(tc.name, func(t *testing.T) {
			var recorder *httptest.ResponseRecorder
			if tc.withTarget {
				recorder = performChannelUserRequest(t, manager, tc.method, tc.path, tc.body, tc.handler, target.Id)
			} else {
				recorder = performChannelUserRequest(t, manager, tc.method, tc.path, tc.body, tc.handler)
			}
			assert.Equal(t, http.StatusOK, recorder.Code)
			assert.Contains(t, recorder.Body.String(), `"success":false`)
		})
	}

	var unchanged model.User
	require.NoError(t, db.First(&unchanged, target.Id).Error)
	assert.Equal(t, originalDisplayName, unchanged.DisplayName)
	assert.Empty(t, unchanged.Remark)
	assert.Equal(t, common.UserStatusEnabled, unchanged.Status)
	assert.EqualValues(t, 1, unchanged.AuthVersion)
	assert.False(t, unchanged.DeletedAt.Valid)
}

func TestChannelManagerDisableAdvancesAuthVersionAndRevokesSession(t *testing.T) {
	db := setupManageUserTestDB(t)
	manager := createChannelManager(t, db, "channel-disable-manager", "channel-a")
	target := createChannelChild(t, db, manager, "channel-disable-user")
	now := time.Now().Unix()
	require.NoError(t, db.Create(&model.UserSession{
		SID: "channel-managed-session", UserID: target.Id, Version: 1, UserAuthVersion: 1,
		Status: model.UserSessionStatusActive, RefreshHash: "refresh-hash", LoginMethod: "password",
		LastActiveAt: now, ExpiresAt: now + 3600,
	}).Error)

	recorder := performChannelUserRequest(t, manager, http.MethodPost, "/api/user/channel/members/manage", fmt.Sprintf(`{"id":%d,"action":"disable"}`, target.Id), ManageChannelManagedUser)
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"success":true`)

	var updated model.User
	require.NoError(t, db.First(&updated, target.Id).Error)
	assert.Equal(t, common.UserStatusDisabled, updated.Status)
	assert.EqualValues(t, 2, updated.AuthVersion)
	var session model.UserSession
	require.NoError(t, db.First(&session, "sid = ?", "channel-managed-session").Error)
	assert.Equal(t, model.UserSessionStatusRevoked, session.Status)
	assert.Equal(t, "channel_manager_status_change", session.RevokedReason)
}

func TestInvalidChannelManagersCannotManageChannelUsers(t *testing.T) {
	for _, tc := range []struct {
		name, accountType string
		status, ownerID   int
	}{
		{name: "standard user", accountType: model.UserAccountTypeStandard, status: common.UserStatusEnabled},
		{name: "disabled channel", accountType: model.UserAccountTypeChannel, status: common.UserStatusDisabled},
		{name: "nested channel", accountType: model.UserAccountTypeChannel, status: common.UserStatusEnabled, ownerID: 999},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := setupManageUserTestDB(t)
			manager := model.User{
				Username: "invalid-channel-manager", Role: common.RoleCommonUser, Status: tc.status,
				Group: "default", AccountType: tc.accountType, ChannelOwnerId: tc.ownerID,
				AuthVersion: 1, AffCode: "invalid-channel-manager-aff",
			}
			require.NoError(t, db.Create(&manager).Error)

			recorder := performChannelUserRequest(t, manager, http.MethodGet, "/api/user/channel/members/", "", GetChannelManagedUsers)
			assert.Equal(t, http.StatusForbidden, recorder.Code)
			assert.Contains(t, recorder.Body.String(), `"success":false`)
		})
	}
}

func createQuotaTestOperator(t *testing.T, db *gorm.DB, role int) model.User {
	t.Helper()
	if role == 0 {
		role = common.RoleRootUser
	}
	operator := model.User{Id: 9999, Username: "root-operator", Role: role, Status: common.UserStatusEnabled, AuthVersion: 1, AffCode: "root-operator-aff"}
	require.NoError(t, db.Create(&operator).Error)
	return operator
}

func TestManageUserQuotaRecordsTopupAndAudit(t *testing.T) {
	for _, tc := range []struct {
		name, mode, action, content string
		value, wantQuota            int
	}{
		{"add", "add", "user.quota_add", "Increased user quota by 500", 500, 1500},
		{"subtract", "subtract", "user.quota_subtract", "Decreased user quota by 500", 500, 500},
		{"override_up", "override", "user.quota_override", "Overrode user quota from 1000 to 1500", 1500, 1500},
		{"override_down", "override", "user.quota_override", "Overrode user quota from 1000 to 500", 500, 500},
		{"override_unchanged", "override", "user.quota_override", "Overrode user quota from 1000 to 1000", 1000, 1000},
		{"override_zero", "override", "user.quota_override", "Overrode user quota from 1000 to 0", 0, 0},
		{"override_negative", "override", "user.quota_override", "Overrode user quota from 1000 to -1", -1, -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := setupManageUserTestDB(t)
			user := model.User{Username: "quota-owner", Role: common.RoleCommonUser, Quota: 1000, AffCode: "quota-owner-aff"}
			require.NoError(t, db.Create(&user).Error)
			createQuotaTestOperator(t, db, common.RoleRootUser)

			recorder := performManageUserRequest(t, fmt.Sprintf(`{"id":%d,"action":"add_quota","mode":%q,"value":%d}`, user.Id, tc.mode, tc.value))
			assert.Equal(t, http.StatusOK, recorder.Code)
			require.Contains(t, recorder.Body.String(), `"success":true`)
			require.NoError(t, db.First(&user, user.Id).Error)
			assert.Equal(t, tc.wantQuota, user.Quota)

			logs, total, err := model.GetAllLogs(model.LogTypeTopup, 0, 0, "", "", "", 0, 20, 0, "", "", "")
			require.NoError(t, err)
			assert.EqualValues(t, 1, total)
			require.Len(t, logs, 1)
			assert.Equal(t, user.Id, logs[0].UserId)
			assert.Equal(t, user.Username, logs[0].Username)
			assert.Equal(t, tc.content, logs[0].Content)
			model.FormatAdminLogs(logs)
			var other model.AuditOther
			require.NoError(t, common.UnmarshalJsonStr(logs[0].Other, &other))
			assert.Equal(t, &model.AuditAdminInfo{AdminID: 9999, AdminUsername: "root-operator", AdminRole: common.RoleRootUser, AuthMethod: "session"}, other.AdminInfo)

			logs, total, err = model.GetUserLogs(user.Id, model.LogTypeTopup, 0, 0, "", "", 0, 20, "", "", "")
			require.NoError(t, err)
			assert.EqualValues(t, 1, total)
			require.Len(t, logs, 1)
			other = model.AuditOther{}
			require.NoError(t, common.UnmarshalJsonStr(logs[0].Other, &other))
			assert.Nil(t, other.AdminInfo)
			require.NotNil(t, other.Op)
			assert.Equal(t, tc.action, other.Op.Action)
			params, err := common.Marshal(other.Op.Params)
			require.NoError(t, err)
			expectedParams := model.AuditFields{"target_user_id": user.Id, "target_username": user.Username, "mode": tc.mode, "requested_quota": tc.value, "from": 1000, "to": tc.wantQuota}
			if tc.mode != "override" {
				expectedParams["quota"] = tc.value
			}
			expected, err := common.Marshal(expectedParams)
			require.NoError(t, err)
			assert.JSONEq(t, string(expected), string(params))
			assert.Equal(t, "quota-test-request", logs[0].RequestId)
			assert.Empty(t, logs[0].Ip, "recipient logs must not disclose the administrator IP")

			logs, total, err = model.GetUserLogs(9999, model.LogTypeTopup, 0, 0, "", "", 0, 20, "", "", "")
			require.NoError(t, err)
			assert.Zero(t, total)
			assert.Empty(t, logs)
			var audits []model.AuditLog
			require.NoError(t, model.LOG_DB.Find(&audits).Error)
			require.Len(t, audits, 1)
			assert.Equal(t, 9999, audits[0].UserId)
			assert.Equal(t, "root-operator", audits[0].Username)
			assert.Equal(t, tc.action, audits[0].Action)
			assert.True(t, audits[0].Success)
			params, err = common.Marshal(audits[0].Other.Op.Params)
			require.NoError(t, err)
			assert.JSONEq(t, string(expected), string(params))
			assert.Equal(t, "quota-test-request", audits[0].RequestId)
		})
	}
}

func TestManageUserQuotaFailuresDoNotRecordTopup(t *testing.T) {
	for _, tc := range []struct {
		name, mode string
		value      int
		failUpdate bool
	}{
		{"zero_add", "add", 0, false},
		{"negative_subtract", "subtract", -1, false},
		{"invalid_mode", "invalid", 500, false},
		{"add_update_error", "add", 500, true},
		{"subtract_update_error", "subtract", 500, true},
		{"override_update_error", "override", 500, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := setupManageUserTestDB(t)
			createQuotaTestOperator(t, db, common.RoleRootUser)
			user := model.User{Username: "quota-owner", Role: common.RoleCommonUser, Quota: 1000}
			require.NoError(t, db.Create(&user).Error)
			if tc.failUpdate {
				require.NoError(t, db.Callback().Update().Before("gorm:update").Register("test:fail_quota_update", func(tx *gorm.DB) {
					if tx.Statement.Table == "users" {
						tx.AddError(errors.New("quota update unavailable"))
					}
				}))
			}
			recorder := performManageUserRequest(t, fmt.Sprintf(`{"id":%d,"action":"add_quota","mode":%q,"value":%d}`, user.Id, tc.mode, tc.value))
			assert.Contains(t, recorder.Body.String(), `"success":false`)
			require.NoError(t, db.First(&user, user.Id).Error)
			assert.Equal(t, 1000, user.Quota)
			var logCount, auditCount int64
			require.NoError(t, model.LOG_DB.Model(&model.Log{}).Count(&logCount).Error)
			require.NoError(t, model.LOG_DB.Model(&model.AuditLog{}).Count(&auditCount).Error)
			assert.Zero(t, logCount)
			assert.EqualValues(t, 1, auditCount)
			var audit model.AuditLog
			require.NoError(t, model.LOG_DB.First(&audit).Error)
			assert.False(t, audit.Success)
			params, err := common.Marshal(audit.Other.Op.Params)
			require.NoError(t, err)
			assert.NotContains(t, string(params), `"from"`)
			assert.NotContains(t, string(params), `"to"`)
			assert.NotContains(t, string(params), `"target_username"`)
			assert.NotContains(t, string(params), "quota update unavailable")
			reason := "invalid_parameters"
			if tc.failUpdate {
				reason = "database_error"
			}
			assert.Contains(t, string(params), `"failure_reason":"`+reason+`"`)
			assert.Contains(t, string(params), fmt.Sprintf(`"requested_quota":%d`, tc.value))
		})
	}
}

func TestManageUserQuotaLogFailureKeepsSuccessfulAdjustment(t *testing.T) {
	for _, failedTable := range []string{"logs", "audit_logs"} {
		t.Run(failedTable, func(t *testing.T) {
			db := setupManageUserTestDB(t)
			createQuotaTestOperator(t, db, common.RoleRootUser)
			user := model.User{Username: "quota-owner", Role: common.RoleCommonUser, Quota: 1000}
			require.NoError(t, db.Create(&user).Error)
			require.NoError(t, model.LOG_DB.Callback().Create().Before("gorm:create").Register("test:fail_quota_log", func(tx *gorm.DB) {
				if tx.Statement.Table == failedTable {
					tx.AddError(errors.New("quota log unavailable"))
				}
			}))
			recorder := performManageUserRequest(t, fmt.Sprintf(`{"id":%d,"action":"add_quota","mode":"add","value":500}`, user.Id))
			assert.Contains(t, recorder.Body.String(), `"success":true`)
			require.NoError(t, db.First(&user, user.Id).Error)
			assert.Equal(t, 1500, user.Quota)
			var logCount, auditCount int64
			require.NoError(t, model.LOG_DB.Model(&model.Log{}).Count(&logCount).Error)
			require.NoError(t, model.LOG_DB.Model(&model.AuditLog{}).Count(&auditCount).Error)
			if failedTable == "logs" {
				assert.Zero(t, logCount)
				assert.EqualValues(t, 1, auditCount)
			} else {
				assert.EqualValues(t, 1, logCount)
				assert.Zero(t, auditCount)
			}
		})
	}
}

func TestManageUserQuotaTargetsAndWalletBounds(t *testing.T) {
	for _, tc := range []struct {
		name, mode, reason                                string
		before, value, targetID, targetRole, operatorRole int
		deleted, failRead                                 bool
	}{
		{name: "zero_id", mode: "add", value: 1, reason: "invalid_parameters"},
		{name: "negative_id", mode: "subtract", value: 1, targetID: -1, reason: "invalid_parameters"},
		{name: "missing", mode: "override", value: 1, targetID: 12345, reason: "target_not_found"},
		{name: "deleted", mode: "add", value: 1, targetID: 1, deleted: true, reason: "target_not_found"},
		{name: "peer_admin", mode: "add", value: 1, targetID: 1, targetRole: common.RoleAdminUser, operatorRole: common.RoleAdminUser, reason: "permission_denied"},
		{name: "higher_role", mode: "override", value: 1, targetID: 1, targetRole: common.RoleRootUser, operatorRole: common.RoleAdminUser, reason: "permission_denied"},
		{name: "read_error", mode: "add", value: 1, targetID: 1, failRead: true, reason: "database_error"},
		{name: "add_overflow", mode: "add", before: common.MaxWalletQuota, value: 1, targetID: 1, reason: "quota_limit_exceeded"},
		{name: "subtract_underflow", mode: "subtract", before: -common.MaxWalletQuota, value: 1, targetID: 1, reason: "quota_limit_exceeded"},
		{name: "oversized_add", mode: "add", value: common.MaxWalletQuota + 1, targetID: 1, reason: "quota_limit_exceeded"},
		{name: "oversized_subtract", mode: "subtract", value: common.MaxWalletQuota + 1, targetID: 1, reason: "quota_limit_exceeded"},
		{name: "oversized_override", mode: "override", value: -common.MaxWalletQuota - 1, targetID: 1, reason: "quota_limit_exceeded"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := setupManageUserTestDB(t)
			createQuotaTestOperator(t, db, tc.operatorRole)
			user := model.User{Id: 1, Username: "private-target-name", Role: tc.targetRole, Quota: tc.before}
			require.NoError(t, db.Create(&user).Error)
			if tc.deleted {
				require.NoError(t, db.Delete(&user).Error)
			}
			if tc.failRead {
				require.NoError(t, db.Callback().Query().Before("gorm:query").Register("test:quota_read_error", func(tx *gorm.DB) {
					if tx.Statement.Table == "users" && len(tx.Statement.Selects) == 0 {
						tx.AddError(errors.New("private database error"))
					}
				}))
			}
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/api/user/manage", strings.NewReader(fmt.Sprintf(`{"id":%d,"action":"add_quota","mode":%q,"value":%d}`, tc.targetID, tc.mode, tc.value)))
			role := tc.operatorRole
			if role == 0 {
				role = common.RoleRootUser
			}
			c.Set("id", 9999)
			c.Set("role", role)
			ManageUser(c)
			require.Contains(t, recorder.Body.String(), `"success":false`)
			if tc.failRead {
				require.NoError(t, db.Callback().Query().Remove("test:quota_read_error"))
			}
			require.NoError(t, db.Unscoped().First(&user, user.Id).Error)
			assert.Equal(t, tc.before, user.Quota)
			var audits []model.AuditLog
			require.NoError(t, model.LOG_DB.Find(&audits).Error)
			require.Len(t, audits, 1)
			assert.False(t, audits[0].Success)
			params, err := common.Marshal(audits[0].Other.Op.Params)
			require.NoError(t, err)
			expected, err := common.Marshal(model.AuditFields{"target_user_id": tc.targetID, "mode": tc.mode, "requested_quota": tc.value, "failure_reason": tc.reason})
			require.NoError(t, err)
			assert.JSONEq(t, string(expected), string(params))
			assert.NotContains(t, audits[0].Content, user.Username)
			var count int64
			require.NoError(t, model.LOG_DB.Model(&model.Log{}).Count(&count).Error)
			assert.Zero(t, count)
		})
	}
}

func TestManageUserQuotaMiddlewareKeepsOneOperationPerRequest(t *testing.T) {
	db := setupManageUserTestDB(t)
	pat := "quota-middleware-test-token"
	operator := model.User{Id: 9999, Username: "root-operator", Role: common.RoleRootUser, Status: common.UserStatusEnabled, AuthVersion: 1, AccessToken: &pat, Quota: 1000}
	require.NoError(t, db.Create(&operator).Error)
	router := gin.New()
	router.Use(middleware.RequestId(), middleware.AccessTokenAudit())
	router.POST("/api/user/manage", middleware.AdminAuth(), ManageUser)
	for _, tc := range []struct {
		body, action string
		success      bool
	}{
		{`{"id":9999,"action":"add_quota","mode":"add","value":100}`, "user.quota_add", true},
		{`{"id":9999,"action":"add_quota","mode":"subtract","value":0}`, "user.quota_subtract", false},
		{`{"id":9999,"action":"add_quota","mode":"invalid","value":1}`, "generic", false},
		{`{"id":`, "generic", false},
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/user/manage", strings.NewReader(tc.body))
		request.Header.Set("Authorization", "Bearer "+pat)
		request.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(recorder, request)
		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), fmt.Sprintf(`"success":%t`, tc.success))
		requestID := recorder.Header().Get(common.RequestIdKey)
		require.NotEmpty(t, requestID)
		var audits []model.AuditLog
		require.NoError(t, model.LOG_DB.Where("request_id = ?", requestID).Find(&audits).Error)
		require.Len(t, audits, 2, "one operation audit and one PAT request audit")
		for _, audit := range audits {
			assert.Equal(t, tc.success, audit.Success)
			if audit.Category != model.AuditCategoryOperation {
				continue
			}
			assert.Equal(t, tc.action, audit.Action)
			assert.Equal(t, operator.Id, audit.UserId)
			assert.Equal(t, "/api/user/manage", audit.Route)
			if tc.success {
				params, err := common.Marshal(audit.Other.Op.Params)
				require.NoError(t, err)
				assert.Contains(t, string(params), `"target_user_id":9999`)
				assert.Contains(t, string(params), `"target_username":"root-operator"`)
			}
		}
		var count int64
		require.NoError(t, model.LOG_DB.Model(&model.Log{}).Where("request_id = ?", requestID).Count(&count).Error)
		if tc.success {
			assert.EqualValues(t, 1, count)
		} else {
			assert.Zero(t, count)
		}
	}
	require.NoError(t, db.First(&operator, operator.Id).Error)
	assert.Equal(t, 1100, operator.Quota)
}

func TestManageUserQuotaConcurrentSnapshots(t *testing.T) {
	db := setupManageUserTestDB(t)
	user := model.User{Username: "concurrent-quota", Quota: 1000}
	require.NoError(t, db.Create(&user).Error)
	var ready sync.WaitGroup
	ready.Add(2)
	release := make(chan struct{})
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("test:concurrent_quota_start", func(tx *gorm.DB) {
		if tx.Statement.Table == "users" {
			ready.Done()
			<-release
		}
	}))
	type result struct {
		adjustment *model.UserQuotaAdjustment
		err        error
		value      int
	}
	results := make(chan result, 2)
	for _, value := range []int{10, 20} {
		go func(value int) {
			adjustment, err := model.AdjustUserQuota(user.Id, common.RoleRootUser, "add", value)
			results <- result{adjustment, err, value}
		}(value)
	}
	ready.Wait()
	close(release)
	var committed []model.UserQuotaAdjustment
	for range 2 {
		result := <-results
		if result.err != nil {
			require.True(t, common.UsingMainDatabase(common.DatabaseTypeSQLite), "row-locking databases must serialize both adjustments: %v", result.err)
			assert.Contains(t, strings.ToLower(result.err.Error()), "locked")
			assert.Nil(t, result.adjustment)
			continue
		}
		require.NotNil(t, result.adjustment)
		assert.Equal(t, result.value, result.adjustment.After-result.adjustment.Before)
		committed = append(committed, *result.adjustment)
	}
	require.NoError(t, db.Callback().Query().Remove("test:concurrent_quota_start"))
	require.NotEmpty(t, committed)
	sort.Slice(committed, func(i, j int) bool { return committed[i].Before < committed[j].Before })
	balance := 1000
	for _, adjustment := range committed {
		assert.Equal(t, balance, adjustment.Before)
		balance = adjustment.After
	}
	require.NoError(t, db.First(&user, user.Id).Error)
	assert.Equal(t, balance, user.Quota)
}

func TestManageUserQuotaCacheUsesCommittedIntegerDifference(t *testing.T) {
	for _, tc := range []struct {
		name, mode                               string
		before, cached, value, after, wantCached int
		failUpdate, failCache, missingCache      bool
	}{
		{name: "add_preserves_reservations", mode: "add", before: 1000, cached: 900, value: 500, after: 1500, wantCached: 1400},
		{name: "subtract_preserves_reservations", mode: "subtract", before: 1000, cached: 900, value: 500, after: 500, wantCached: 400},
		{name: "override_preserves_reservations", mode: "override", before: 1000, cached: 900, value: 2000, after: 2000, wantCached: 1900},
		{name: "large_odd_difference", mode: "override", before: common.MaxWalletQuota - 1, cached: common.MaxWalletQuota - 1, value: -common.MaxWalletQuota, after: -common.MaxWalletQuota, wantCached: -common.MaxWalletQuota},
		{name: "rollback_does_not_change_cache", mode: "subtract", before: 1000, cached: 900, value: 500, after: 1000, wantCached: 900, failUpdate: true},
		{name: "cache_error_keeps_committed_change", mode: "add", before: 1000, value: 500, after: 1500, failCache: true},
		{name: "missing_cache_is_not_partially_created", mode: "add", before: 1000, value: 500, after: 1500, missingCache: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := setupManageUserTestDB(t)
			operator := createQuotaTestOperator(t, db, common.RoleRootUser)
			server := miniredis.RunT(t)
			oldRDB := common.RDB
			common.RDB = redis.NewClient(&redis.Options{Addr: server.Addr(), MaxRetries: -1})
			common.RedisEnabled = true
			t.Cleanup(func() { _ = common.RDB.Close(); common.RDB = oldRDB })
			user := model.User{Username: "cached-quota", Quota: tc.before, AuthVersion: 1}
			require.NoError(t, db.Create(&user).Error)
			cache, err := model.GetUserCache(user.Id)
			require.NoError(t, err)
			assert.Equal(t, tc.before, cache.Quota)
			_, err = model.GetUserCache(operator.Id)
			require.NoError(t, err)
			keys := server.Keys()
			var quotaKey string
			for _, key := range keys {
				if server.HGet(key, "Id") == strconv.Itoa(user.Id) && server.HGet(key, "Quota") != "" {
					quotaKey = key
					break
				}
			}
			require.NotEmpty(t, quotaKey)
			server.HSet(quotaKey, "Quota", strconv.Itoa(tc.cached))
			if tc.missingCache {
				server.Del(quotaKey)
			}
			if tc.failCache {
				server.SetError("ERR quota cache unavailable")
			}
			if tc.failUpdate {
				require.NoError(t, db.Callback().Update().After("gorm:update").Register("test:cache_quota_rollback", func(tx *gorm.DB) {
					if tx.Statement.Table == "users" {
						tx.AddError(errors.New("quota rollback"))
					}
				}))
			}
			recorder := performManageUserRequest(t, fmt.Sprintf(`{"id":%d,"action":"add_quota","mode":%q,"value":%d}`, user.Id, tc.mode, tc.value))
			assert.Contains(t, recorder.Body.String(), fmt.Sprintf(`"success":%t`, !tc.failUpdate))
			require.NoError(t, db.First(&user, user.Id).Error)
			assert.Equal(t, tc.after, user.Quota)
			if tc.missingCache {
				// Log username lookup may hydrate the whole user after commit.
				if server.Exists(quotaKey) {
					assert.Equal(t, strconv.Itoa(user.Id), server.HGet(quotaKey, "Id"))
					assert.NotEmpty(t, server.HGet(quotaKey, "CacheSchema"))
					assert.Equal(t, strconv.Itoa(tc.after), server.HGet(quotaKey, "Quota"))
				}
			} else if !tc.failCache {
				assert.Equal(t, strconv.Itoa(tc.wantCached), server.HGet(quotaKey, "Quota"))
			}
		})
	}
}
