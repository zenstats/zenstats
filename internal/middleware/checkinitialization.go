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
		// Check if the user table is empty
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
