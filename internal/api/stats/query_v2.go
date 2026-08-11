package stats

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	querystats "github.com/zenstats/zenstats/internal/service/stats"
	"github.com/zenstats/zenstats/pkg/response"
)

// QueryV2Request is the POST /api/v2/query request body.
type QueryV2Request struct {
	// Time range: period shorthand or from/to custom
	Period    string `json:"period"`     // day | p7 | p14 | p30 | custom
	DateRange string `json:"date_range"` // period alias, period takes precedence
	From      string `json:"from"`
	To        string `json:"to"`
	Date      string `json:"date"`

	// Metrics and dimensions
	Metrics    []string `json:"metrics"`
	Dimensions []string `json:"dimensions"`

	// Filters: filter tuple array or JSON string
	Filters json.RawMessage `json:"filters"`

	// 排序：[["visitors","desc"]]
	OrderBy [][]string `json:"order_by"`

	// Pagination
	Limit  int `json:"limit"`
	Offset int `json:"offset"`

	// 采样
	SampleThreshold int64 `json:"sample_threshold"`

	// 对比（仅聚合查询）：previous_period | year_over_year | custom
	// custom 模式下用 compare_from/compare_to 指定对比区间
	Compare     string `json:"compare"`
	CompareFrom string `json:"compare_from"`
	CompareTo   string `json:"compare_to"`
}

// QueryV2 POST /api/v2/query 通用查询入口，支持多维度、排序、offset Pagination。
//
//	@Summary		通用查询（v2）
//	@Description	POST-based query with full parameter support. More suitable for complex queries and SDK integration.
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

		// date_range is an alias for period
		period := req.Period
		if period == "" {
			period = req.DateRange
		}
		if period == "" {
			period = "p30"
		}

		// 非 custom/realtime 且未传 date 时默认今天（与 v1 行为一致）
		if period != "custom" && period != "realtime" && req.Date == "" {
			req.Date = time.Now().Format("2006-01-02")
		}

		// Serialize filters back to string for existing pipeline
		filtersStr := ""
		if len(req.Filters) > 0 && string(req.Filters) != "null" {
			filtersStr = string(req.Filters)
		}

		// Parse order_by into sort string for service layer
		orderByStr := buildOrderByString(req.OrderBy)

		limit := req.Limit
		if limit <= 0 {
			limit = 100
		}

		metricsStr := strings.Join(req.Metrics, ",")
		// 多维细分由 req.Dimensions（全部维度）驱动，SQL 会对所有维度 SELECT + GROUP BY，
		// 结果保留各维度列。dimensionsStr 仅为兼容旧签名传入首个维度，不再限制维度数量。
		dimensionsStr := ""
		if len(req.Dimensions) > 0 {
			dimensionsStr = req.Dimensions[0]
		}

		result, err := s.statsService.QueryV2(c, siteID, period, req.From, req.To, req.Date,
			metricsStr, dimensionsStr, filtersStr, orderByStr, limit, req.Offset, req.Dimensions, req.SampleThreshold,
			req.Compare, req.CompareFrom, req.CompareTo)
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
	return errors.As(err, target)
}
