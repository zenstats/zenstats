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
	if req.Period == "yesterday" && req.Date == "" {
		req.Date = time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	} else if req.Period != "custom" && req.Period != "realtime" && req.Date == "" {
		req.Date = time.Now().Format("2006-01-02")
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

// GetAggregate 获取聚合统计指标。
//
//	@Summary		获取聚合统计指标
//	@Description	获取指定域名在给定时间范围内的总览指标和对比数据。常用指标包括 visitors、pageviews、visits、bounce_rate、visit_duration、events。
//	@Tags			统计分析
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Accept			json
//	@Produce		json
//	@Param			domain		path		string	true	"站点域名，例如 example.com"
//	@Param			period		query		string	true	"时间周期" Enums(realtime, day, p7, p14, p30, custom)
//	@Param			date		query		string	false	"统计日期，格式 YYYY-MM-DD；非 custom/realtime 周期未传时默认今天"
//	@Param			from		query		string	false	"自定义开始日期，period=custom 时必填，格式 YYYY-MM-DD"
//	@Param			to			query		string	false	"自定义结束日期，period=custom 时必填，格式 YYYY-MM-DD"
//	@Param			metrics		query		string	false	"指标列表，逗号分隔；支持 visitors,pageviews,visits,bounce_rate,visit_duration,events" default(visitors,pageviews,visits,bounce_rate,visit_duration)
//	@Param			filters		query		string	false	"过滤条件 JSON 字符串，例如 [[\"is\",\"visit:country\",[\"CN\"]]]"
//	@Success		200			{object}	response.SuccessResponse{data=any}	"成功响应，data 为聚合指标对象"
//	@Failure		400			{object}	response.ErrorResponse	"请求参数错误"
//	@Failure		401			{object}	response.ErrorResponse	"未认证或认证失败"
//	@Failure		500			{object}	response.ErrorResponse	"服务器内部错误"
//	@Router			/stats/{domain}/aggregate [get]
func (s *StatsHandle) GetAggregate() gin.HandlerFunc {
	return func(c *gin.Context) {
		siteID := c.GetInt64("site_id")
		req, err := s.validate(c)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		stats, err := s.statsService.GetAggregate(c, siteID, req)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		response.Success(c, stats)
	}
}

// GetTimeSeries 获取时间序列统计。
//
//	@Summary		获取时间序列统计
//	@Description	获取指定域名按时间间隔聚合的统计数据。该接口是 main-graph 的兼容别名。
//	@Tags			统计分析
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Accept			json
//	@Produce		json
//	@Param			domain		path		string	true	"站点域名，例如 example.com"
//	@Param			period		query		string	true	"时间周期" Enums(realtime, day, p7, p14, p30, custom)
//	@Param			date		query		string	false	"统计日期，格式 YYYY-MM-DD"
//	@Param			from		query		string	false	"自定义开始日期，period=custom 时必填，格式 YYYY-MM-DD"
//	@Param			to			query		string	false	"自定义结束日期，period=custom 时必填，格式 YYYY-MM-DD"
//	@Param			interval	query		string	false	"时间间隔" Enums(minute, hourly, daily, weekly, monthly, yearly)
//	@Param			metrics		query		string	false	"指标列表，逗号分隔" default(visitors,pageviews)
//	@Param			filters		query		string	false	"过滤条件 JSON 字符串"
//	@Success		200			{object}	response.SuccessResponse{data=any}	"成功响应，data 为时间序列数组"
//	@Failure		400			{object}	response.ErrorResponse	"请求参数错误"
//	@Failure		401			{object}	response.ErrorResponse	"未认证或认证失败"
//	@Failure		500			{object}	response.ErrorResponse	"服务器内部错误"
//	@Router			/stats/{domain}/time_series [get]
func (s *StatsHandle) GetTimeSeries() gin.HandlerFunc {
	return func(c *gin.Context) {
		siteID := c.GetInt64("site_id")

		req, err := s.validate(c)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		stats, err := s.statsService.GetTimeSeries(c, siteID, req)
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
