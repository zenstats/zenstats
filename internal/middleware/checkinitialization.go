package middleware

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/pkg/globals"
	"github.com/zenstats/zenstats/pkg/response"
)

func CheckInitialization() gin.HandlerFunc {
	return func(c *gin.Context) {
		db := globals.GetDB()
		// 查询用户表是否为空
		count, err := db.Client.User.Query().Count(c.Request.Context())
		if err != nil {
			response.Error(c, http.StatusInternalServerError, errors.New("failed to query user table"))
			c.Abort()
			return
		}
		if count > 0 {
			response.ErrorWithKey(c, http.StatusForbidden, "auth.system_initialized")
			c.Abort()
			return
		}

		c.Next()
	}
}
