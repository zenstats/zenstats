package user

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/api/types"
	"github.com/zenstats/zenstats/pkg/response"
)

// @Summary		获取子账号列表
// @Description	获取当前用户的子账号列表。
// @Tags			用户
// @Security		BearerAuth
// @Accept			json
// @Produce		json
// @Success		200	{object}	response.SuccessResponse{data=types.SubAccountListResponse}	"子账号列表"
// @Failure		401	{object}	response.ErrorResponse	"未授权"
// @Failure		500	{object}	response.ErrorResponse	"服务器内部错误"
// @Router			/user/sub-accounts [get]
func (h *UserHandler) ListSubAccounts() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetInt64("user_id")

		// 检查用户是否有子账号权限
		hasPermission, err := h.subAccountService.HasSubAccountPermission(c.Request.Context(), userID)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		subAccounts, err := h.subAccountService.GetUserSubAccounts(c.Request.Context(), userID)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		// 转换为API响应格式
		subAccountList := make([]*types.SubAccount, 0, len(subAccounts))
		for _, sa := range subAccounts {
			lastSeen := sa.LastSeen
			subAccountList = append(subAccountList, &types.SubAccount{
				ID:           sa.ID,
				Email:        sa.Email,
				Name:         sa.Name,
				Role:         sa.Role,
				Status:       sa.Status,
				LastSeen:     &lastSeen,
				CreatedAt:    sa.CreatedAt,
				UpdatedAt:    sa.UpdatedAt,
			})
		}

		// 获取最大子账号数量
		maxSubAccounts, err := h.subAccountService.GetMaxSubAccounts(c.Request.Context(), userID)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		response.Success(c, &types.SubAccountListResponse{
			SubAccounts:    subAccountList,
			HasPermission:  hasPermission,
			MaxSubAccounts: maxSubAccounts,
			CurrentCount:   len(subAccounts),
		})
	}
}

// @Summary		创建子账号
// @Description	为当前用户创建子账号。
// @Tags			用户
// @Security		BearerAuth
// @Accept			json
// @Produce		json
// @Param			request	body		types.CreateSubAccountRequest	true	"子账号信息"
// @Success		200		{object}	response.SuccessResponse{data=types.SubAccount}	"创建成功"
// @Failure		400		{object}	response.ErrorResponse		"请求参数错误"
// @Failure		401		{object}	response.ErrorResponse		"未授权"
// @Failure		403		{object}	response.ErrorResponse		"无权限"
// @Failure		409		{object}	response.ErrorResponse		"邮箱已存在"
// @Failure		500		{object}	response.ErrorResponse		"服务器内部错误"
// @Router			/user/sub-accounts [post]
func (h *UserHandler) CreateSubAccount() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetInt64("user_id")

		// 检查用户是否有子账号权限
		hasPermission, err := h.subAccountService.HasSubAccountPermission(c.Request.Context(), userID)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}
		if !hasPermission {
			response.Error(c, http.StatusForbidden, ErrNoSubAccountPermission)
			return
		}

		// 检查子账号数量限制
		currentCount, err := h.subAccountService.GetUserSubAccountCount(c.Request.Context(), userID)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		maxSubAccounts, err := h.subAccountService.GetMaxSubAccounts(c.Request.Context(), userID)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		if maxSubAccounts != -1 && currentCount >= maxSubAccounts {
			response.Error(c, http.StatusForbidden, ErrSubAccountLimitReached)
			return
		}

		var req types.CreateSubAccountRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		subAccount, err := h.subAccountService.CreateSubAccount(c.Request.Context(), userID, req.Email, req.Name, req.Password)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		response.Success(c, &types.SubAccount{
			ID:        subAccount.ID,
			Email:     subAccount.Email,
			Name:      subAccount.Name,
			Role:      subAccount.Role,
			Status:    subAccount.Status,
			CreatedAt: subAccount.CreatedAt,
			UpdatedAt: subAccount.UpdatedAt,
		})
	}
}

