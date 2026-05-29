package stats

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/pkg/response"
)

// GetBreakdown 获取维度细分数据（对标 Plausible /api/v1/stats/breakdown）
//
//	@Summary		获取维度细分数据
//	@Description	按指定维度获取细分统计数据，支持 visit:source, visit:country, visit:browser, visit:os, visit:device, visit:entry_page, visit:exit_page, event:page 等维度
//	@Tags			统计分析
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			domain		path		string	true	"站点域名"
//	@Param			period		query		string	true	"时间周期 (day|p7|p14|p30|custom)"	Enums(day, p7, p14, p30, custom)
//	@Param			date		query		string	false	"日期 (YYYY-MM-DD)"
//	@Param			from		query		string	false	"自定义开始日期"
//	@Param			to			query		string	false	"自定义结束日期"
//	@Param			property	query		string	true	"细分维度"	Enums(visit:source, visit:country, visit:region, visit:city, visit:browser, visit:os, visit:device, visit:screen_size, visit:entry_page, visit:exit_page, event:page, event:name)
//	@Param			metrics		query		string	false	"指标列表，逗号分隔"	default(visitors)
//	@Param			limit		query		int		false	"返回条数限制"	default(9)
//	@Param			page		query		int		false	"页码"	default(1)
//	@Param			filters		query		string	false	"过滤条件 (JSON)"
//	@Success		200			{object}	response.SuccessResponse{data=any}	"成功响应"
//	@Failure		400			{object}	response.ErrorResponse	"请求参数错误"
//	@Router			/stats/{domain}/breakdown [get]
func (s *StatsHandle) GetBreakdown() gin.HandlerFunc {
	return func(c *gin.Context) {
		domain := c.Param("domain")

		req, err := s.validate(c)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		property := c.Query("property")
		if property == "" {
			response.Error(c, http.StatusBadRequest, nil)
			return
		}

		metrics := c.Query("metrics")
		if metrics == "" {
			metrics = "visitors"
		}

		result, err := s.statsService.GetBreakdown(c, domain, req, property, metrics)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		response.Success(c, result)
	}
}
