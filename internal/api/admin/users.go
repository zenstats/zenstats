package admin

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/api/types"
	"github.com/zenstats/zenstats/pkg/response"
)

// @Summary		获取用户列表
// @Description	获取所有用户的列表，支持分页和搜索。
// @Tags			管理员
// @Accept			json
// @Produce		json
// @Param			page		query		int		false	"页码"		default(1)
// @Param			page_size	query		int		false	"每页数量"	default(20)
// @Param			search		query		string	false	"搜索关键词"
// @Success		200			{object}	response.SuccessResponse{data=types.AdminUserListResponse}	"用户列表"
// @Failure		401			{object}	response.ErrorResponse	"未授权"
// @Failure		500			{object}	response.ErrorResponse	"服务器内部错误"
// @Router			/admin/users [get]
func (h *AdminHandler) ListUsers() gin.HandlerFunc {
	return func(c *gin.Context) {
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

		if page < 1 {
			page = 1
		}
		if pageSize < 1 || pageSize > 100 {
			pageSize = 20
		}

		offset := (page - 1) * pageSize

		users, err := h.userService.GetAllUsers(c.Request.Context(), offset, pageSize)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		totalCount, err := h.userService.GetUserCountAdmin(c.Request.Context())
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		// 转换为API响应格式
		userList := make([]*types.AdminUser, 0, len(users))
		for _, u := range users {
			adminUser := &types.AdminUser{
				ID:        u.ID,
				Email:     u.Email,
				Name:      u.Name,
				IsAdmin:   u.IsAdmin,
				CreatedAt: u.CreatedAt,
				UpdatedAt: u.UpdatedAt,
			}

			// 获取用户配置
			if u.Edges.UserConfig != nil {
				adminUser.Status = u.Edges.UserConfig.Status
				if u.Edges.UserConfig.Edges.Group != nil {
					adminUser.GroupName = u.Edges.UserConfig.Edges.Group.Name
				}
			}

			userList = append(userList, adminUser)
		}

		response.Success(c, &types.AdminUserListResponse{
			Users:      userList,
			TotalCount: totalCount,
			Page:       page,
			PageSize:   pageSize,
		})
	}
}

// @Summary		获取用户详情
// @Description	获取指定用户的详细信息。
// @Tags			管理员
// @Accept			json
// @Produce		json
// @Param			userId	path		int	true	"用户ID"
// @Success		200		{object}	response.SuccessResponse{data=types.AdminUserDetail}	"用户详情"
// @Failure		401		{object}	response.ErrorResponse	"未授权"
// @Failure		404		{object}	response.ErrorResponse	"用户不存在"
// @Failure		500		{object}	response.ErrorResponse	"服务器内部错误"
// @Router			/admin/users/{userId} [get]
func (h *AdminHandler) GetUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		userIdStr := c.Param("userId")
		userId, err := strconv.ParseInt(userIdStr, 10, 64)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		user, err := h.userService.GetUserWithConfig(c.Request.Context(), userId)
		if err != nil {
			response.Error(c, http.StatusNotFound, err)
			return
		}

		adminUser := &types.AdminUserDetail{
			ID:        user.ID,
			Email:     user.Email,
			Name:      user.Name,
			IsAdmin:   user.IsAdmin,
			CreatedAt: user.CreatedAt,
			UpdatedAt: user.UpdatedAt,
		}

		// 获取用户配置
		if user.Edges.UserConfig != nil {
			adminUser.Status = user.Edges.UserConfig.Status
			if user.Edges.UserConfig.Edges.Group != nil {
				adminUser.GroupID = user.Edges.UserConfig.Edges.Group.ID
				adminUser.GroupName = user.Edges.UserConfig.Edges.Group.Name
				adminUser.MaxSites = user.Edges.UserConfig.Edges.Group.MaxSites
				adminUser.MaxMonthlyEvents = user.Edges.UserConfig.Edges.Group.MaxMonthlyEvents
				adminUser.MaxAPIKeys = user.Edges.UserConfig.Edges.Group.MaxAPIKeys
				adminUser.MaxSubAccounts = user.Edges.UserConfig.Edges.Group.MaxSubAccounts
				adminUser.CustomSearchEngines = user.Edges.UserConfig.Edges.Group.CustomSearchEngines
			}
		}

		// 获取用户站点数量
		siteCount, err := h.siteService.GetUserSiteCount(c.Request.Context(), userId)
		if err == nil {
			adminUser.SiteCount = siteCount
		}

		response.Success(c, adminUser)
	}
}

// @Summary		更新用户套餐
// @Description	更新指定用户的套餐配置。
// @Tags			管理员
// @Accept			json
// @Produce		json
// @Param			userId	path		int						true	"用户ID"
// @Param			request	body		types.UpdateUserGroupRequest	true	"套餐配置"
// @Success		200		{object}	response.SuccessResponse		"更新成功"
// @Failure		400		{object}	response.ErrorResponse		"请求参数错误"
// @Failure		401		{object}	response.ErrorResponse		"未授权"
// @Failure		500		{object}	response.ErrorResponse		"服务器内部错误"
// @Router			/admin/users/{userId}/group [put]
func (h *AdminHandler) UpdateUserGroup() gin.HandlerFunc {
	return func(c *gin.Context) {
		userIdStr := c.Param("userId")
		userId, err := strconv.ParseInt(userIdStr, 10, 64)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		var req types.UpdateUserGroupRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		err = h.userService.UpdateUserGroup(c.Request.Context(), userId, req.GroupID)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		response.Success(c, nil)
	}
}

// @Summary		更新用户状态
// @Description	更新指定用户的状态（启用/禁用）。
// @Tags			管理员
// @Accept			json
// @Produce		json
// @Param			userId	path		int							true	"用户ID"
// @Param			request	body		types.UpdateUserStatusRequest	true	"状态配置"
// @Success		200		{object}	response.SuccessResponse		"更新成功"
// @Failure		400		{object}	response.ErrorResponse		"请求参数错误"
// @Failure		401		{object}	response.ErrorResponse		"未授权"
// @Failure		500		{object}	response.ErrorResponse		"服务器内部错误"
// @Router			/admin/users/{userId}/status [put]
func (h *AdminHandler) UpdateUserStatus() gin.HandlerFunc {
	return func(c *gin.Context) {
		userIdStr := c.Param("userId")
		userId, err := strconv.ParseInt(userIdStr, 10, 64)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		var req types.UpdateUserStatusRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		err = h.userService.UpdateUserStatus(c.Request.Context(), userId, req.Status)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		response.Success(c, nil)
	}
}
