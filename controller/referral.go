package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

func GetReferralOverview(c *gin.Context) {
	overview, err := model.GetReferralOverview(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, overview)
}

func GetReferralInvitees(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	if page, err := strconv.Atoi(c.Query("page")); err == nil && page > 0 {
		pageInfo.Page = page
	}
	invitees, total, err := model.GetReferralInvitees(c.GetInt("id"), pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    invitees,
		"total":   total,
	})
}

func GetReferralRecords(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	if page, err := strconv.Atoi(c.Query("page")); err == nil && page > 0 {
		pageInfo.Page = page
	}
	records, total, err := model.GetReferralRecords(c.GetInt("id"), pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    records,
		"total":   total,
	})
}
