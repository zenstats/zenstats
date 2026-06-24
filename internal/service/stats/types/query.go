package types

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// Query 表示一个完整的统计查询，包含站点、时间范围、指标、维度、过滤器等所有查询参数。
type Query struct {
	SiteID                 string         `json:"site_id"`                              // 站点ID
	UserID                 int64          `json:"user_id"`                              // 用户ID
	UTCTimeRange           TimeRange      `json:"utc_time_range"`                       // UTC时间范围
	ComparisonUTCTimeRange *TimeRange     `json:"comparison_utc_time_range,omitempty"`  // 对比时间范围
	Interval               string         `json:"interval,omitempty"`                   // 时间间隔
	Period                 string         `json:"period,omitempty"`                     // 周期类型
	Date                   string         `json:"date,omitempty"`                       // 日期
	From                   string         `json:"from,omitempty"`                       // 开始时间
	To                     string         `json:"to,omitempty"`                         // 结束时间
	Timezone               string         `json:"timezone,omitempty"`                   // 时区
	Dimensions             []string       `json:"dimensions,omitempty"`                 // 维度列表 如["visit:country", "event:page"]
	Metrics                []string       `json:"metrics,omitempty"`                    // 查询的指标列表，如["visitors", "pageviews", "bounce_rate"]
	Filters                []*Filter      `json:"filters,omitempty"`                    // 过滤器列表
	TimeOnPageData         TimeOnPageData `json:"time_on_page_data,omitempty"`          // 页面停留时间配置
	SampleThreshold        int64          `json:"sample_threshold,omitempty"`           // 采样阈值
	Now                    time.Time      `json:"now,omitempty"`                        // 当前时间
	SiteNativeStatsStartAt time.Time      `json:"site_native_stats_start_at,omitempty"` // 站点本地统计开始时间
	Pagination             *Pagination    `json:"pagination,omitempty"`                 // 分页配置
	OrderBy                []*OrderBy     `json:"order_by,omitempty"`                   // 排序配置

	// query pipeline and are intentionally omitted from the public API contract.
	SQLJoinType          string    `json:"-"`
	SmearSessionMetrics  bool      `json:"-"`
	DropTimeOnPageMetric bool      `json:"-"`
	InputUTCTimeRange    TimeRange `json:"-"`
}