// @Summary		更新子账号
// @Description	更新指定的子账号信息。
// @Tags			用户
// @Security		BearerAuth
// @Accept			json
// @Produce		json
// @Param			id		path		int							true	"子账号ID"
// @Param			request	body		types.UpdateSubAccountRequest	true	"子账号信息"
// @Success		200		{object}	response.SuccessResponse		"更新成功"
// @Failure		400		{object}	response.ErrorResponse		"请求参数错误"
// @Failure		401		{object}	response.ErrorResponse		"未授权"
// @Failure		403		{object}	response.ErrorResponse		"无权限"
// @Failure		404		{object}	response.ErrorResponse		"子账号不存在"
// @Failure		500		{object}	response.ErrorResponse		"服务器内部错误"
// @Router			/user/sub-accounts/{id} [put]
func (h *UserHandler) UpdateSubAccount() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetInt64("user_id")
		subAccountIDStr := c.Param("id")
		subAccountID, err := strconv.ParseInt(subAccountIDStr, 10, 64)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		// 检查子账号是否属于当前用户
		subAccount, err := h.subAccountService.GetSubAccountByID(c.Request.Context(), subAccountID)
		if err != nil {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		if subAccount.ParentUserID != userID {
			response.Error(c, http.StatusForbidden, ErrNotSubAccountOwner)
			return
		}

		var req types.UpdateSubAccountRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		_, err = h.subAccountService.UpdateSubAccount(c.Request.Context(), subAccountID, req.Name, req.Status)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		response.Success(c, nil)
	}
}

// @Summary		删除子账号
// @Description	删除指定的子账号。
// @Tags			用户
// @Security		BearerAuth
// @Accept			json
// @Produce		json
// @Param			id	path		int	true	"子账号ID"
// @Success		200	{object}	response.SuccessResponse	"删除成功"
// @Failure		401	{object}	response.ErrorResponse	"未授权"
// @Failure		403	{object}	response.ErrorResponse	"无权限"
// @Failure		404	{object}	response.ErrorResponse	"子账号不存在"
// @Failure		500	{object}	response.ErrorResponse	"服务器内部错误"
// @Router			/user/sub-accounts/{id} [delete]
func (h *UserHandler) DeleteSubAccount() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetInt64("user_id")
		subAccountIDStr := c.Param("id")
		subAccountID, err := strconv.ParseInt(subAccountIDStr, 10, 64)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		// 检查子账号是否属于当前用户
		subAccount, err := h.subAccountService.GetSubAccountByID(c.Request.Context(), subAccountID)
		if err != nil {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		if subAccount.ParentUserID != userID {
			response.Error(c, http.StatusForbidden, ErrNotSubAccountOwner)
			return
		}

		err = h.subAccountService.DeleteSubAccount(c.Request.Context(), subAccountID)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		response.Success(c, nil)
	}
}

// @Summary		重置子账号密码
// @Description	重置指定子账号的密码。
// @Tags			用户
// @Security		BearerAuth
// @Accept			json
// @Produce		json
// @Param			id		path		int								true	"子账号ID"
// @Param			request	body		types.ResetSubAccountPasswordRequest	true	"新密码"
// @Success		200		{object}	response.SuccessResponse		"重置成功"
// @Failure		400		{object}	response.ErrorResponse		"请求参数错误"
// @Failure		401		{object}	response.ErrorResponse		"未授权"
// @Failure		403		{object}	response.ErrorResponse		"无权限"
// @Failure		404		{object}	response.ErrorResponse		"子账号不存在"
// @Failure		500		{object}	response.ErrorResponse		"服务器内部错误"
// @Router			/user/sub-accounts/{id}/reset-password [post]
func (h *UserHandler) ResetSubAccountPassword() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetInt64("user_id")
		subAccountIDStr := c.Param("id")
		subAccountID, err := strconv.ParseInt(subAccountIDStr, 10, 64)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		// 检查子账号是否属于当前用户
		subAccount, err := h.subAccountService.GetSubAccountByID(c.Request.Context(), subAccountID)
		if err != nil {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		if subAccount.ParentUserID != userID {
			response.Error(c, http.StatusForbidden, ErrNotSubAccountOwner)
			return
		}

		var req types.ResetSubAccountPasswordRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		err = h.subAccountService.ResetSubAccountPassword(c.Request.Context(), subAccountID, req.NewPassword)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		response.Success(c, nil)
	}
}

// 错误定义
var (
	ErrNoSubAccountPermission = &NoSubAccountPermissionError{}
	ErrSubAccountLimitReached = &SubAccountLimitReachedError{}
	ErrNotSubAccountOwner     = &NotSubAccountOwnerError{}
)

type NoSubAccountPermissionError struct{}

func (e *NoSubAccountPermissionError) Error() string {
	return "you don't have permission to create sub accounts"
}

type SubAccountLimitReachedError struct{}

func (e *SubAccountLimitReachedError) Error() string {
	return "sub account limit reached"
}

type NotSubAccountOwnerError struct{}

func (e *NotSubAccountOwnerError) Error() string {
	return "sub account does not belong to you"
}
