package stats

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/zenstats/zenstats/internal/service/stats/types"
)

var metricsToRound = map[string]bool{
	"sample_percent": true,
}

// TimeSeriesPoint 时序数据点，包含时间戳和对应指标值。
type TimeSeriesPoint struct {
	Timestamp string         `json:"timestamp"`
	Metrics   map[string]any `json:"metrics"`
}

// AggregateResult 聚合统计结果，包含各指标的值、对比值和变化率。
type AggregateResult struct {
	Results map[string]MetricResult `json:"results"`
	Meta    any                     `json:"meta,omitempty"`
}

// MetricResult 单个指标的聚合结果，包含当前值、对比值和变化百分比。
type MetricResult struct {
	Value           any      `json:"value"`
	ComparisonValue *float64 `json:"comparison_value"`
	Change          *float64 `json:"change"`
}

// AggregateService 处理数据聚合
type AggregateService struct {
	qs *QueryService
}

// NewAggregateService 创建新的聚合服务
func NewAggregateService(qs *QueryService) *AggregateService {
	return &AggregateService{
		qs: qs,
	}
}

// GetAggregate 计算聚合指标（含对比数据）
func (as *AggregateService) GetAggregate(ctx context.Context, params *types.Params) (*AggregateResult, error) {
	// 处理 views_per_visit：替换为 visits 指标，事后在 Go 层计算
	needsViewsPerVisit := containsString(params.Metrics, "views_per_visit")
	queryMetrics := replaceViewsPerVisit(params.Metrics)

	// Handle conversion_rate: replace with conversions (numerator), query denominator (baseline visitors) separately, compute ratio in Go.
	// Since replacement happens before CreateQuery validation, we explicitly guard against missing goal filter:
	// Aggregate query (no dimensions) requires goal filter for conversion_rate, otherwise numerator=denominator = 100%.
	needsConversionRate := containsString(params.Metrics, "conversion_rate")
	if needsConversionRate && !types.HasEventGoalFilter(params.Filters) {
		return nil, &ValidationError{
			StatusCode: 400,
			Message:    "metric `conversion_rate` can only be queried with a goal filter",
		}
	}
	queryMetrics = replaceConversionRate(queryMetrics)

	// 创建无维度的查询参数
	aggParams := &types.Params{
		SiteID:       params.SiteID,
		Period:       params.Period,
		Date:         params.Date,
		From:         params.From,
		To:           params.To,
		Timezone:     params.Timezone,
		UTCTimeRange: params.UTCTimeRange,
		Metrics:      queryMetrics,
		Dimensions:   []string{}, // 无维度，只获取总计
		Filters:      params.Filters,
		Interval:     "",
	}
	query, err := as.qs.CreateQuery(aggParams)
	if err != nil {
		return nil, err
	}
	site := &types.Site{ID: query.SiteID, UserID: query.UserID, Timezone: query.Timezone}
	// 执行主查询
	result, err := as.qs.runner.RunQuery(ctx, query, site)
	if err != nil {
		return nil, fmt.Errorf("failed to run aggregate query: %w", err)
	}

	results := make(map[string]MetricResult)

	// 如果有对比时间范围，执行对比查询
	var comparisonData []map[string]any
	if params.ComparisonUTCTimeRange != nil {
		compParams := &types.Params{
			SiteID:       params.SiteID,
			Period:       params.Period,
			Date:         params.Date,
			From:         params.From,
			To:           params.To,
			Timezone:     params.Timezone,
			UTCTimeRange: *params.ComparisonUTCTimeRange,
			Metrics:      queryMetrics,
			Dimensions:   []string{},
			Filters:      params.Filters,
			Interval:     "",
		}
		compQuery, err := as.qs.CreateQuery(compParams)
		if err != nil {
			slog.Warn("failed to create comparison query", "error", err)
		} else if compResult, err := as.qs.runner.RunQuery(ctx, compQuery, site); err != nil {
			slog.Warn("failed to run comparison query", "error", err)
		} else {
			comparisonData = compResult.Data
		}
	}

	// conversion_rate denominator: baseline visitors (same site/time window, uniq visitors excluding goal filter).
	// Consistent with funnel — compute ratio in Go from separate numerator/denominator queries, not a single SQL expression.
	var baselineVisitors, compBaselineVisitors float64
	if needsConversionRate {
		baselineVisitors = as.baselineVisitors(ctx, site, params, params.UTCTimeRange)
		if params.ComparisonUTCTimeRange != nil {
			compBaselineVisitors = as.baselineVisitors(ctx, site, params, *params.ComparisonUTCTimeRange)
		}
	}

	for _, metric := range params.Metrics {
		if metric == "conversion_rate" && needsConversionRate {
			rate := conversionRate(toFloat64(getMetricValue(result.Data, "conversions")), baselineVisitors)
			mr := MetricResult{Value: rate}
			if comparisonData != nil && len(comparisonData) > 0 {
				compRate := conversionRate(toFloat64(getMetricValue(comparisonData, "conversions")), compBaselineVisitors)
				mr.ComparisonValue = &compRate
				mr.Change = calculateChange(rate, compRate)
			}
			results[metric] = mr
			continue
		}
		if metric == "views_per_visit" && needsViewsPerVisit {
			// views_per_visit = pageviews / visits，从查询结果中计算
			pv := getMetricValue(result.Data, "pageviews")
			vt := getMetricValue(result.Data, "visits")
			pvFloat := toFloat64(pv)
			vtFloat := toFloat64(vt)
			var vpvFloat float64
			if vtFloat > 0 {
				vpvFloat = math.Round(pvFloat/vtFloat*100) / 100
			}
			mr := MetricResult{Value: vpvFloat}
			if comparisonData != nil && len(comparisonData) > 0 {
				compPV := toFloat64(getMetricValue(comparisonData, "pageviews"))
				compVT := toFloat64(getMetricValue(comparisonData, "visits"))
				var compVPV float64
				if compVT > 0 {
					compVPV = math.Round(compPV/compVT*100) / 100
				}
				mr.ComparisonValue = &compVPV
				mr.Change = calculateChange(mr.Value, compVPV)
			}
			results[metric] = mr
			continue
		}
		mr := buildMetricResult(result.Data, metric)
		if comparisonData != nil && len(comparisonData) > 0 {
			compValue := getMetricValue(comparisonData, metric)
			compFloat := toFloat64(compValue)
			mr.ComparisonValue = &compFloat
			mr.Change = calculateChange(mr.Value, compValue)
		}
		results[metric] = mr
	}

	return &AggregateResult{
		Results: results,
	}, nil
}

