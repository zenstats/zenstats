package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/zenstats/zenstats/internal/auth"
	"github.com/zenstats/zenstats/internal/service"
	"github.com/zenstats/zenstats/pkg/response"
)

// APIKeyOrJWTAuth 支持 API Key 或 JWT Token 认证的中间件。
// API Key 通过 Authorization: Bearer zen_xxx 传递，以 "zen_" 前缀区分。
// JWT Token 通过标准 Bearer token 传递。
func APIKeyOrJWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := c.GetHeader("Authorization")
		if tokenString == "" {
			tokenString = c.Query("token")
		}
		if len(tokenString) > 7 && tokenString[:7] == "Bearer " {
			tokenString = tokenString[7:]
		}
		if tokenString == "" {
			response.Error(c, http.StatusUnauthorized, errors.New("auth token not provided"))
			c.Abort()
			return
		}

		// 判断是 API Key 还是 JWT Token
		if strings.HasPrefix(tokenString, "zen_") {
			// API Key 认证
			apiKeyService := service.GetAPIKeyService()
			userID, err := apiKeyService.ValidateAPIKey(c, tokenString)
			if err != nil {
				response.Error(c, http.StatusUnauthorized, errors.New("invalid api key"))
				c.Abort()
				return
			}
			c.Set("user_id", userID)
			c.Set("auth_type", "api_key")
		} else {
			// JWT Token 认证
			claims, err := auth.ParseToken(tokenString)
			if err != nil {
				if errors.Is(err, jwt.ErrTokenExpired) {
					response.Error(c, 430, errors.New("token expired"))
					c.Abort()
					return
				}
				response.Error(c, http.StatusUnauthorized, errors.New("invalid token"))
				c.Abort()
				return
			}
			c.Set("user_id", claims.UserID)
			c.Set("auth_type", "jwt")
		}

		c.Next()
	}
}
