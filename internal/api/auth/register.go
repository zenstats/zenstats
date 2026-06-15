package auth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/api/types"
	"github.com/zenstats/zenstats/internal/auth"
	"github.com/zenstats/zenstats/internal/service"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent"
	"github.com/zenstats/zenstats/pkg/response"
)

// @Summary		用户注册
// @Description	通过邮箱、名称和密码注册新用户账号。
// @Tags			认证
// @Accept			json
// @Produce		json
// @Param			request	body		types.RegisterRequest				true	"注册请求参数"
// @Success		200		{object}	response.SuccessResponse{data=types.RegisterResponse}	"注册成功返回令牌和用户信息"
// @Failure		400		{object}	response.ErrorResponse	"请求参数错误"
// @Failure		409		{object}	response.ErrorResponse	"邮箱已存在"
// @Failure		500		{object}	response.ErrorResponse	"服务器内部错误"
// @Router			/auth/register [post]
func (h *AuthHandler) Register() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.RegisterRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		// 检查注册功能是否开启
		configService := service.GetSystemConfigService()
		if !configService.IsRegistrationEnabled(c.Request.Context()) {
			response.Error(c, http.StatusForbidden, errors.New("registration is disabled"))
			return
		}

		// 检查邮箱是否已存在
		_, err := h.service.GetUserByEmail(c.Request.Context(), req.Email)
		if err == nil {
			response.Error(c, http.StatusConflict, errors.New("email already exists"))
			return
		}
		if !ent.IsNotFound(err) {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		// 检查是否是第一个用户（第一个用户自动成为管理员）
		userCount, err := h.service.GetUserCount(c.Request.Context())
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}
		isFirstUser := userCount == 0

		// 创建用户
		user, err := h.service.CreateUser(c.Request.Context(), req.Name, req.Email, req.Password)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		// 如果是第一个用户，设置为管理员（管理员不需要邮箱验证）
		if isFirstUser {
			err = h.service.SetAdmin(c.Request.Context(), user.ID, true)
			if err != nil {
				// 即使设置管理员失败，用户仍然可以登录
				// 这里记录日志但不返回错误
			}
		}

		// 为新用户创建配置（使用默认套餐）
		err = h.service.CreateUserConfig(c.Request.Context(), user.ID)
		if err != nil {
			// 即使配置创建失败，用户仍然可以登录，只是没有套餐配置
			// 这里记录日志但不返回错误
		}

		// 发送验证邮件（管理员不需要验证）
		if !isFirstUser {
			emailService := service.GetEmailService()
			baseURL := service.GetBaseURLFromRequest(c)
			_ = emailService.SendVerificationEmail(c.Request.Context(), user.ID, user.Email, baseURL)
		}

		// 颁发jwt token
		token, err := auth.GenerateToken(user.ID)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}
		refreshToken, err := auth.GenerateRefreshToken(user.ID)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}
		data := &types.RegisterResponse{
			Token:        token,
			RefreshToken: refreshToken,
			User: &types.User{
				ID:            user.ID,
				Email:         user.Email,
				Name:          user.Name,
				IsAdmin:       isFirstUser,
				EmailVerified: isFirstUser, // 管理员默认已验证
			},
		}

		response.Success(c, data)
	}
}
