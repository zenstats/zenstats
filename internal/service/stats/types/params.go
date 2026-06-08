package types

import (
	"fmt"

	"github.com/dromara/carbon/v2"
)

// Params 统计查询参数，包含站点信息、时间范围、指标、维度、过滤器等配置。
type Params struct {
	SiteID                 string      `json:"site_id"`                             // 站点ID
	UTCTimeRange           TimeRange   `json:"utc_time_range"`                      // UTC时间范围
	ComparisonUTCTimeRange *TimeRange  `json:"comparison_utc_time_range,omitempty"` // 对比时间范围
	Interval               string      `json:"interval,omitempty"`                  // 时间间隔
	Period                 string      `json:"period,omitempty"`                    // 周期类型
	Date                   string      `json:"date,omitempty"`                      // 日期
	From                   string      `json:"from,omitempty"`                      // 开始时间
	To                     string      `json:"to,omitempty"`                        // 结束时间
	Timezone               string      `json:"timezone,omitempty"`                  // 时区
	Property               string      `json:"property,omitempty"`                  // 属性ID
	Dimensions             []string    `json:"dimensions,omitempty"`                // 维度列表
	Metrics                []string    `json:"metrics,omitempty"`                   // 查询的指标列表
	Filters                []*Filter   `json:"filters,omitempty"`                   // 过滤器列表
	Pagination             *Pagination `json:"pagination,omitempty"`                // 分页配置
	OrderBy                []*OrderBy  `json:"order_by,omitempty"`                  // 排序配置
	SampleThreshold        int64       `json:"sample_threshold,omitempty"`          // 采样阈值，0 表示不采样
}

// ParsePeriodToUTCTimeRange 根据Period参数解析UTC时间范围
func (p *Params) ParsePeriodToUTCTimeRange(siteTimezone string) error {
	var start, end *carbon.Carbon
	now := carbon.Now(siteTimezone)

	switch p.Period {
	case "realtime":
		start = now.SubMinutes(30)
		end = now
	case "day", "yesterday":
		// 解析日期并转换为UTC
		localDate := carbon.Parse(p.Date, siteTimezone)
		if localDate.Error != nil {
			return localDate.Error
		}
		start = localDate.StartOfDay()
		end = localDate.EndOfDay()
	case "p7":
		localEndDate := carbon.Parse(p.Date, siteTimezone)
		if localEndDate.Error != nil {
			return localEndDate.Error
		}
		start = localEndDate.SubDays(6).StartOfDay()
		end = localEndDate.EndOfDay()
	case "p14":
		localEndDate := carbon.Parse(p.Date, siteTimezone)
		if localEndDate.Error != nil {
			return localEndDate.Error
		}
		start = localEndDate.SubDays(13).StartOfDay()
		end = localEndDate.EndOfDay()
	case "p30":
		localEndDate := carbon.Parse(p.Date, siteTimezone)
		if localEndDate.Error != nil {
			return localEndDate.Error
		}
		start = localEndDate.SubDays(29).StartOfDay()
		end = localEndDate.EndOfDay()
	case "custom":
		start = carbon.Parse(p.From, siteTimezone)
		end = carbon.Parse(p.To, siteTimezone)
		if start.Error != nil {
			return start.Error
		}
		if end.Error != nil {
			return end.Error
		}
	default:
		return fmt.Errorf("unsupported period: %s", p.Period)
	}

	p.UTCTimeRange = TimeRange{
		Start: start.SetTimezone(carbon.UTC).StdTime(),
		End:   end.SetTimezone(carbon.UTC).StdTime(),
	}
	return nil
}

// {
//   "date_range": ["2023-10-01", "2023-10-31"],
//   "metrics": ["visitors", "pageviews"],
//   "filters": [["is", "visit:browser", ["Chrome"]]],
//   "dimensions": ["event:page"],
//   "order_by": [{"visitors", "desc"}],
//   "include": {"comparisons": "previous_period"},
//   "pagination": {"limit": 10, "offset": 0}
// }
