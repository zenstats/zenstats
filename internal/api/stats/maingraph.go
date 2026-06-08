package stats

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/pkg/response"
)

// GetMainGraph 获取主图表时序数据（对标 Plausible /api/v1/stats/timeseries）
//
//	@Summary		获取主图表时序数据
//	@Description	获取指定域名的主图表时序统计数据，支持多个指标
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
//	@Param			metrics		query		string	false	"指标列表，逗号分隔；支持 visitors,pageviews,visits,bounce_rate,visit_duration,events" default(visitors,pageviews)
//	@Param			filters		query		string	false	"过滤条件 JSON 字符串，例如 [[\"is\",\"visit:browser\",[\"Chrome\"]]]"
//	@Success		200			{object}	response.SuccessResponse{data=any}	"成功响应，data 为按时间分组的指标数组"
//	@Failure		400			{object}	response.ErrorResponse	"请求参数错误"
//	@Failure		401			{object}	response.ErrorResponse	"未认证或认证失败"
//	@Failure		500			{object}	response.ErrorResponse	"服务器内部错误"
//	@Router			/stats/{domain}/main-graph [get]
func (s *StatsHandle) GetMainGraph() gin.HandlerFunc {
	return func(c *gin.Context) {
		domain := c.Param("domain")

		req, err := s.validate(c)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		metrics := req.Metrics
		if metrics == "" {
			metrics = "visitors,pageviews"
		}

		result, err := s.statsService.GetMainGraph(c, domain, req, metrics)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		response.Success(c, result)
	}
}