// getMetricValue 从查询结果数据中获取指定指标的值。
func getMetricValue(data []map[string]any, metric string) any {
	if len(data) == 0 {
		return 0
	}
	if val, ok := data[0][metric]; ok {
		return val
	}
	if val, ok := data[0]["cur_"+metric]; ok {
		return val
	}
	return 0
}

// calculateChange 计算变化百分比
func calculateChange(current, previous any) *float64 {
	cur := toFloat64(current)
	prev := toFloat64(previous)
	if prev == 0 {
		if cur == 0 {
			change := 0.0
			return &change
		}
		return nil // 无法计算变化率
	}
	change := (cur - prev) / prev * 100
	change = math.Round(change*100) / 100
	return &change
}

// toFloat64 将任意数值类型转换为 float64。
func toFloat64(val any) float64 {
	if val == nil {
		return 0
	}
	switch v := val.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int32:
		return float64(v)
	case int64:
		return float64(v)
	case uint32:
		return float64(v)
	case uint64:
		return float64(v)
	default:
		return 0
	}
}

func buildMetricResult(entry []map[string]any, metric string) MetricResult {
	if len(entry) == 0 {
		return MetricResult{
			Value: 0,
		}
	}

	if val, ok := entry[0][metric]; ok {
		return MetricResult{
			Value: maybeRoundValue(val, metric),
		}
	}
	// fix nested aggregation function when views_per_visit and pageviews are obtained simultaneously
	if val, ok := entry[0]["cur_"+metric]; ok {
		return MetricResult{
			Value: maybeRoundValue(val, metric),
		}
	}

	return MetricResult{
		Value: 0,
	}
}

func maybeRoundValue(val any, metric string) any {
	if val == nil {
		return nil
	}
	if metricsToRound[metric] {
		switch v := val.(type) {
		case float64:
			return math.Round(v)
		case float32:
			return math.Round(float64(v))
		}
	}
	return val
}

// intervalToDimension 根据 interval 返回对应的时间维度
func intervalToDimension(interval string) string {
	switch interval {
	case "minute":
		return "time:minute"
	case "hourly", "hour":
		return "time:hour"
	case "daily", "day":
		return "time:day"
	case "weekly", "week":
		return "time:week"
	case "monthly", "month":
		return "time:month"
	case "yearly", "year":
		return "time:year"
	default:
		return "time:hour"
	}
}

