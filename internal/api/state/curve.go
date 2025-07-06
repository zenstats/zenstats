package state

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/pkg/response"
)

func (s *StateHandle) GetCurve() gin.HandlerFunc {
	return func(c *gin.Context) {
		domain := c.Param("domain")

		req, err := s.validate(c)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		stats, err := s.service.GetCurve(c, domain, req)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		response.Success(c, stats)
	}
}
