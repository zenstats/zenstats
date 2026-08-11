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

// APIKeyOrJWTAuth supports authentication via API Key or JWT token.
// API keys use Authorization: Bearer zen_xxx, distinguished by the "zen_" prefix.
// JWT tokens use the standard Bearer token format.
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
			response.ErrorWithKey(c, http.StatusUnauthorized, "auth.token_not_provided")
			c.Abort()
			return
		}

		// Determine whether it is an API key or JWT token
		if strings.HasPrefix(tokenString, "zen_") {
			// API key authentication
			apiKeyService := service.GetAPIKeyService()
			userID, err := apiKeyService.ValidateAPIKey(c, tokenString)
			if err != nil {
				response.ErrorWithKey(c, http.StatusUnauthorized, "apikeys.invalid")
				c.Abort()
				return
			}
			c.Set("user_id", userID)
			c.Set("auth_type", "api_key")
		} else {
			// JWT token authentication
			claims, err := auth.ParseToken(tokenString)
			if err != nil {
				if errors.Is(err, jwt.ErrTokenExpired) {
					response.ErrorWithKey(c, 430, "auth.token_expired")
					c.Abort()
					return
				}
				response.ErrorWithKey(c, http.StatusUnauthorized, "auth.invalid_token")
				c.Abort()
				return
			}
			c.Set("user_id", claims.UserID)
			c.Set("auth_type", "jwt")
			// 与 JWTAuth 保持一致地写入子账号身份/权限，否则 SubAccountHasPerm 因缺少 user_type
			// 而误判为非子账号并放行——会让无对应写权限的子账号绕过 v2 路由上的权限门控。
			c.Set("user_type", claims.UserType)
			c.Set("sub_account_id", claims.SubAccountID)
			c.Set("role", claims.Role)
			permSet := make(map[string]struct{}, len(claims.Permissions))
			for _, p := range claims.Permissions {
				permSet[p] = struct{}{}
			}
			c.Set("permissions", permSet)
		}

		c.Next()
	}
}
