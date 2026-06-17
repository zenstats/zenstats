package stats

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/dromara/carbon/v2"
	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/service"
	"github.com/zenstats/zenstats/internal/service/stats"
	"github.com/zenstats/zenstats/pkg/response"
)

// GetSuggestions 获取筛选器建议值，用于前端自动补全。
//
//	@Summary		获取筛选器建议
//	@Description	根据 filter_name 返回可选值列表，支持模糊搜索。
//	@Tags			统计分析
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		json
//	@Param			domain			path		string	true	"站点域名"
//	@Param			filter_name		query		string	true	"过滤器名称: prop_key 获取所有属性键; event:props:xxx 获取指定属性值"
//	@Param			q				query		string	false	"搜索关键词（模糊匹配）"
//	@Param			period			query		string	false	"时间周期" default(p30)
//	@Param			date			query		string	false	"统计日期，格式 YYYY-MM-DD"
//	@Param			from			query		string	false	"自定义开始日期"
//	@Param			to				query		string	false	"自定义结束日期"
//	@Success		200				{object}	response.SuccessResponse{data=[]stats.SuggestionItem}
//	@Failure		400				{object}	response.ErrorResponse
//	@Failure		401				{object}	response.ErrorResponse
//	@Failure		500				{object}	response.ErrorResponse
//	@Router			/stats/{domain}/suggestions [get]
func (s *StatsHandle) GetSuggestions() gin.HandlerFunc {
	return func(c *gin.Context) {
		siteID := c.GetInt64("site_id")

		filterName := c.Query("filter_name")
		if filterName == "" {
			response.Error(c, http.StatusBadRequest, fmt.Errorf("filter_name is required"))
			return
		}

		query := c.Query("q")

		// 计算时间范围
		siteService := service.GetSiteService()
		site, err := siteService.GetSiteByID(c, int(siteID))
		if err != nil {
			response.Error(c, http.StatusNotFound, err)
			return
		}

		timezone := site.Timezone
		if timezone == "" {
			timezone = "UTC"
		}

		period := c.DefaultQuery("period", "p30")
		date := c.DefaultQuery("date", "")
		from := c.Query("from")
		to := c.Query("to")

		start, end := parseSuggestionsTimeRange(period, date, from, to, timezone)

		suggestionService := stats.NewSuggestionService()
		startStr := start.StdTime().Format("2006-01-02 15:04:05")
		endStr := end.StdTime().Format("2006-01-02 15:04:05")
		siteIDStr := fmt.Sprintf("%d", siteID)

		if filterName == "prop_key" {
			items, err := suggestionService.GetPropKeys(c, siteIDStr, startStr, endStr, query)
			if err != nil {
				response.Error(c, http.StatusInternalServerError, err)
				return
			}
			response.Success(c, items)
			return
		}

		// event:props:xxx → 获取指定属性的值建议
		if strings.HasPrefix(filterName, "event:props:") {
			propKey := strings.TrimPrefix(filterName, "event:props:")
			items, err := suggestionService.GetPropValues(c, siteIDStr, startStr, endStr, propKey, query)
			if err != nil {
				response.Error(c, http.StatusInternalServerError, err)
				return
			}
			response.Success(c, items)
			return
		}

		response.Error(c, http.StatusBadRequest, fmt.Errorf("unsupported filter_name: %s", filterName))
	}
}

// parseSuggestionsTimeRange 解析建议查询的时间范围。
func parseSuggestionsTimeRange(period, date, from, to, timezone string) (start, end *carbon.Carbon) {
	switch period {
	case "day", "yesterday":
		if date == "" && period == "yesterday" {
			date = carbon.Now(timezone).SubDays(1).Format("2006-01-02")
		} else if date == "" {
			date = carbon.Now(timezone).Format("2006-01-02")
		}
		d := carbon.Parse(date, timezone)
		start = d.StartOfDay()
		end = d.EndOfDay()
	case "p7":
		if date == "" {
			date = carbon.Now(timezone).Format("2006-01-02")
		}
		base := carbon.Parse(date, timezone)
		end = base.EndOfDay()
		start = base.SubDays(6).StartOfDay()
	case "p14":
		if date == "" {
			date = carbon.Now(timezone).Format("2006-01-02")
		}
		base := carbon.Parse(date, timezone)
		end = base.EndOfDay()
		start = base.SubDays(13).StartOfDay()
	default:
		// p30 或其他默认 30 天
		if date == "" {
			date = carbon.Now(timezone).Format("2006-01-02")
		}
		base := carbon.Parse(date, timezone)
		end = base.EndOfDay()
		start = base.SubDays(29).StartOfDay()
	case "custom":
		if from == "" || to == "" {
			// fallback to 30 days
			base := carbon.Now(timezone)
			end = base.EndOfDay()
			start = base.SubDays(29).StartOfDay()
		} else {
			start = carbon.Parse(from, timezone).StartOfDay()
			end = carbon.Parse(to, timezone).EndOfDay()
		}
	}
	return
}
