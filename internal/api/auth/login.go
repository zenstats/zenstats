package auth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/auth"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent"
	"github.com/zenstats/zenstats/pkg/response"
)

func (h *AuthHandler) Login() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req LoginRequest
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
		data := &LoginResponse{
			Token:        token,
			RefreshToken: refreshToken,
			User: &User{
				Email: user.Email,
				Name:  user.Name,
			},
		}

		response.Success(c, data)
	}
}
