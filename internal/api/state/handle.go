package state

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/api/types"
	"github.com/zenstats/zenstats/internal/service"
)

type StateHandle struct {
	service *service.StateService
}

func NewStateHandle() *StateHandle {
	return &StateHandle{
		service: service.GetStateService(),
	}
}

func (s *StateHandle) validate(c *gin.Context) (*types.TopStatsRequest, error) {
	var req types.TopStatsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		return nil, err
	}
	if req.Period == "cr" && (req.StartDate == "" || req.EndDate == "") {
		return nil, fmt.Errorf("start_date and end_date must be provided")
	}
	if req.Period != "cr" && req.Period != "R" && req.Date == "" {
		return nil, fmt.Errorf("date must be provided")
	}

	if req.Date != "" && !s.dateIsValid(req.Date) {
		return nil, fmt.Errorf("date format must be valid")
	}
	if req.StartDate != "" && !s.dateIsValid(req.Date) {
		return nil, fmt.Errorf("date format must be valid")
	}
	if req.EndDate != "" && !s.dateIsValid(req.Date) {
		return nil, fmt.Errorf("date format must be valid")
	}
	return &req, nil
}

func (s *StateHandle) dateIsValid(dateStr string) bool {
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return false
	}

	return t.Format("2006-01-02") == dateStr
}
