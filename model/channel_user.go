package model

import (
	"errors"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

var (
	ErrChannelAccountRequired = errors.New("active channel account required")
	ErrChannelUserNotFound    = errors.New("channel user not found")
)

var channelManagedUserColumns = []string{
	"id", "username", "display_name", "role", "account_type", "channel_owner_id",
	"status", "quota", "used_quota", "request_count", "group", "remark",
	"created_at", "last_login_at", "deleted_at", "auth_version",
}

func loadChannelAccountManager(tx *gorm.DB, managerID int, lock bool) (*User, error) {
	if managerID <= 0 {
		return nil, ErrChannelAccountRequired
	}
	query := tx
	if lock {
		query = lockForUpdate(query)
	}
	var manager User
	err := query.Select([]string{"id", "role", "account_type", "channel_owner_id", "status", "group"}).
		Where("id = ? AND role = ? AND account_type = ? AND channel_owner_id = ? AND status = ?",
			managerID, common.RoleCommonUser, UserAccountTypeChannel, 0, common.UserStatusEnabled).
		First(&manager).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrChannelAccountRequired
	}
	if err != nil {
		return nil, err
	}
	return &manager, nil
}

func channelManagedUsersQuery(tx *gorm.DB, managerID int) *gorm.DB {
	return tx.Unscoped().Model(&User{}).
		Where("channel_owner_id = ? AND role = ? AND account_type = ?",
			managerID, common.RoleCommonUser, UserAccountTypeStandard)
}

func ListChannelManagedUsers(managerID int, keyword string, status *int, pageInfo *common.PageInfo, sortOptions ...UserSortOptions) ([]*User, int64, error) {
	if _, err := loadChannelAccountManager(DB, managerID, false); err != nil {
		return nil, 0, err
	}

	query := channelManagedUsersQuery(DB, managerID)
	if keyword != "" {
		condition := "username LIKE ? OR display_name LIKE ?"
		args := []any{"%" + keyword + "%", "%" + keyword + "%"}
		if id, err := strconv.Atoi(keyword); err == nil {
			condition = "id = ? OR " + condition
			args = append([]any{id}, args...)
		}
		query = query.Where("("+condition+")", args...)
	}
	if status != nil {
		if *status == -1 {
			query = query.Where("deleted_at IS NOT NULL")
		} else {
			query = query.Where("deleted_at IS NULL AND status = ?", *status)
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var users []*User
	order := resolveUserSortOptions(sortOptions)
	if err := order.Apply(query.Select(channelManagedUserColumns)).
		Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&users).Error; err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

func GetChannelManagedUser(managerID, userID int) (*User, error) {
	if _, err := loadChannelAccountManager(DB, managerID, false); err != nil {
		return nil, err
	}
	query := channelManagedUsersQuery(DB, managerID).Select(channelManagedUserColumns).Where("deleted_at IS NULL")
	var user User
	if err := query.Where("id = ?", userID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrChannelUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

func CreateChannelManagedUser(managerID int, user *User) error {
	if user == nil {
		return ErrChannelUserNotFound
	}
	if err := DB.Transaction(func(tx *gorm.DB) error {
		manager, err := loadChannelAccountManager(tx, managerID, true)
		if err != nil {
			return err
		}
		user.Role = common.RoleCommonUser
		user.AccountType = UserAccountTypeStandard
		user.ChannelOwnerId = manager.Id
		user.Group = manager.Group
		user.Status = common.UserStatusEnabled
		return user.InsertWithTx(tx, 0)
	}); err != nil {
		return err
	}
	user.FinishInsert(0)
	return nil
}

func UpdateChannelManagedUser(managerID, userID int, displayName, remark string) (*User, error) {
	var user User
	err := DB.Transaction(func(tx *gorm.DB) error {
		if _, err := loadChannelAccountManager(tx, managerID, true); err != nil {
			return err
		}
		query := lockForUpdate(channelManagedUsersQuery(tx, managerID)).Where("id = ? AND deleted_at IS NULL", userID)
		if err := query.First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrChannelUserNotFound
			}
			return err
		}
		if displayName == "" {
			displayName = user.Username
		}
		if err := tx.Model(&user).Updates(map[string]any{
			"display_name": displayName,
			"remark":       remark,
		}).Error; err != nil {
			return err
		}
		return tx.Select(channelManagedUserColumns).First(&user, userID).Error
	})
	return &user, err
}

func SetChannelManagedUserStatus(managerID, userID, status int) (*User, error) {
	if status != common.UserStatusEnabled && status != common.UserStatusDisabled {
		return nil, errors.New("invalid user status")
	}
	var user User
	changed := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		if _, err := loadChannelAccountManager(tx, managerID, true); err != nil {
			return err
		}
		query := lockForUpdate(channelManagedUsersQuery(tx, managerID)).Where("id = ? AND deleted_at IS NULL", userID)
		if err := query.First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrChannelUserNotFound
			}
			return err
		}
		if user.Status == status {
			return nil
		}
		next, err := IncrementUserAuthVersionWithTx(tx, user.Id)
		if err != nil {
			return err
		}
		if err := tx.Model(&user).Update("status", status).Error; err != nil {
			return err
		}
		user.Status = status
		user.AuthVersion = next
		changed = true
		return nil
	})
	if err != nil {
		return nil, err
	}
	if changed {
		if err := PublishUserAuthCache(user.Id); err != nil {
			return nil, err
		}
		if _, err := RevokeAllUserSessions(user.Id, "channel_manager_status_change"); err != nil {
			return nil, err
		}
		if err := InvalidateUserTokensCache(user.Id); err != nil {
			return nil, err
		}
	}
	return &user, nil
}

func DeleteChannelManagedUser(managerID, userID int) error {
	var user User
	var nextAuthVersion int64
	if err := DB.Transaction(func(tx *gorm.DB) error {
		if _, err := loadChannelAccountManager(tx, managerID, true); err != nil {
			return err
		}
		query := lockForUpdate(channelManagedUsersQuery(tx, managerID)).Where("id = ? AND deleted_at IS NULL", userID)
		if err := query.First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrChannelUserNotFound
			}
			return err
		}
		var err error
		nextAuthVersion, err = IncrementUserAuthVersionWithTx(tx, user.Id)
		if err != nil {
			return err
		}
		return tx.Delete(&user).Error
	}); err != nil {
		return err
	}
	if err := publishCommittedUserAuthVersion(user.Id, nextAuthVersion); err != nil {
		return err
	}
	if _, err := RevokeAllUserSessions(user.Id, "channel_manager_delete"); err != nil {
		return err
	}
	if err := InvalidateUserTokensCache(user.Id); err != nil {
		return err
	}
	return invalidateUserCache(user.Id)
}