// Validate 验证查询参数合法性
func (q *Query) Validate() error {
	if q.SiteID == "" {
		return errors.New("site ID is required")
	}
	if q.UTCTimeRange.Start.After(q.UTCTimeRange.End) {
		return errors.New("start time must be before end time")
	}
	if len(q.Metrics) == 0 {
		return errors.New("at least one metric is required")
	}

	// 验证维度格式
	for _, dim := range q.Dimensions {
		if !strings.Contains(dim, ":") {
			return errors.New("invalid dimension format: " + dim + ", expected format like 'event:page'")
		}
	}

	// 验证指标名称
	for _, m := range q.Metrics {
		if m == "" {
			return errors.New("metric name cannot be empty")
		}
	}

	// 验证排序方向和维度存在性
	for _, ob := range q.OrderBy {
		if ob.Direction != "asc" && ob.Direction != "desc" {
			return errors.New("invalid order direction: " + ob.Direction + ", must be 'asc' or 'desc'")
		}

		// 检查排序维度是否存在于查询维度或指标中
		dimExists := false
		for _, dim := range q.Dimensions {
			if dim == ob.Dimension {
				dimExists = true
				break
			}
		}
		if !dimExists {
			for _, m := range q.Metrics {
				if m == ob.Dimension {
					dimExists = true
					break
				}
			}
		}
		if !dimExists {
			return errors.New("order by dimension not found in query dimensions or metrics: " + ob.Dimension)
		}
	}

	// 检查重复指标
	metricNames := make(map[string]bool)
	for _, m := range q.Metrics {
		if metricNames[m] {
			return errors.New("duplicate metric: " + m)
		}
		metricNames[m] = true
	}

	// 检查重复维度
	dimNames := make(map[string]bool)
	for _, dim := range q.Dimensions {
		if dimNames[dim] {
			return errors.New("duplicate dimension: " + dim)
		}
		dimNames[dim] = true
	}

	// 验证采样阈值非负
	if q.SampleThreshold < 0 {
		return errors.New("sample threshold cannot be negative")
	}

	// 验证时间参数不冲突
	hasFromTo := q.From != "" && q.To != ""
	hasPeriod := q.Period != ""
	hasDate := q.Date != ""

	if hasFromTo && (hasPeriod || hasDate) {
		return errors.New("cannot specify both From/To and Period/Date")
	}
	if hasPeriod && hasDate {
		return errors.New("cannot specify both Period and Date")
	}

	// 验证日期格式
	if hasDate {
		// 检查日期是否为午夜时间点
		parsedDate, err := time.Parse("2006-01-02", q.Date)
		if err != nil {
			return errors.New("invalid date format, expected '2006-01-02'")
		}
		if parsedDate.Hour() != 0 || parsedDate.Minute() != 0 || parsedDate.Second() != 0 || parsedDate.Nanosecond() != 0 {
			return errors.New("date must be at midnight")
		}
	}

	// 验证对比时间范围
	if q.ComparisonUTCTimeRange != nil {
		if q.ComparisonUTCTimeRange.Start.After(q.ComparisonUTCTimeRange.End) {
			return errors.New("comparison start time must be before end time")
		}
		if q.ComparisonUTCTimeRange.Start.IsZero() || q.ComparisonUTCTimeRange.End.IsZero() {
			return errors.New("comparison time range start and end cannot be zero")
		}
	}

	// 验证时区格式
	if q.Timezone == "" {
		return errors.New("timezone must be specified")
	}
	if _, err := time.LoadLocation(q.Timezone); err != nil {
		return errors.New("invalid timezone: " + q.Timezone)
	}

	// 验证指标依赖关系
	metricSet := make(map[string]bool)
	for _, m := range q.Metrics {
		metricSet[m] = true
	}

	// bounce_rate requires visitors metric
	if metricSet["bounce_rate"] && !metricSet["visitors"] {
		return errors.New("bounce_rate metric requires visitors metric")
	}

	// conversion_rate requires conversions and visitors metrics
	if metricSet["conversion_rate"] && (!metricSet["conversions"] || !metricSet["visitors"]) {
		return errors.New("conversion_rate metric requires conversions and visitors metrics")
	}

	// average_revenue requires conversions metric
	if metricSet["average_revenue"] && !metricSet["conversions"] {
		return errors.New("average_revenue metric requires conversions metric")
	}

	// 验证指标-维度依赖关系
	containsDimension := func(dimensions []string, target string) bool {
		for _, d := range dimensions {
			if d == target {
				return true
			}
		}
		return false
	}

	if metricSet["scroll_depth"] && !containsDimension(q.Dimensions, "page") {
		return errors.New("scroll_depth metric requires page dimension")
	}

	if metricSet["time_on_page"] && !containsDimension(q.Dimensions, "page") {
		return errors.New("time_on_page metric requires page dimension")
	}

	// 验证TimeOnPageData依赖
	if q.TimeOnPageData.NewMetricVisible {
		hasTimeOnPageMetric := false
		for _, m := range q.Metrics {
			if m == "time_on_page" {
				hasTimeOnPageMetric = true
				break
			}
		}
		if !hasTimeOnPageMetric {
			return errors.New("time_on_page metric is required when TimeOnPageData.NewMetricVisible is true")
		}
	}

	return nil
}

// Metric 表示一个统计指标，包含名称、描述和 SQL 表达式。
type Metric struct {
	Name        string
	Description string
	SQLExpr     string
	Valid       bool
}

// ComparisonConfig 对比查询配置，定义对比模式和周期。
type ComparisonConfig struct {
	Mode   string // 对比模式，如"previous_period", "same_period_last_year"
	Period string // 对比周期类型
}

// TimeOnPageData 页面停留时间数据配置。
type TimeOnPageData struct {
	CutoffDate       time.Time
	NewMetricVisible bool
	IncludeNewMetric bool
	Cutoff           time.Time
}

// Site 表示查询上下文中的站点信息。
type Site struct {
	ID                 string    `json:"id"`
	UserID             int64     `json:"user_id"`
	Timezone           string    `json:"timezone"`
	NativeStatsStartAt time.Time `json:"native_stats_start_at"`
}

// Pagination 分页查询配置。
type Pagination struct {
	Limit  int
	Offset int
}

// QueryResult 封装查询结果
type QueryResult struct {
	Columns   []string         `json:"columns"`
	Data      []map[string]any `json:"data"`
	TotalRows int              `json:"total_rows,omitempty"`
}

// ToJSON 将结果转换为JSON
func (qr *QueryResult) ToJSON() ([]byte, error) {
	return json.Marshal(qr)
}

// OrderBy 排序配置，指定排序维度和方向（asc/desc）。
type OrderBy struct {
	Dimension string
	Direction string // "asc" or "desc"
}
