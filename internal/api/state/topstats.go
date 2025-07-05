package state

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/api/types"
	"github.com/zenstats/zenstats/pkg/response"
)

func (s *StateHandle) GetTopStats() gin.HandlerFunc {
	return func(c *gin.Context) {
		domain := c.Param("domain")

		// 获取请求参数
		var req types.TopStatsRequest
		if err := c.ShouldBindQuery(&req); err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}
		if req.Period == "cr" && (req.StartDate == "" || req.EndDate == "") {
			response.Error(c, http.StatusBadRequest, fmt.Errorf("start_date and end_date must be provided"))
			return
		}
		if req.Period != "cr" && req.Period != "R" && req.Date == "" {
			response.Error(c, http.StatusBadRequest, fmt.Errorf("date must be provided"))
			return
		}

		if req.Date != "" && !s.dateIsValid(req.Date) {
			response.Error(c, http.StatusBadRequest, fmt.Errorf("date format must be valid"))
			return
		}
		if req.StartDate != "" && !s.dateIsValid(req.Date) {
			response.Error(c, http.StatusBadRequest, fmt.Errorf("date format must be valid"))
			return
		}
		if req.EndDate != "" && !s.dateIsValid(req.Date) {
			response.Error(c, http.StatusBadRequest, fmt.Errorf("date format must be valid"))
			return
		}
		stats, err := s.service.GetTopStats(c, domain, &req)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		response.Success(c, stats)
	}
}

func (s *StateHandle) dateIsValid(dateStr string) bool {
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return false
	}

	return t.Format("2006-01-02") == dateStr
}
