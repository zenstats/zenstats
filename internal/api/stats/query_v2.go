package stats

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	querystats "github.com/zenstats/zenstats/internal/service/stats"
	"github.com/zenstats/zenstats/pkg/response"
)

// QueryV2Request POST /api/v2/query 请求体结构。
type QueryV2Request struct {
	// 时间范围：支持 period 简写或 from/to 自定义
	Period    string `json:"period"`     // day | p7 | p14 | p30 | custom
	DateRange string `json:"date_range"` // period 别名，优先使用 period
	From      string `json:"from"`
	To        string `json:"to"`
	Date      string `json:"date"`

	// 指标与维度
	Metrics    []string `json:"metrics"`
	Dimensions []string `json:"dimensions"`

	// 过滤器：[["is","visit:country",["CN"]]] 或 JSON 字符串
	Filters json.RawMessage `json:"filters"`

	// 排序：[["visitors","desc"]]
	OrderBy [][]string `json:"order_by"`

	// 分页
	Limit  int `json:"limit"`
	Offset int `json:"offset"`

	// 采样
	SampleThreshold int64 `json:"sample_threshold"`
}

// QueryV2 POST /api/v2/query 通用查询入口，支持多维度、排序、offset 分页。
//
//	@Summary		通用查询（v2）
//	@Description	POST 方式提交查询，支持 metrics[]、dimensions[]、filters、order_by、limit/offset 全量参数。比 GET 接口更适合复杂查询和 SDK 集成。
//	@Tags			统计分析
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Accept			json
//	@Produce		json
//	@Param			domain	path		string			true	"站点域名"
//	@Param			body	body		QueryV2Request	true	"查询参数"
//	@Success		200		{object}	response.SuccessResponse{data=any}
//	@Failure		400		{object}	response.ErrorResponse
//	@Failure		401		{object}	response.ErrorResponse
//	@Failure		500		{object}	response.ErrorResponse
//	@Router			/api/v2/query/{domain} [post]
func (s *StatsHandle) QueryV2() gin.HandlerFunc {
	return func(c *gin.Context) {
		siteID := c.GetInt64("site_id")

		var req QueryV2Request
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		// date_range 是 period 的别名
		period := req.Period
		if period == "" {
			period = req.DateRange
		}
		if period == "" {
			period = "p30"
		}

		// 序列化 filters 回字符串交给现有管道
		filtersStr := ""
		if len(req.Filters) > 0 && string(req.Filters) != "null" {
			filtersStr = string(req.Filters)
		}

		// 解析 order_by：[["visitors","desc"]] → 排序字符串供服务层使用
		orderByStr := buildOrderByString(req.OrderBy)

		limit := req.Limit
		if limit <= 0 {
			limit = 100
		}

		metricsStr := strings.Join(req.Metrics, ",")
		dimensionsStr := ""
		if len(req.Dimensions) > 0 {
			dimensionsStr = req.Dimensions[0] // 当前支持单维度，多维度由维度列表依次查询
		}

		result, err := s.statsService.QueryV2(c, siteID, period, req.From, req.To, req.Date,
			metricsStr, dimensionsStr, filtersStr, orderByStr, limit, req.Offset, req.Dimensions, req.SampleThreshold)
		if err != nil {
			var validationErr *querystats.ValidationError
			if isValidationError(err, &validationErr) {
				response.Error(c, validationErr.StatusCode, err)
				return
			}
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		response.Success(c, result)
	}
}

func buildOrderByString(orderBy [][]string) string {
	if len(orderBy) == 0 {
		return ""
	}
	parts := make([]string, 0, len(orderBy))
	for _, ob := range orderBy {
		if len(ob) >= 2 {
			parts = append(parts, ob[0]+":"+ob[1])
		} else if len(ob) == 1 {
			parts = append(parts, ob[0]+":desc")
		}
	}
	return strings.Join(parts, ",")
}

func isValidationError(err error, target **querystats.ValidationError) bool {
	if ve, ok := err.(*querystats.ValidationError); ok {
		*target = ve
		return true
	}
	return false
}
