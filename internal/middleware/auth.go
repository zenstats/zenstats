package middleware

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/zenstats/zenstats/internal/auth"
	"github.com/zenstats/zenstats/pkg/response"
)

func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从 Header 或 Query 中获取 Token
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

		// 解析 Token
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

		// 将用户信息存入 Context
		c.Set("user_id", claims.UserID)
		c.Next()
	}
}
