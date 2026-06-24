package service

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/dromara/carbon/v2"
	"github.com/gin-gonic/gin"
	atypes "github.com/zenstats/zenstats/internal/api/types"
	"github.com/zenstats/zenstats/internal/service/stats"
	"github.com/zenstats/zenstats/internal/service/stats/types"
	cl "github.com/zenstats/zenstats/internal/store/clickhouse"
	"github.com/zenstats/zenstats/internal/store/postgresql"
	"github.com/zenstats/zenstats/pkg/globals"
)

// StatsService 统计服务，基于查询引擎提供聚合、时序、细分等统计查询。
type StatsService struct {
	db *postgresql.Client
	cl driver.Conn
}

// GetStatsService 获取 StatsService 单例实例。
var GetStatsService = sync.OnceValue(func() *StatsService {
	db := globals.GetDB()
	if db == nil {
		panic("DB is not initialized")
	}
	return &StatsService{
		db: db,
		cl: cl.GetConnection(),
	}
})

// GetAggregate 获取聚合统计（含对比数据）
func (s *StatsService) GetAggregate(ctx *gin.Context, siteID int64, req *atypes.StatsRequest) (*stats.AggregateResult, error) {
	site, err := GetSiteService().GetSiteByID(ctx, int(siteID))
	if err != nil {
		return nil, fmt.Errorf("site not found")
	}
	start, end := s.getDateRange(req, site.Timezone, 0)
	filters, err := parseRequestFilters(req.Filters)
	if err != nil {
		return nil, err
	}

	// 计算对比时间范围：优先使用显式对比日期，否则自动计算上一同期
	var comparisonStart, comparisonEnd time.Time
	if req.CompareFrom != "" {
		comparisonStart, comparisonEnd = s.getDateRange(&atypes.StatsRequest{
			Period: "custom",
			From:   req.CompareFrom,
			To:     req.CompareTo,
		}, site.Timezone, 0)
	} else {
		comparisonStart, comparisonEnd = s.getComparisonDateRange(req, site.Timezone)
	}

	params := &types.Params{
		UserID:    ctx.GetInt64("user_id"),
		Interval: req.Interval,
		SiteID:   fmt.Sprintf("%d", site.ID),
		UTCTimeRange: types.TimeRange{
			Start: start,
			End:   end,
		},
		ComparisonUTCTimeRange: &types.TimeRange{
			Start: comparisonStart,
			End:   comparisonEnd,
		},
		Period:          req.Period,
		Date:            req.Date,
		From:            req.From,
		To:              req.To,
		Timezone:        site.Timezone,
		Metrics:         parseMetricsWithDefault(req.Metrics, []string{"visitors", "pageviews", "bounce_rate", "visit_duration", "views_per_visit"}),
		Dimensions:      []string{},
		Filters:         filters,
		SampleThreshold: req.SampleThreshold,
	}
	qs := s.getQueryService()

	aggregateService := stats.NewAggregateService(qs)

	return aggregateService.GetAggregate(ctx, params)
}

// GetTimeSeries 获取时间序列统计数据（默认 visitors 指标）。
func (s *StatsService) GetTimeSeries(ctx *gin.Context, siteID int64, req *atypes.StatsRequest) ([]stats.TimeSeriesPoint, error) {
	site, err := GetSiteService().GetSiteByID(ctx, int(siteID))
	if err != nil {
		return nil, fmt.Errorf("site not found")
	}
	start, end := s.getDateRange(req, site.Timezone, 0)
	filters, err := parseRequestFilters(req.Filters)
	if err != nil {
		return nil, err
	}

	// 当未指定 interval 时，根据周期类型选择默认值
	interval := req.Interval
	if interval == "" {
		interval = stats.DefaultIntervalForPeriod(req.Period)
	}

	params := &types.Params{
		UserID:   ctx.GetInt64("user_id"),
		Interval: interval,
		SiteID:   fmt.Sprintf("%d", site.ID),
		UTCTimeRange: types.TimeRange{
			Start: start,
			End:   end,
		},
		Period:          req.Period,
		Date:            req.Date,
		From:            req.From,
		To:              req.To,
		Timezone:        site.Timezone,
		Metrics:         parseMetricsWithDefault(req.Metrics, []string{"visitors"}),
		Dimensions:      []string{},
		Filters:         filters,
		SampleThreshold: req.SampleThreshold,
	}
	qs := s.getQueryService()

	aggregateService := stats.NewAggregateService(qs)

	return aggregateService.GetTimeSeries(ctx, params)
}

