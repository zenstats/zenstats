package user

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/api/types"
	"github.com/zenstats/zenstats/internal/event"
	"github.com/zenstats/zenstats/pkg/response"
)

// @Summary		获取用户额度信息
// @Description	获取当前用户的套餐额度及使用情况。
// @Tags			用户
// @Accept			json
// @Produce		json
// @Success		200	{object}	response.SuccessResponse{data=types.UserQuotaInfo}	"额度信息"
// @Failure		401	{object}	response.ErrorResponse	"未授权"
// @Failure		500	{object}	response.ErrorResponse	"服务器内部错误"
// @Router			/user/quota [get]
func (h *UserHandler) GetQuota() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetInt64("user_id")

		user, err := h.userService.GetUserWithConfig(c.Request.Context(), userID)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		// 默认值（无配置时）
		groupName := "Free"
		maxSites := 3
		maxMonthlyEvents := 10000
		maxAPIKeys := 2
		maxSubAccounts := 0
		customSearchEngines := false

		if user.Edges.UserConfig != nil && user.Edges.UserConfig.Edges.Group != nil {
			g := user.Edges.UserConfig.Edges.Group
			groupName = g.Name
			maxSites = g.MaxSites
			maxMonthlyEvents = g.MaxMonthlyEvents
			maxAPIKeys = g.MaxAPIKeys
			maxSubAccounts = g.MaxSubAccounts
			customSearchEngines = g.CustomSearchEngines
		}

		// 获取当前使用量
		currentSites, _ := h.siteService.GetUserSiteCount(c.Request.Context(), userID)
		currentAPIKeys, _ := h.apiKeyService.GetUserAPIKeyCount(c.Request.Context(), userID)
		currentSubAccounts, _ := h.subAccountService.GetUserSubAccountCount(c.Request.Context(), userID)
		currentMonthlyEvents := event.GetMonthlyQuota().Get(userID)

		response.Success(c, &types.UserQuotaInfo{
			GroupName:            groupName,
			MaxSites:             maxSites,
			MaxMonthlyEvents:     maxMonthlyEvents,
			MaxAPIKeys:           maxAPIKeys,
			MaxSubAccounts:       maxSubAccounts,
			CustomSearchEngines:  customSearchEngines,
			CurrentSites:         currentSites,
			CurrentMonthlyEvents: currentMonthlyEvents,
			CurrentAPIKeys:       currentAPIKeys,
			CurrentSubAccounts:   currentSubAccounts,
		})
	}
}
