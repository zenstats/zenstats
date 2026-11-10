package stats

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/service"
	"github.com/zenstats/zenstats/pkg/response"
)

// GetUserFlows 获取用户行为流向（Sankey 图数据）
//
//	@Summary		获取用户流向（Sankey）
//	@Description	按会话统计所选时间范围内首次观察到的页面路径；连续重复页合并，未观察到后续页面不代表真实离站
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
//	@Param			entry_path	query		string	false	"限定时间范围内首次观察到的页面（如 / 或 /pricing）"
//	@Param			filters	query		string	false	"标准统计筛选JSON，选出匹配会话后保留其范围内的完整路径"
//	@Param			steps		query		int		false	"分析步数 (2 ~ 4，默认 3)"
//	@Param			limit		query		int		false	"每步热门路径数量 (3 ~ 12，默认 6)"
//	@Success		200			{object}	response.SuccessResponse{data=service.UserFlowsResponse}	"成功响应，data 为包含 nodes 和 links 的桑基图数据"
//	@Failure		400			{object}	response.ErrorResponse	"请求参数错误"
//	@Failure		401			{object}	response.ErrorResponse	"未认证或认证失败"
//	@Failure		500			{object}	response.ErrorResponse	"服务器内部错误"
//	@Router			/stats/{domain}/user-flows [get]
func (s *StatsHandle) GetUserFlows() gin.HandlerFunc {
	return func(c *gin.Context) {
		siteID := c.GetInt64("site_id")

		req, err := s.validate(c)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		entryPath := strings.TrimSpace(c.Query("entry_path"))
		if len(entryPath) > 2000 || (entryPath != "" && !strings.HasPrefix(entryPath, "/")) {
			response.Error(c, http.StatusBadRequest, fmt.Errorf("entry_path must be a path starting with /, at most 2000 bytes"))
			return
		}

		steps := 3
		if sStr := c.Query("steps"); sStr != "" {
			v, err := strconv.Atoi(sStr)
			if err != nil || v < 2 || v > 4 {
				response.Error(c, http.StatusBadRequest, fmt.Errorf("steps must be between 2 and 4"))
				return
			}
			steps = v
		}

		limit := 6
		if lStr := c.Query("limit"); lStr != "" {
			v, err := strconv.Atoi(lStr)
			if err != nil || v < 3 || v > 12 {
				response.Error(c, http.StatusBadRequest, fmt.Errorf("limit must be between 3 and 12"))
				return
			}
			limit = v
		}

		flowParams := service.UserFlowsParams{
			EntryPath: entryPath,
			Steps:     steps,
			Limit:     limit,
		}

		result, err := s.statsService.GetUserFlows(c, siteID, req, flowParams)
		if err != nil {
			respondQueryError(c, err)
			return
		}

		response.Success(c, result)
	}
}