// GetBreakdown 按指定维度获取细分数据
func (s *StatsService) GetBreakdown(ctx *gin.Context, siteID int64, req *atypes.StatsRequest, property, metricsStr string) (*types.QueryResult, error) {
	site, err := GetSiteService().GetSiteByID(ctx, int(siteID))
	if err != nil {
		return nil, fmt.Errorf("site not found")
	}
	start, end := s.getDateRange(req, site.Timezone, 0)
	filters, err := parseRequestFilters(req.Filters)
	if err != nil {
		return nil, err
	}
	offset := 0
	if req.Limit <= 0 {
		req.Limit = 9
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.Page > 1 {
		offset = (req.Page - 1) * req.Limit
	}
	// 解析 metrics 逗号分隔
	metricsList := parseMetrics(metricsStr)

	// 设置默认排序：按第一个指标降序
	orderBy := []*types.OrderBy{
		{Dimension: metricsList[0], Direction: "desc"},
	}

	params := &types.Params{
		UserID:   ctx.GetInt64("user_id"),
		SiteID: fmt.Sprintf("%d", site.ID),
		UTCTimeRange: types.TimeRange{
			Start: start,
			End:   end,
		},
		Period:          req.Period,
		Date:            req.Date,
		From:            req.From,
		To:              req.To,
		Timezone:        site.Timezone,
		Metrics:         metricsList,
		Dimensions:      []string{property},
		Filters:         filters,
		SampleThreshold: req.SampleThreshold,
		Pagination: &types.Pagination{
			Limit:  req.Limit,
			Offset: offset,
		},
		OrderBy: orderBy,
	}
	qs := s.getQueryService()

	query, err := qs.CreateQuery(params)
	if err != nil {
		return nil, err
	}
	psite := &types.Site{ID: query.SiteID, Timezone: query.Timezone}

	return qs.Execute(ctx, query, psite)
}

// GetMainGraph 获取主图表时序数据
func (s *StatsService) GetMainGraph(ctx *gin.Context, siteID int64, req *atypes.StatsRequest, metricsStr string) ([]stats.TimeSeriesPoint, error) {
	site, err := GetSiteService().GetSiteByID(ctx, int(siteID))
	if err != nil {
		return nil, fmt.Errorf("site not found")
	}
	start, end := s.getDateRange(req, site.Timezone, 0)
	filters, err := parseRequestFilters(req.Filters)
	if err != nil {
		return nil, err
	}

	metricsList := parseMetrics(metricsStr)

	// 当未指定 interval 时，根据周期类型选择默认值
	interval := req.Interval
	if interval == "" {
		interval = stats.DefaultIntervalForPeriod(req.Period)
	}

	params := &types.Params{
		UserID:   ctx.GetInt64("user_id"),
		Interval: interval,
		SiteID:   fmt.Sprintf("%d", site.ID),
		UTCTimeRange: types.TimeRange{
			Start: start,
			End:   end,
		},
		Period:          req.Period,
		Date:            req.Date,
		From:            req.From,
		To:              req.To,
		Timezone:        site.Timezone,
		Metrics:         metricsList,
		Dimensions:      []string{},
		Filters:         filters,
		SampleThreshold: req.SampleThreshold,
	}
	qs := s.getQueryService()

	aggregateService := stats.NewAggregateService(qs)

	points, err := aggregateService.GetTimeSeries(ctx, params)
	if err != nil {
		return nil, err
	}

	// 如果有对比标志或显式对比日期，获取对比时段数据并合并
	if req.Compare == "1" || req.CompareFrom != "" {
		var compStart, compEnd time.Time
		var compPeriod, compFrom, compTo string
		if req.CompareFrom != "" {
			// 显式对比日期
			compStart, compEnd = s.getDateRange(&atypes.StatsRequest{
				Period: "custom",
				From:   req.CompareFrom,
				To:     req.CompareTo,
			}, site.Timezone, 0)
			compPeriod = "custom"
			compFrom = req.CompareFrom
			compTo = req.CompareTo
		} else {
			// 自动计算上一同期
			compStart, compEnd = s.getComparisonDateRange(req, site.Timezone)
			compPeriod = req.Period
			compFrom = compStart.Format("2006-01-02")
			compTo = compEnd.Format("2006-01-02")
		}

		compParams := &types.Params{
			UserID:   ctx.GetInt64("user_id"),
			Interval: interval,
			SiteID:   fmt.Sprintf("%d", site.ID),
			UTCTimeRange: types.TimeRange{
				Start: compStart,
				End:   compEnd,
			},
			Period:          compPeriod,
			Date:            compFrom, // 对比时段日期，validation 需要
			From:            compFrom,
			To:              compTo,
			Timezone:        site.Timezone,
			Metrics:         metricsList,
			Dimensions:      []string{},
			Filters:         filters,
			SampleThreshold: req.SampleThreshold,
		}

		compPoints, err := aggregateService.GetTimeSeries(ctx, compParams)
		if err != nil {
			// 对比数据获取失败，记录日志但不中断；曲线显示为 0
			slog.Warn("failed to fetch comparison time series", "error", err)
			compPoints = nil
		}

		// 合并对比数据（compPoints 为空时 merge 会补 0，曲线正常渲染）
		points = mergeComparisonTimeSeries(points, compPoints, metricsList)
	}

	return points, nil
}

// mergeComparisonTimeSeries 将对比时序数据合并到主时序数据中（指标名加 _comparison 后缀）。
// 当对比数据不足时（数组更短），缺失的位置补 0，确保前端能渲染对比曲线。
func mergeComparisonTimeSeries(primary, comparison []stats.TimeSeriesPoint, metrics []string) []stats.TimeSeriesPoint {
	for i := range primary {
		for _, m := range metrics {
			if i < len(comparison) {
				if v, exists := comparison[i].Metrics[m]; exists {
					primary[i].Metrics[m+"_comparison"] = v
					continue
				}
			}
			// 对比数据缺失 → 补 0，让前端对比曲线正常渲染
			primary[i].Metrics[m+"_comparison"] = float64(0)
		}
	}
	return primary
}

// GetCurrentVisitors 获取实时在线访客数
func (s *StatsService) GetCurrentVisitors(ctx *gin.Context, siteID int64) (*stats.CurrentVisitors, error) {
	_, err := GetSiteService().GetSiteByID(ctx, int(siteID))
	if err != nil {
		return nil, fmt.Errorf("site not found")
	}

	runner := stats.NewQueryRunner()
	cvService := stats.NewCurrentVisitorsService(runner)

	return cvService.GetCurrentVisitors(ctx, fmt.Sprintf("%d", siteID), 0)
}

// parseMetrics 解析逗号分隔的指标列表
func parseMetrics(metricsStr string) []string {
	return parseMetricsWithDefault(metricsStr, []string{"visitors"})
}

func parseMetricsWithDefault(metricsStr string, defaults []string) []string {
	if metricsStr == "" {
		return defaults
	}
	parts := strings.Split(metricsStr, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	if len(result) == 0 {
		return defaults
	}
	return result
}

func parseRequestFilters(raw string) ([]*types.Filter, error) {
	return types.ParseRawFiltersJSON(raw)
}

// getQueryService 创建并返回查询服务实例。
func (s *StatsService) getQueryService() *stats.QueryService {
	runner := stats.NewQueryRunner()
	return stats.NewQueryService(runner)
}

// getComparisonDateRange 计算对比时间范围（上一同期）
func (s *StatsService) getComparisonDateRange(req *atypes.StatsRequest, timezone string) (time.Time, time.Time) {
	switch req.Period {
	case "day", "yesterday":
		return s.getDateRange(req, timezone, -1)
	case "p7":
		return s.getDateRange(req, timezone, -7)
	case "p14":
		return s.getDateRange(req, timezone, -14)
	case "p30":
		return s.getDateRange(req, timezone, -30)
	case "custom":
		start := carbon.Parse(req.From, timezone)
		end := carbon.Parse(req.To, timezone)
		days := end.DiffInDays(start) + 1
		return s.getDateRange(req, timezone, -int(days))
	default:
		return s.getDateRange(req, timezone, -1)
	}
}

// getDateRange 根据请求参数计算UTC日期范围
func (s *StatsService) getDateRange(req *atypes.StatsRequest, timezone string, offsetDays int) (startDate, endDate time.Time) {
	switch req.Period {
	case "realtime":
		endDate = carbon.Now(timezone).SetTimezone(carbon.UTC).StdTime()
		startDate = carbon.Now(timezone).SubMinutes(30).SetTimezone(carbon.UTC).StdTime()
	case "day", "yesterday":
		date := carbon.Parse(req.Date, timezone).AddDays(offsetDays)
		startDate = date.StartOfDay().SetTimezone(carbon.UTC).StdTime()
		endDate = date.EndOfDay().SetTimezone(carbon.UTC).StdTime()
	case "p7":
		baseDate := carbon.Parse(req.Date, timezone).AddDays(offsetDays)
		endDate = baseDate.EndOfDay().SetTimezone(carbon.UTC).StdTime()
		startDate = baseDate.SubDays(6).StartOfDay().SetTimezone(carbon.UTC).StdTime()
	case "p14":
		baseDate := carbon.Parse(req.Date, timezone).AddDays(offsetDays)
		endDate = baseDate.EndOfDay().SetTimezone(carbon.UTC).StdTime()
		startDate = baseDate.SubDays(13).StartOfDay().SetTimezone(carbon.UTC).StdTime()
	case "p30":
		baseDate := carbon.Parse(req.Date, timezone).AddDays(offsetDays)
		endDate = baseDate.EndOfDay().SetTimezone(carbon.UTC).StdTime()
		startDate = baseDate.SubDays(29).StartOfDay().SetTimezone(carbon.UTC).StdTime()
	case "w":
		baseDate := carbon.Parse(req.Date, timezone).AddDays(offsetDays)
		startDate = baseDate.StartOfWeek().StartOfDay().SetTimezone(carbon.UTC).StdTime()
		endDate = baseDate.EndOfWeek().EndOfDay().SetTimezone(carbon.UTC).StdTime()
	case "m":
		baseDate := carbon.Parse(req.Date, timezone).AddDays(offsetDays)
		startDate = baseDate.StartOfMonth().StartOfDay().SetTimezone(carbon.UTC).StdTime()
		endDate = baseDate.EndOfMonth().EndOfDay().SetTimezone(carbon.UTC).StdTime()
	case "custom":
		if hasTime := len(req.From) > 10 || len(req.To) > 10; hasTime {
			// 含时间部分（如 "2026-06-25 14:00"），保留精确时间
			fromCarbon := carbon.Parse(req.From, timezone)
			toCarbon := carbon.Parse(req.To, timezone)
			startDate = fromCarbon.AddDays(offsetDays).SetTimezone(carbon.UTC).StdTime()
			endDate = toCarbon.AddDays(offsetDays).SetTimezone(carbon.UTC).StdTime()
		} else {
			startDate = carbon.Parse(req.From, timezone).StartOfDay().AddDays(offsetDays).SetTimezone(carbon.UTC).StdTime()
			endDate = carbon.Parse(req.To, timezone).EndOfDay().AddDays(offsetDays).SetTimezone(carbon.UTC).StdTime()
		}
	}

	return
}
