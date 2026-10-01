package controller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type channelManagedUserView struct {
	Id             int        `json:"id"`
	Username       string     `json:"username"`
	DisplayName    string     `json:"display_name"`
	Role           int        `json:"role"`
	AccountType    string     `json:"account_type"`
	ChannelOwnerId int        `json:"channel_owner_id"`
	Status         int        `json:"status"`
	Quota          int        `json:"quota"`
	UsedQuota      int        `json:"used_quota"`
	RequestCount   int        `json:"request_count"`
	Group          string     `json:"group"`
	Remark         string     `json:"remark,omitempty"`
	CreatedAt      int64      `json:"created_at"`
	LastLoginAt    int64      `json:"last_login_at"`
	DeletedAt      *time.Time `json:"DeletedAt"`
}

type channelUserCreateRequest struct {
	Username    string `json:"username" validate:"required,max=20"`
	Password    string `json:"password" validate:"required,min=8,max=128"`
	DisplayName string `json:"display_name" validate:"max=20"`
	Remark      string `json:"remark" validate:"max=255"`
}

type channelUserUpdateRequest struct {
	DisplayName string `json:"display_name" validate:"max=20"`
	Remark      string `json:"remark" validate:"max=255"`
}

type channelUserManageRequest struct {
	Id     int    `json:"id"`
	Action string `json:"action"`
}

func channelManagedUserToView(user *model.User) channelManagedUserView {
	view := channelManagedUserView{
		Id:             user.Id,
		Username:       user.Username,
		DisplayName:    user.DisplayName,
		Role:           user.Role,
		AccountType:    user.AccountType,
		ChannelOwnerId: user.ChannelOwnerId,
		Status:         user.Status,
		Quota:          user.Quota,
		UsedQuota:      user.UsedQuota,
		RequestCount:   user.RequestCount,
		Group:          user.Group,
		Remark:         user.Remark,
		CreatedAt:      user.CreatedAt,
		LastLoginAt:    user.LastLoginAt,
	}
	if user.DeletedAt.Valid {
		view.DeletedAt = &user.DeletedAt.Time
	}
	return view
}

func writeChannelUserError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, model.ErrChannelAccountRequired):
		message := common.TranslateMessage(c, i18n.MsgUserChannelAccountRequired)
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": message})
	case errors.Is(err, model.ErrChannelUserNotFound):
		common.ApiErrorI18n(c, i18n.MsgUserNotExists)
	default:
		common.ApiError(c, err)
	}
}

func GetChannelManagedUsers(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	var status *int
	if raw := c.Query("status"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			common.ApiErrorI18n(c, i18n.MsgInvalidParams)
			return
		}
		status = &parsed
	}
	users, total, err := model.ListChannelManagedUsers(
		c.GetInt("id"), strings.TrimSpace(c.Query("keyword")), status, pageInfo,
		model.NewUserSortOptions(c.Query("sort_by"), c.Query("sort_order")),
	)
	if err != nil {
		writeChannelUserError(c, err)
		return
	}
	items := make([]channelManagedUserView, len(users))
	for i := range users {
		items[i] = channelManagedUserToView(users[i])
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func GetChannelManagedUser(c *gin.Context) {
	userID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	user, err := model.GetChannelManagedUser(c.GetInt("id"), userID)
	if err != nil {
		writeChannelUserError(c, err)
		return
	}
	common.ApiSuccess(c, channelManagedUserToView(user))
}

func CreateChannelManagedUser(c *gin.Context) {
	var req channelUserCreateRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	if err := common.Validate.Struct(&req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgUserInputInvalid, map[string]any{"Error": err.Error()})
		return
	}
	if req.DisplayName == "" {
		req.DisplayName = req.Username
	}
	user := model.User{
		Username:    req.Username,
		Password:    req.Password,
		DisplayName: req.DisplayName,
		Remark:      req.Remark,
	}
	if err := model.CreateChannelManagedUser(c.GetInt("id"), &user); err != nil {
		writeChannelUserError(c, err)
		return
	}
	recordManageAuditFor(c, user.Id, "channel_user.create", map[string]any{
		"username": user.Username,
		"id":       user.Id,
	})
	common.ApiSuccess(c, channelManagedUserToView(&user))
}

func UpdateChannelManagedUser(c *gin.Context) {
	userID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	var req channelUserUpdateRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	if err := common.Validate.Struct(&req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgUserInputInvalid, map[string]any{"Error": err.Error()})
		return
	}
	user, err := model.UpdateChannelManagedUser(c.GetInt("id"), userID, req.DisplayName, req.Remark)
	if err != nil {
		writeChannelUserError(c, err)
		return
	}
	recordManageAuditFor(c, user.Id, "channel_user.update", map[string]any{
		"username": user.Username,
		"id":       user.Id,
	})
	common.ApiSuccess(c, channelManagedUserToView(user))
}

func ManageChannelManagedUser(c *gin.Context) {
	var req channelUserManageRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil || req.Id <= 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	status := 0
	switch req.Action {
	case "enable":
		status = common.UserStatusEnabled
	case "disable":
		status = common.UserStatusDisabled
	default:
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	user, err := model.SetChannelManagedUserStatus(c.GetInt("id"), req.Id, status)
	if err != nil {
		writeChannelUserError(c, err)
		return
	}
	recordManageAuditFor(c, user.Id, "channel_user.manage", map[string]any{
		"action":   req.Action,
		"username": user.Username,
		"id":       user.Id,
	})
	common.ApiSuccess(c, channelManagedUserToView(user))
}

func DeleteChannelManagedUser(c *gin.Context) {
	userID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	user, err := model.GetChannelManagedUser(c.GetInt("id"), userID)
	if err != nil {
		writeChannelUserError(c, err)
		return
	}
	if err := model.DeleteChannelManagedUser(c.GetInt("id"), userID); err != nil {
		writeChannelUserError(c, err)
		return
	}
	recordManageAuditFor(c, user.Id, "channel_user.delete", map[string]any{
		"username": user.Username,
		"id":       user.Id,
	})
	common.ApiSuccess(c, gin.H{})
}
