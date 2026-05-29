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
//	@Accept			json
//	@Produce		json
//	@Param			domain		path		string	true	"站点域名"
//	@Param			period		query		string	true	"时间周期"	Enums(realtime, day, p7, p14, p30, custom)
//	@Param			date		query		string	false	"日期 (YYYY-MM-DD)"
//	@Param			from		query		string	false	"自定义开始日期"
//	@Param			to			query		string	false	"自定义结束日期"
//	@Param			interval	query		string	false	"时间间隔"	Enums(minute, hourly, daily, weekly, monthly)
//	@Param			metrics		query		string	false	"指标列表，逗号分隔"	default(visitors,pageviews)
//	@Param			filters		query		string	false	"过滤条件 (JSON)"
//	@Success		200			{object}	response.SuccessResponse{data=any}	"成功响应"
//	@Failure		400			{object}	response.ErrorResponse	"请求参数错误"
//	@Router			/stats/{domain}/main-graph [get]
func (s *StatsHandle) GetMainGraph() gin.HandlerFunc {
	return func(c *gin.Context) {
		domain := c.Param("domain")

		req, err := s.validate(c)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		metrics := c.Query("metrics")
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
