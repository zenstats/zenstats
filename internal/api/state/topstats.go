package state

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/pkg/response"
)

func (s *StateHandle) GetTopStats() gin.HandlerFunc {
	return func(c *gin.Context) {
		stats, err := s.service.GetTopStats()
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		response.Success(c, stats)
	}
}