// GetTimeSeries 获取时序聚合数据
func (as *AggregateService) GetTimeSeries(ctx context.Context, params *types.Params) ([]TimeSeriesPoint, error) {
	// 验证时间间隔
	interval, err := ParseInterval(params.Interval)
	if err != nil {
		return nil, err
	}

	// 处理 views_per_visit：替换为 visits 指标，事后在 Go 层计算
	needsViewsPerVisit := containsString(params.Metrics, "views_per_visit")
	queryMetrics := replaceViewsPerVisit(params.Metrics)

	// 桶数上限：时间维度不再有默认 LIMIT（见 query_builder.paginate），
	// 这里限制 范围/颗粒度 组合的桶数，避免超长 custom 区间配过细颗粒度打爆查询与前端渲染。
	if n := estimateBucketCount(params.UTCTimeRange, interval); n > maxTimeSeriesBuckets {
		return nil, &ValidationError{
			StatusCode: 400,
			Message:    fmt.Sprintf("interval %q is too fine for the selected period (%d buckets, max %d) — choose a coarser interval", interval, n, maxTimeSeriesBuckets),
		}
	}

	// 动态设置时间维度
	dimension := intervalToDimension(string(interval))
	tsParams := &types.Params{
		SiteID:       params.SiteID,
		Period:       params.Period,
		Date:         params.Date,
		From:         params.From,
		To:           params.To,
		Timezone:     params.Timezone,
		UTCTimeRange: params.UTCTimeRange,
		Metrics:      queryMetrics,
		Dimensions:   []string{dimension}, // 动态设置
		Filters:      params.Filters,
		Interval:     string(interval),
	}

	query, err := as.qs.CreateQuery(tsParams)
	if err != nil {
		return nil, err
	}
	site := &types.Site{ID: query.SiteID, UserID: query.UserID, Timezone: query.Timezone}
	result, err := as.qs.runner.RunQuery(ctx, query, site)
	if err != nil {
		return nil, fmt.Errorf("failed to run time series query: %w", err)
	}

	points := []TimeSeriesPoint{}

	for _, row := range result.Data {
		var timestamp string

		// processResults 将单维度列重命名为 "name"，所以用 "name" 作为 key
		if val, ok := row["name"]; ok {
			timestamp = fmt.Sprintf("%v", val)
		}
		metrics := make(map[string]any)
		for _, metric := range params.Metrics {
			if metric == "views_per_visit" && needsViewsPerVisit {
				pv := toFloat64(row["pageviews"])
				vt := toFloat64(row["visits"])
				if vt > 0 {
					metrics[metric] = math.Round(pv/vt*100) / 100
				} else {
					metrics[metric] = 0
				}
			} else if v, ok := row[metric]; ok {
				metrics[metric] = v
			} else {
				metrics[metric] = 0
			}
		}
		points = append(points, TimeSeriesPoint{
			Timestamp: timestamp,
			Metrics:   metrics,
		})
	}

	return as.fillMissingTimePoints(points, interval, params), nil
}

// maxTimeSeriesBuckets 单次时序查询允许的最大桶数。
// 覆盖所有合理组合（30d×hourly=720、12mo×daily=365、6 个月×hourly≈4400），
// 超出即提示改用更粗颗粒度。
const maxTimeSeriesBuckets = 5000

// estimateBucketCount 估算给定时间范围与颗粒度的桶数（上取整）。
func estimateBucketCount(tr types.TimeRange, interval Interval) int {
	dur := tr.End.Sub(tr.Start)
	if dur <= 0 {
		return 0
	}
	var unit time.Duration
	switch interval {
	case IntervalMinute:
		unit = time.Minute
	case IntervalHourly:
		unit = time.Hour
	case IntervalDaily:
		unit = 24 * time.Hour
	case IntervalWeekly:
		unit = 7 * 24 * time.Hour
	case IntervalMonthly:
		unit = 28 * 24 * time.Hour
	case IntervalYearly:
		unit = 365 * 24 * time.Hour
	default:
		unit = time.Hour
	}
	return int(dur/unit) + 1
}

