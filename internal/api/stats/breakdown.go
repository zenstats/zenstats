package stats

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	querystats "github.com/zenstats/zenstats/internal/service/stats"
	"github.com/zenstats/zenstats/pkg/response"
)

// GetBreakdown 获取维度细分数据（对标 Plausible /api/v1/stats/breakdown）
//
//	@Summary		获取维度细分数据
//	@Description	按指定维度获取细分统计数据，支持 visit:source, visit:country, visit:browser, visit:os, visit:device, visit:entry_page, visit:exit_page, event:page 等维度
//	@Tags			统计分析
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Accept			json
//	@Produce		json
//	@Param			domain		path		string	true	"站点域名，例如 example.com"
//	@Param			period		query		string	true	"时间周期" Enums(day, p7, p14, p30, custom)
//	@Param			date		query		string	false	"统计日期，格式 YYYY-MM-DD"
//	@Param			from		query		string	false	"自定义开始日期，period=custom 时必填，格式 YYYY-MM-DD"
//	@Param			to			query		string	false	"自定义结束日期，period=custom 时必填，格式 YYYY-MM-DD"
//	@Param			property	query		string	true	"细分维度" Enums(visit:source, visit:country, visit:region, visit:city, visit:browser, visit:os, visit:device, visit:screen_size, visit:entry_page, visit:entry_page_hostname, visit:exit_page, visit:exit_page_hostname, event:page, event:name, event:hostname)
//	@Param			metrics		query		string	false	"指标列表，逗号分隔；event:name 等事件维度不支持 pageviews,bounce_rate,visit_duration,views_per_visit" default(visitors)
//	@Param			limit		query		int		false	"返回条数限制"	default(9)
//	@Param			page		query		int		false	"页码"	default(1)
//	@Param			filters		query		string	false	"过滤条件 JSON 字符串，例如 [[\"contains\",\"event:page\",[\"/docs\"]]]"
//	@Success		200			{object}	response.SuccessResponse{data=any}	"成功响应，data 为维度排行数组和分页信息"
//	@Failure		400			{object}	response.ErrorResponse	"请求参数错误"
//	@Failure		401			{object}	response.ErrorResponse	"未认证或认证失败"
//	@Failure		500			{object}	response.ErrorResponse	"服务器内部错误"
//	@Router			/stats/{domain}/breakdown [get]
func (s *StatsHandle) GetBreakdown() gin.HandlerFunc {
	return func(c *gin.Context) {
		siteID := c.GetInt64("site_id")

		req, err := s.validate(c)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		property := c.Query("property")
		if property == "" {
			response.Error(c, http.StatusBadRequest, errors.New("property is required"))
			return
		}

		metrics := c.Query("metrics")
		if metrics == "" {
			metrics = "visitors"
		}

		result, err := s.statsService.GetBreakdown(c, siteID, req, property, metrics)
		if err != nil {
			var validationErr *querystats.ValidationError
			if errors.As(err, &validationErr) {
				response.Error(c, validationErr.StatusCode, err)
				return
			}
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		response.Success(c, result)
	}
}
