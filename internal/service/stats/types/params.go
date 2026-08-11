package types

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/dromara/carbon/v2"
)

// parsePeriodShorthand 解析 "Nd"/"Nmo" 相对区间简写，返回 (数量, 单位, 是否匹配)。
// 单位为 "d"（天）或 "mo"（月）。仅接受正整数。
func parsePeriodShorthand(period string) (int, string, bool) {
	var numStr, unit string
	switch {
	case strings.HasSuffix(period, "mo"):
		numStr, unit = strings.TrimSuffix(period, "mo"), "mo"
	case strings.HasSuffix(period, "d"):
		numStr, unit = strings.TrimSuffix(period, "d"), "d"
	default:
		return 0, "", false
	}
	n, err := strconv.Atoi(numStr)
	if err != nil || n < 1 {
		return 0, "", false
	}
	return n, unit, true
}

// Params 统计查询参数，包含站点信息、时间范围、指标、维度、过滤器等配置。
type Params struct {
	SiteID                 string      `json:"site_id"`                             // 站点ID
	UserID                 int64       `json:"user_id"`                             // 用户ID
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
	case "w":
		// 遗留别名：最近 7 天（此前通过校验但未实现，会触发 default 报错）
		localEndDate := carbon.Parse(p.Date, siteTimezone)
		if localEndDate.Error != nil {
			return localEndDate.Error
		}
		start = localEndDate.SubDays(6).StartOfDay()
		end = localEndDate.EndOfDay()
	case "month", "m":
		// 日期所在的自然月（1 号到月末），与 Plausible `month` 对齐
		localDate := carbon.Parse(p.Date, siteTimezone)
		if localDate.Error != nil {
			return localDate.Error
		}
		start = localDate.StartOfMonth()
		end = localDate.EndOfMonth()
	case "6mo":
		// 注意：carbon v2 的 AddMonths/SubMonths 会原地修改接收者，必须先 Copy()，
		// 否则计算 start 时会污染 localEndDate，导致 end 落到错误日期（区间塌缩）。
		localEndDate := carbon.Parse(p.Date, siteTimezone)
		if localEndDate.Error != nil {
			return localEndDate.Error
		}
		start = localEndDate.Copy().SubMonths(5).StartOfMonth()
		end = localEndDate.EndOfDay()
	case "12mo":
		localEndDate := carbon.Parse(p.Date, siteTimezone)
		if localEndDate.Error != nil {
			return localEndDate.Error
		}
		start = localEndDate.Copy().SubMonths(11).StartOfMonth()
		end = localEndDate.EndOfDay()
	case "year":
		// 日期所在自然年（1/1 到 12/31），与 Plausible `year` 对齐
		localDate := carbon.Parse(p.Date, siteTimezone)
		if localDate.Error != nil {
			return localDate.Error
		}
		start = localDate.StartOfYear()
		end = localDate.EndOfYear()
	case "all":
		// 全部数据：Params 不携带站点起始日期，退化为最近 5 年的宽窗口作为近似。
		// Copy() 避免 SubYears 原地修改 now 而污染 end。
		start = now.Copy().SubYears(5).StartOfDay()
		end = now.EndOfDay()
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
		// Plausible 风格相对区间简写：Nd（最近 N 天）、Nmo（最近 N 个月）
		if n, unit, ok := parsePeriodShorthand(p.Period); ok {
			localEndDate := carbon.Parse(p.Date, siteTimezone)
			if localEndDate.Error != nil {
				return localEndDate.Error
			}
			if unit == "mo" {
				start = localEndDate.Copy().SubMonths(n - 1).StartOfMonth()
			} else {
				start = localEndDate.Copy().SubDays(n - 1).StartOfDay()
			}
			end = localEndDate.EndOfDay()
		} else {
			return fmt.Errorf("unsupported period: %s", p.Period)
		}
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
