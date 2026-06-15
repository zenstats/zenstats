package admin

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/api/types"
	"github.com/zenstats/zenstats/internal/service"
	"github.com/zenstats/zenstats/pkg/response"
)

// @Summary		获取套餐列表
// @Description	获取所有套餐的列表。
// @Tags			管理员
// @Accept			json
// @Produce		json
// @Success		200	{object}	response.SuccessResponse{data=types.AdminGroupListResponse}	"套餐列表"
// @Failure		401	{object}	response.ErrorResponse	"未授权"
// @Failure		500	{object}	response.ErrorResponse	"服务器内部错误"
// @Router			/admin/groups [get]
func (h *AdminHandler) ListGroups() gin.HandlerFunc {
	return func(c *gin.Context) {
		groups, err := h.userGroupService.GetAllGroups(c.Request.Context())
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		// 转换为API响应格式
		groupList := make([]*types.AdminGroup, 0, len(groups))
		for _, g := range groups {
			adminGroup := &types.AdminGroup{
				ID:                  g.ID,
				Name:                g.Name,
				Description:         g.Description,
				MaxSites:            g.MaxSites,
				MaxMonthlyEvents:    g.MaxMonthlyEvents,
				MaxAPIKeys:          g.MaxAPIKeys,
				MaxSubAccounts:      g.MaxSubAccounts,
				CustomSearchEngines: g.CustomSearchEngines,
				IsDefault:           g.IsDefault,
				Price:               g.Price,
				CreatedAt:           g.CreatedAt,
				UpdatedAt:           g.UpdatedAt,
			}

			// 获取用户数量
			userCount, err := h.userGroupService.GetGroupUserCount(c.Request.Context(), g.ID)
			if err == nil {
				adminGroup.UserCount = userCount
			}

			groupList = append(groupList, adminGroup)
		}

		response.Success(c, &types.AdminGroupListResponse{
			Groups: groupList,
		})
	}
}

// @Summary		创建套餐
// @Description	创建新的套餐配置。
// @Tags			管理员
// @Accept			json
// @Produce		json
// @Param			request	body		types.CreateGroupRequest	true	"套餐配置"
// @Success		200		{object}	response.SuccessResponse{data=types.AdminGroup}	"创建成功"
// @Failure		400		{object}	response.ErrorResponse		"请求参数错误"
// @Failure		401		{object}	response.ErrorResponse		"未授权"
// @Failure		500		{object}	response.ErrorResponse		"服务器内部错误"
// @Router			/admin/groups [post]
func (h *AdminHandler) CreateGroup() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.CreateGroupRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		group, err := h.userGroupService.CreateGroup(
			c.Request.Context(),
			req.Name,
			req.Description,
			req.MaxSites,
			req.MaxMonthlyEvents,
			req.MaxAPIKeys,
			req.MaxSubAccounts,
			req.CustomSearchEngines,
			req.IsDefault,
			req.Price,
		)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		adminGroup := &types.AdminGroup{
			ID:                  group.ID,
			Name:                group.Name,
			Description:         group.Description,
			MaxSites:            group.MaxSites,
			MaxMonthlyEvents:    group.MaxMonthlyEvents,
			MaxAPIKeys:          group.MaxAPIKeys,
			MaxSubAccounts:      group.MaxSubAccounts,
			CustomSearchEngines: group.CustomSearchEngines,
			IsDefault:           group.IsDefault,
			Price:               group.Price,
			CreatedAt:           group.CreatedAt,
			UpdatedAt:           group.UpdatedAt,
		}

		response.Success(c, adminGroup)
	}
}

// @Summary		更新套餐
// @Description	更新指定套餐的配置。
// @Tags			管理员
// @Accept			json
// @Produce		json
// @Param			groupId	path		int						true	"套餐ID"
// @Param			request	body		types.UpdateGroupRequest	true	"套餐配置"
// @Success		200		{object}	response.SuccessResponse	"更新成功"
// @Failure		400		{object}	response.ErrorResponse		"请求参数错误"
// @Failure		401		{object}	response.ErrorResponse		"未授权"
// @Failure		500		{object}	response.ErrorResponse		"服务器内部错误"
// @Router			/admin/groups/{groupId} [put]
func (h *AdminHandler) UpdateGroup() gin.HandlerFunc {
	return func(c *gin.Context) {
		groupIdStr := c.Param("groupId")
		groupId, err := strconv.ParseInt(groupIdStr, 10, 64)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		var req types.UpdateGroupRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		_, err = h.userGroupService.UpdateGroup(
			c.Request.Context(),
			groupId,
			req.Name,
			req.Description,
			req.MaxSites,
			req.MaxMonthlyEvents,
			req.MaxAPIKeys,
			req.MaxSubAccounts,
			req.CustomSearchEngines,
			req.IsDefault,
			req.Price,
		)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		response.Success(c, nil)
	}
}

// @Summary		删除套餐
// @Description	删除指定套餐。
// @Tags			管理员
// @Accept			json
// @Produce		json
// @Param			groupId	path		int	true	"套餐ID"
// @Success		200		{object}	response.SuccessResponse	"删除成功"
// @Failure		400		{object}	response.ErrorResponse		"请求参数错误"
// @Failure		401		{object}	response.ErrorResponse		"未授权"
// @Failure		409		{object}	response.ErrorResponse		"套餐正在使用中"
// @Failure		500		{object}	response.ErrorResponse		"服务器内部错误"
// @Router			/admin/groups/{groupId} [delete]
func (h *AdminHandler) DeleteGroup() gin.HandlerFunc {
	return func(c *gin.Context) {
		groupIdStr := c.Param("groupId")
		groupId, err := strconv.ParseInt(groupIdStr, 10, 64)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		err = h.userGroupService.DeleteGroup(c.Request.Context(), groupId)
		if err != nil {
			if err == service.ErrGroupHasUsers {
				response.Error(c, http.StatusConflict, err)
				return
			}
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		response.Success(c, nil)
	}
}

// @Summary		获取系统统计
// @Description	获取系统统计数据，包括用户数、站点数等。
// @Tags			管理员
// @Accept			json
// @Produce		json
// @Success		200	{object}	response.SuccessResponse{data=types.SystemStats}	"系统统计"
// @Failure		401	{object}	response.ErrorResponse	"未授权"
// @Failure		500	{object}	response.ErrorResponse	"服务器内部错误"
// @Router			/admin/stats [get]
func (h *AdminHandler) GetStats() gin.HandlerFunc {
	return func(c *gin.Context) {
		userCount, err := h.userService.GetUserCountAdmin(c.Request.Context())
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		groups, err := h.userGroupService.GetAllGroups(c.Request.Context())
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		// 统计各套餐用户数
		groupStats := make([]*types.GroupStat, 0, len(groups))
		for _, g := range groups {
			userCount, err := h.userGroupService.GetGroupUserCount(c.Request.Context(), g.ID)
			if err != nil {
				continue
			}
			groupStats = append(groupStats, &types.GroupStat{
				GroupID:   g.ID,
				GroupName: g.Name,
				UserCount: userCount,
			})
		}

		response.Success(c, &types.SystemStats{
			UserCount:  userCount,
			GroupStats: groupStats,
		})
	}
}
