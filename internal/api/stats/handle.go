// Package stats 处理统计分析相关的 HTTP 请求，包括聚合统计、时间序列、维度细分等。
package stats

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/api/types"
	"github.com/zenstats/zenstats/internal/service"
	"github.com/zenstats/zenstats/pkg/response"
)

// StatsHandle 统计分析处理器，基于新版查询引擎处理各类统计查询请求。
type StatsHandle struct {
	statsService *service.StatsService
}

// NewStatsHandle 创建并返回一个新的 StatsHandle 实例。
func NewStatsHandle() *StatsHandle {
	return &StatsHandle{
		statsService: service.GetStatsService(),
	}
}

// validate 从查询参数中解析并验证统计请求参数。
// 校验时间周期、日期格式等，返回解析后的 StatsRequest 或错误。
func (s *StatsHandle) validate(c *gin.Context) (*types.StatsRequest, error) {
	var req types.StatsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		return nil, err
	}
	if req.Period == "custom" && (req.From == "" || req.To == "") {
		return nil, fmt.Errorf("start_date and end_date must be provided")
	}
	if req.Period != "custom" && req.Period != "realtime" && req.Date == "" {
		return nil, fmt.Errorf("date must be provided")
	}

	if req.Date != "" && !s.dateIsValid(req.Date) {
		return nil, fmt.Errorf("date format must be valid")
	}
	if req.From != "" && !s.dateIsValid(req.From) {
		return nil, fmt.Errorf("from date format must be valid")
	}
	if req.To != "" && !s.dateIsValid(req.To) {
		return nil, fmt.Errorf("to date format must be valid")
	}
	return &req, nil
}

// GetAggregate 获取来源聚合统计
//
//	@Summary		获取来源聚合统计
//	@Description	获取指定域名的来源聚合统计数据
//	@Tags			统计分析
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			domain	path		string	true	"站点域名"
//	@Success		200		{object}	response.SuccessResponse{data=any}	"成功响应，返回来源聚合统计数据"
//	@Failure		400		{object}	response.ErrorResponse	"请求参数错误"
//	@Router			/stats/{domain}/aggregate [get]
func (s *StatsHandle) GetAggregate() gin.HandlerFunc {
	return func(c *gin.Context) {
		domain := c.Param("domain")
		req, err := s.validate(c)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		stats, err := s.statsService.GetAggregate(c, domain, req)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		response.Success(c, stats)
	}
}

// GetTimeSeries 获取来源时间序列统计
//
//	@Summary		获取来源时间序列统计
//	@Description	获取指定域名的来源时间序列统计数据
//	@Tags			统计分析
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			domain	path		string	true	"站点域名"
//	@Success		200		{object}	response.SuccessResponse{data=any}	"成功响应，返回来源时间序列统计数据"
//	@Failure		400		{object}	response.ErrorResponse	"请求参数错误"
//	@Router			/stats/{domain}/time_series [get]
func (s *StatsHandle) GetTimeSeries() gin.HandlerFunc {
	return func(c *gin.Context) {
		domain := c.Param("domain")

		req, err := s.validate(c)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		stats, err := s.statsService.GetTimeSeries(c, domain, req)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		response.Success(c, stats)
	}
}

// dateIsValid 验证日期字符串是否符合 YYYY-MM-DD 格式。
func (s *StatsHandle) dateIsValid(dateStr string) bool {
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return false
	}

	return t.Format("2006-01-02") == dateStr
}
