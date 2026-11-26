package auth

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/api/types"
	"github.com/zenstats/zenstats/internal/auth"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent"
	"github.com/zenstats/zenstats/pkg/response"
)

// loginService keeps login testable without initializing external databases.
type loginService interface {
	GetUserByEmail(context.Context, string) (*ent.User, error)
	CheckPassword(context.Context, *ent.User, string) bool
	GetUserStatus(context.Context, int64) string
}

// @Summary		用户登录
// @Description	通过邮箱和密码登录系统并获取访问令牌和刷新令牌。
// @Tags			认证
// @Accept			json
// @Produce		json
// @Param			request	body		types.LoginRequest					true	"登录请求参数"
// @Success		200		{object}	response.SuccessResponse{data=types.LoginResponse}	"登录成功返回令牌和用户信息"
// @Failure		400		{object}	response.ErrorResponse	"请求参数错误"
// @Failure		401		{object}	response.ErrorResponse	"认证失败"
// @Failure		403		{object}	response.ErrorResponse	"邮箱未验证"
// @Failure		500		{object}	response.ErrorResponse	"服务器内部错误"
// @Router			/auth/login [post]
func (h *AuthHandler) Login() gin.HandlerFunc {
	return login(h.service)
}

func login(svc loginService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.LoginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		user, err := svc.GetUserByEmail(c.Request.Context(), req.Email)
		if err != nil {
			if ent.IsNotFound(err) {
				// 与密码错误返回相同的状态码与文案，避免通过响应差异枚举已注册邮箱
				response.ErrorWithKey(c, http.StatusUnauthorized, "auth.invalid_credentials")
				return
			}
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		// 验证密码
		if !svc.CheckPassword(c, user, req.Password) {
			response.ErrorWithKey(c, http.StatusUnauthorized, "auth.invalid_credentials")
			return
		}

		// 拒绝已被管理员停用的父账号（此前只校验密码，停用不生效）
		if svc.GetUserStatus(c.Request.Context(), user.ID) == "suspended" {
			response.Error(c, http.StatusForbidden, errors.New("account suspended"))
			return
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
		data := &types.LoginResponse{
			Token:        token,
			RefreshToken: refreshToken,
			User: &types.User{
				ID:            user.ID,
				Email:         user.Email,
				Name:          user.Name,
				IsAdmin:       user.IsAdmin,
				EmailVerified: user.EmailVerified,
			},
		}

		response.Success(c, data)
	}
}
