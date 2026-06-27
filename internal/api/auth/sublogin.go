package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/auth"
	"github.com/zenstats/zenstats/internal/api/types"
	"github.com/zenstats/zenstats/pkg/response"
)

// @Summary		子账号登录
// @Description	子账号通过邮箱和密码登录系统。
// @Tags			认证
// @Accept			json
// @Produce		json
// @Param			body	body		types.SubAccountLoginRequest	true	"子账号登录参数"
// @Success		200		{object}	response.SuccessResponse{data=types.SubAccountLoginResponse}	"登录成功"
// @Failure		400		{object}	response.ErrorResponse	"请求参数错误"
// @Failure		401		{object}	response.ErrorResponse	"认证失败"
// @Failure		403		{object}	response.ErrorResponse	"账号已被禁用"
// @Router			/auth/sub-login [post]
func (h *AuthHandler) SubLogin() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.SubAccountLoginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		subAccount, err := h.subAccountService.SubAccountLogin(c.Request.Context(), req.Email, req.Password)
		if err != nil {
			response.ErrorWithKey(c, http.StatusUnauthorized, "auth.invalid_credentials")
			return
		}

		// 获取父用户信息
		parentUser, err := h.service.GetUserByID(c.Request.Context(), subAccount.ParentUserID)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		// 生成子账号 Token
		token, err := auth.GenerateSubAccountToken(subAccount.ID, subAccount.ParentUserID, subAccount.Role, subAccount.Permissions)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		refreshToken, err := auth.GenerateSubAccountRefreshToken(subAccount.ID, subAccount.ParentUserID, subAccount.Role, subAccount.Permissions)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		perms := subAccount.Permissions
		if perms == nil {
			perms = []string{}
		}
		response.Success(c, &types.SubAccountLoginResponse{
			Token:        token,
			RefreshToken: refreshToken,
			User: &types.SubAccountUser{
				ID:           subAccount.ID,
				Email:        subAccount.Email,
				Name:         subAccount.Name,
				Role:         subAccount.Role,
				Permissions:  perms,
				ParentUserID: parentUser.ID,
			},
		})
	}
}
