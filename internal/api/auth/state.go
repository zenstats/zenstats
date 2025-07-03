package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/pkg/response"
)

func (h *AuthHandler) State() gin.HandlerFunc {
	return func(c *gin.Context) {
		count, err := h.service.GetUserCount(c)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}
		if count == 0 {
			response.Success(c, "not_initialized")
			return
		}
		response.Success(c, "initialized")
	}
}