// fillMissingTimePoints 填补时序数据中缺失的时间点
func (as *AggregateService) fillMissingTimePoints(points []TimeSeriesPoint, interval Interval, params *types.Params) []TimeSeriesPoint {
	//将 params.Timezone 转换为 time.Location
	loc, err := time.LoadLocation(params.Timezone)
	if err != nil {
		// 处理错误，例如使用默认时区
		loc = time.UTC
	}

	// 按站点本地时区生成完整的时间范围。
	// SQL 现已用 toTimeZone(timestamp, tz) 在本地时区分桶（标签为本地日期/时刻），
	// 因此这里也在本地时区生成期望桶并直接用本地键匹配，不再做 UTC→本地二次重贴。
	localStart := params.UTCTimeRange.Start.In(loc)
	localEnd := params.UTCTimeRange.End.In(loc)
	ranges, err := GenerateTimeRanges(localStart, localEnd, interval)
	if err != nil || len(ranges) == 0 {
		return points
	}

	// 创建时间点映射（本地字符串，与 SQL 标签一致）
	pointMap := make(map[string]TimeSeriesPoint)
	for _, p := range points {
		pointMap[p.Timestamp] = p
	}

	completePoints := []TimeSeriesPoint{}

	for _, r := range ranges {
		// 本地桶起始时间格式化为键（r.Start 已在站点时区）
		key := formatTimestamp(r.Start, interval)

		if pt, exists := pointMap[key]; exists {
			completePoints = append(completePoints, pt)
			continue
		}
		emptyMetrics := make(map[string]any)
		for _, m := range params.Metrics {
			emptyMetrics[m] = 0
		}
		completePoints = append(completePoints, TimeSeriesPoint{
			Timestamp: key,
			Metrics:   emptyMetrics,
		})
	}

	return completePoints
}

// formatTimestamp 格式化时间戳
func formatTimestamp(t time.Time, interval Interval) string {
	switch interval {
	case IntervalMinute:
		return t.Format("2006-01-02 15:04")
	case IntervalHourly:
		return t.Format("2006-01-02 15")
	case IntervalDaily, IntervalWeekly, IntervalMonthly:
		return t.Format("2006-01-02")
	case IntervalYearly:
		return t.Format("2006")
	default:
		return t.Format("2006-01-02 15")
	}
}

func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// replaceConversionRate replaces conversion_rate with its numerator metric conversions (dedup),
// the ratio is computed in Go using baseline visitors.
func replaceConversionRate(metrics []string) []string {
	result := make([]string, 0, len(metrics))
	for _, m := range metrics {
		if m == "conversion_rate" {
			if !containsString(result, "conversions") {
				result = append(result, "conversions")
			}
			continue
		}
		if m == "conversions" && containsString(result, "conversions") {
			continue
		}
		result = append(result, m)
	}
	return result
}

// baselineVisitors queries baseline unique visitors for the given time window: keeps original filters but strips goal filters,
// used as the denominator for conversion_rate. Returns 0 on query failure (ratio becomes 0).
func (as *AggregateService) baselineVisitors(ctx context.Context, site *types.Site, params *types.Params, tr types.TimeRange) float64 {
	baseParams := &types.Params{
		SiteID:       params.SiteID,
		Period:       params.Period,
		Date:         params.Date,
		From:         params.From,
		To:           params.To,
		Timezone:     params.Timezone,
		UTCTimeRange: tr,
		Metrics:      []string{"visitors"},
		Dimensions:   []string{},
		Filters:      stripGoalFilters(params.Filters),
		Interval:     "",
	}
	query, err := as.qs.CreateQuery(baseParams)
	if err != nil {
		slog.Warn("failed to create conversion_rate baseline query", "error", err)
		return 0
	}
	res, err := as.qs.runner.RunQuery(ctx, query, site)
	if err != nil {
		slog.Warn("failed to run conversion_rate baseline query", "error", err)
		return 0
	}
	return toFloat64(getMetricValue(res.Data, "visitors"))
}

// conversionRate computes conversion rate (%, 1 decimal place), returns 0 when denominator is 0.
func conversionRate(conversions, baseline float64) float64 {
	if baseline <= 0 {
		return 0
	}
	return math.Round(conversions*100/baseline*10) / 10
}

// stripGoalFilters returns filters with event:goal removed (used for baseline visitor computation).
func stripGoalFilters(filters []*types.Filter) []*types.Filter {
	out := make([]*types.Filter, 0, len(filters))
	for _, f := range filters {
		if f.AnyDimension(func(d string) bool { return d == "event:goal" }) {
			continue
		}
		out = append(out, f)
	}
	return out
}

// replaceViewsPerVisit replaces "views_per_visit" with "visits" in metrics list,
// and ensures "pageviews" is present (needed for post-processing computation).
func replaceViewsPerVisit(metrics []string) []string {
	result := make([]string, 0, len(metrics)+1)
	seen := make(map[string]bool, len(metrics)+1)
	add := func(m string) {
		if !seen[m] {
			seen[m] = true
			result = append(result, m)
		}
	}
	for _, m := range metrics {
		if m == "views_per_visit" {
			// views_per_visit 由 visits + pageviews 在 Go 层计算；去重避免与显式请求的
			// visits/pageviews 产生重复列别名（ClickHouse 会因重复别名报错）。
			add("visits")
		} else {
			add(m)
		}
	}
	if !seen["pageviews"] {
		add("pageviews")
	}
	return result
}
