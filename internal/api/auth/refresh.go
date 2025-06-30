package auth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	authJwt "github.com/zenstats/zenstats/internal/auth"
	"github.com/zenstats/zenstats/pkg/response"
)

func (h *AuthHandler) Refresh() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req RefreshRequest
		if err := c.ShouldBindQuery(&req); err != nil {
			response.Error(c, http.StatusBadRequest, errors.New("invalid refresh token"+err.Error()))
			return
		}

		claims, err := authJwt.ParseToken(req.RefreshToken)

		if err != nil {
			if errors.Is(err, jwt.ErrTokenExpired) {
				response.Error(c, 430, errors.New("the refresh token has expired"))
				return
			}
			response.Error(c, http.StatusUnauthorized, errors.New("invalid refresh token"))
			return
		}

		if claims.Subject != "refresh_token" {
			response.Error(c, http.StatusUnauthorized, errors.New("invalid refresh token"))
			return
		}

		token, err := authJwt.GenerateToken(claims.UserID)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		response.Success(c, token)
	}
}
