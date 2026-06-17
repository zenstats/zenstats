package stats

import (
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	querystats "github.com/zenstats/zenstats/internal/service/stats"
	"github.com/zenstats/zenstats/pkg/response"
)

// ExportBreakdown 导出维度细分数据为 CSV 文件
//
//	@Summary		导出维度细分数据 CSV
//	@Description	按指定维度导出细分统计数据为 CSV 文件，支持所有 breakdown 相同参数
//	@Tags			统计分析
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		text/csv
//	@Param			domain		path		string	true	"站点域名"
//	@Param			period		query		string	true	"时间周期"
//	@Param			date		query		string	false	"统计日期"
//	@Param			from		query		string	false	"自定义开始日期"
//	@Param			to			query		string	false	"自定义结束日期"
//	@Param			property	query		string	true	"细分维度"
//	@Param			metrics		query		string	false	"指标列表，逗号分隔"
//	@Param			filters		query		string	false	"过滤条件 JSON"
//	@Param			limit		query		int		false	"最大导出条数"	default(10000)
//	@Success		200			{file}		binary	"CSV 文件"
//	@Failure		400			{object}	response.ErrorResponse
//	@Failure		401			{object}	response.ErrorResponse
//	@Failure		500			{object}	response.ErrorResponse
//	@Router			/stats/{domain}/export [get]
func (s *StatsHandle) ExportBreakdown() gin.HandlerFunc {
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

		// 导出上限，默认 10000 条
		limit := 10000
		if l := c.Query("limit"); l != "" {
			if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100000 {
				limit = parsed
			}
		}
		req.Limit = limit

		// 复用 breakdown 逻辑获取全量数据
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

		// 超过上限截断
		if limit > 0 && len(result.Data) > limit {
			result.Data = result.Data[:limit]
		}

		// 设置 CSV 响应头
		domain := c.Param("domain")
		filename := fmt.Sprintf("%s_%s_%s.csv", domain, property, req.Date)
		if req.Period == "custom" {
			filename = fmt.Sprintf("%s_%s_%s_%s.csv", domain, property, req.From, req.To)
		}
		c.Header("Content-Type", "text/csv; charset=utf-8")
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

		// 写入 UTF-8 BOM
		c.Writer.Write([]byte{0xEF, 0xBB, 0xBF})

		writer := csv.NewWriter(c.Writer)
		defer writer.Flush()

		// 写入表头
		if err := writer.Write(result.Columns); err != nil {
			return
		}

		// 写入数据行
		for _, row := range result.Data {
			record := make([]string, len(result.Columns))
			for i, col := range result.Columns {
				val := row[col]
				switch v := val.(type) {
				case float64:
					record[i] = strconv.FormatFloat(v, 'f', -1, 64)
				case int64:
					record[i] = strconv.FormatInt(v, 10)
				case int:
					record[i] = strconv.Itoa(v)
				default:
					record[i] = fmt.Sprintf("%v", v)
				}
			}
			if err := writer.Write(record); err != nil {
				return
			}
		}
	}
}
