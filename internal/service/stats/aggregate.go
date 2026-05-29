package stats

import (
	"context"
	"fmt"
	"math"
	"strings"
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
	Value           any `json:"value"`
	ComparisonValue any `json:"comparison_value,omitempty"`
	Change          any `json:"change,omitempty"`
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
	// 创建无维度的查询参数
	aggParams := &types.Params{
		SiteID:       params.SiteID,
		Period:       params.Period,
		Date:         params.Date,
		From:         params.From,
		To:           params.To,
		Timezone:     params.Timezone,
		UTCTimeRange: params.UTCTimeRange,
		Metrics:      params.Metrics,
		Dimensions:   []string{}, // 无维度，只获取总计
		Filters:      params.Filters,
		Interval:     "",
	}
	query, err := as.qs.CreateQuery(aggParams)
	if err != nil {
		return nil, err
	}
	site := &types.Site{ID: query.SiteID, Timezone: query.Timezone}
	// 执行主查询
	result, err := as.qs.runner.RunQuery(ctx, query, site)
	if err != nil {
		return nil, fmt.Errorf("failed to run aggregate query: %v", err)
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
			Metrics:      params.Metrics,
			Dimensions:   []string{},
			Filters:      params.Filters,
			Interval:     "",
		}
		compQuery, err := as.qs.CreateQuery(compParams)
		if err == nil {
			compResult, err := as.qs.runner.RunQuery(ctx, compQuery, site)
			if err == nil {
				comparisonData = compResult.Data
			}
		}
	}

	for _, metric := range params.Metrics {
		mr := buildMetricResult(result.Data, metric)
		if comparisonData != nil && len(comparisonData) > 0 {
			compValue := getMetricValue(comparisonData, metric)
			mr.ComparisonValue = compValue
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

	// 动态设置时间维度
	dimension := intervalToDimension(params.Interval)
	tsParams := &types.Params{
		SiteID:       params.SiteID,
		Period:       params.Period,
		Date:         params.Date,
		From:         params.From,
		To:           params.To,
		Timezone:     params.Timezone,
		UTCTimeRange: params.UTCTimeRange,
		Metrics:      params.Metrics,
		Dimensions:   []string{dimension}, // 动态设置
		Filters:      params.Filters,
		Interval:     params.Interval,
	}

	query, err := as.qs.CreateQuery(tsParams)
	if err != nil {
		return nil, err
	}
	site := &types.Site{ID: query.SiteID, Timezone: query.Timezone}
	result, err := as.qs.runner.RunQuery(ctx, query, site)
	if err != nil {
		return nil, fmt.Errorf("failed to run time series query: %v", err)
	}

	points := []TimeSeriesPoint{}

	for _, row := range result.Data {
		var timestamp string

		key := strings.TrimPrefix(dimension, "time:")
		if val, ok := row[key]; ok {
			timestamp = fmt.Sprintf("%v", val)
		}
		metrics := make(map[string]any)
		for _, metric := range params.Metrics {
			if v, ok := row[metric]; ok {
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

// fillMissingTimePoints 填补时序数据中缺失的时间点
func (as *AggregateService) fillMissingTimePoints(points []TimeSeriesPoint, interval Interval, params *types.Params) []TimeSeriesPoint {
	//将 params.Timezone 转换为 time.Location
	loc, err := time.LoadLocation(params.Timezone)
	if err != nil {
		// 处理错误，例如使用默认时区
		loc = time.UTC
	}

	// 生成完整的时间范围（UTC）
	ranges, err := GenerateTimeRanges(params.UTCTimeRange.Start.UTC(), params.UTCTimeRange.End.UTC(), interval)
	if err != nil || len(ranges) == 0 {
		return points
	}

	// 创建时间点映射（UTC字符串）
	pointMap := make(map[string]TimeSeriesPoint)
	for _, p := range points {
		pointMap[p.Timestamp] = p
	}

	completePoints := []TimeSeriesPoint{}

	for _, r := range ranges {
		// 格式化UTC时间戳
		utcTimestamp := formatTimestamp(r.Start.UTC(), interval)

		// 查找现有数据点
		var p TimeSeriesPoint
		if pt, exists := pointMap[utcTimestamp]; exists {
			p = pt
		} else {
			emptyMetrics := make(map[string]any)
			for _, m := range params.Metrics {
				emptyMetrics[m] = 0
			}
			p = TimeSeriesPoint{
				Timestamp: utcTimestamp,
				Metrics:   emptyMetrics,
			}
		}

		// 转换为目标时区
		t, err := time.ParseInLocation(getTimestampLayout(interval), p.Timestamp, time.UTC)
		if err == nil {
			p.Timestamp = t.In(loc).Format(getTimestampLayout(interval))
		}
		completePoints = append(completePoints, p)
	}

	return completePoints
}

// 获取时间戳格式
func getTimestampLayout(interval Interval) string {
	switch interval {
	case IntervalMinute:
		return "2006-01-02 15:04"
	case IntervalHourly:
		return "2006-01-02 15"
	case IntervalDaily, IntervalWeekly, IntervalMonthly:
		return "2006-01-02"
	case IntervalYearly:
		return "2006"
	default:
		return "2006-01-02 15"
	}
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
