package auth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/api/types"
	"github.com/zenstats/zenstats/internal/auth"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent"
	"github.com/zenstats/zenstats/pkg/response"
)

// @Summary		用户登录
// @Description	通过邮箱和密码登录系统并获取访问令牌
// @Tags			auth
// @Accept			json
// @Produce		json
// @Param			request	body		types.LoginRequest		true	"登录请求参数"
// @Success		200		{object}	types.LoginResponse		"登录成功返回令牌和用户信息"
// @Failure		400		{object}	response.ErrorResponse	"请求参数错误"
// @Failure		401		{object}	response.ErrorResponse	"密码错误"
// @Failure		404		{object}	response.ErrorResponse	"用户不存在"
// @Failure		500		{object}	response.ErrorResponse	"服务器内部错误"
// @Router			/auth/login [post]
func (h *AuthHandler) Login() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.LoginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		user, err := h.service.GetUserByEmail(c.Request.Context(), req.Email)
		if err != nil {
			if ent.IsNotFound(err) {
				response.Error(c, http.StatusNotFound, errors.New("user not found"))
				return
			}
			response.Error(c, http.StatusBadRequest, err)
			return
		}
		// 验证密码
		if !h.service.CheckPassword(c, user, req.Password) {
			response.Error(c, http.StatusUnauthorized, errors.New("invalid password"))
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
				Email: user.Email,
				Name:  user.Name,
			},
		}

		response.Success(c, data)
	}
}
