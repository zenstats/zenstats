package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/api/types"
	"github.com/zenstats/zenstats/internal/auth"
	"github.com/zenstats/zenstats/pkg/response"
)

func (h *AuthHandler) Initialize() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.InitializeRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		user, err := h.service.CreateUser(c.Request.Context(), req.Name, req.Email, req.Password)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
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
		data := &types.InitializeResponse{
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
