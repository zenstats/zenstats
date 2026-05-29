package service

import (
	"fmt"
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

var (
	statsServiceInstance *StatsService
	statsOnce            sync.Once
)

// StatsService 统计服务，基于查询引擎提供聚合、时序、细分等统计查询。
type StatsService struct {
	db *postgresql.Client
	cl driver.Conn
}

// GetStatsService 获取 StatsService 单例实例。
func GetStatsService() *StatsService {
	statsOnce.Do(func() {
		db := globals.GetDB()
		if db == nil {
			panic("DB is not initialized")
		}
		statsServiceInstance = &StatsService{
			db: db,
			cl: cl.GetConnection(),
		}
	})
	return statsServiceInstance
}

// GetAggregate 获取聚合统计（含对比数据）
func (s *StatsService) GetAggregate(ctx *gin.Context, domain string, req *atypes.StatsRequest) (*stats.AggregateResult, error) {
	site, err := GetSiteService().GetSiteByDomain(ctx, domain)
	if err != nil {
		return nil, fmt.Errorf("site not found")
	}
	start, end := s.getDateRange(req, site.Timezone, 0)

	// 计算对比时间范围（上一同期）
	comparisonStart, comparisonEnd := s.getComparisonDateRange(req, site.Timezone)

	params := &types.Params{
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
		Period:     req.Period,
		Date:       req.Date,
		From:       req.From,
		To:         req.To,
		Timezone:   site.Timezone,
		Metrics:    []string{"visitors", "pageviews", "bounce_rate", "visit_duration", "views_per_visit"},
		Dimensions: []string{},
	}
	qs := s.getQueryService()

	aggregateService := stats.NewAggregateService(qs)

	return aggregateService.GetAggregate(ctx, params)
}

// GetTimeSeries 获取时间序列统计数据（默认 visitors 指标）。
func (s *StatsService) GetTimeSeries(ctx *gin.Context, domain string, req *atypes.StatsRequest) ([]stats.TimeSeriesPoint, error) {
	site, err := GetSiteService().GetSiteByDomain(ctx, domain)
	if err != nil {
		return nil, fmt.Errorf("site not found")
	}
	start, end := s.getDateRange(req, site.Timezone, 0)

	params := &types.Params{
		Interval: req.Interval,
		SiteID:   fmt.Sprintf("%d", site.ID),
		UTCTimeRange: types.TimeRange{
			Start: start,
			End:   end,
		},
		Period:     req.Period,
		Date:       req.Date,
		From:       req.From,
		To:         req.To,
		Timezone:   site.Timezone,
		Metrics:    []string{"visitors"},
		Dimensions: []string{},
	}
	qs := s.getQueryService()

	aggregateService := stats.NewAggregateService(qs)

	return aggregateService.GetTimeSeries(ctx, params)
}

// GetBreakdown 按指定维度获取细分数据
func (s *StatsService) GetBreakdown(ctx *gin.Context, domain string, req *atypes.StatsRequest, property, metricsStr string) (*types.QueryResult, error) {
	site, err := GetSiteService().GetSiteByDomain(ctx, domain)
	if err != nil {
		return nil, fmt.Errorf("site not found")
	}
	start, end := s.getDateRange(req, site.Timezone, 0)
	filter, err := types.ParseRawFilter(req.Filters)
	if err != nil {
		return nil, err
	}
	offset := 0
	if req.Page > 1 {
		offset = (req.Page - 1) * req.Limit
	}
	filters := []*types.Filter{}
	if filter != nil {
		filters = append(filters, filter)
	}

	// 解析 metrics 逗号分隔
	metricsList := parseMetrics(metricsStr)

	// 设置默认排序：按第一个指标降序
	orderBy := []*types.OrderBy{
		{Dimension: metricsList[0], Direction: "desc"},
	}

	params := &types.Params{
		SiteID: fmt.Sprintf("%d", site.ID),
		UTCTimeRange: types.TimeRange{
			Start: start,
			End:   end,
		},
		Period:     req.Period,
		Date:       req.Date,
		From:       req.From,
		To:         req.To,
		Timezone:   site.Timezone,
		Metrics:    metricsList,
		Dimensions: []string{property},
		Filters:    filters,
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
func (s *StatsService) GetMainGraph(ctx *gin.Context, domain string, req *atypes.StatsRequest, metricsStr string) ([]stats.TimeSeriesPoint, error) {
	site, err := GetSiteService().GetSiteByDomain(ctx, domain)
	if err != nil {
		return nil, fmt.Errorf("site not found")
	}
	start, end := s.getDateRange(req, site.Timezone, 0)

	metricsList := parseMetrics(metricsStr)

	params := &types.Params{
		Interval: req.Interval,
		SiteID:   fmt.Sprintf("%d", site.ID),
		UTCTimeRange: types.TimeRange{
			Start: start,
			End:   end,
		},
		Period:     req.Period,
		Date:       req.Date,
		From:       req.From,
		To:         req.To,
		Timezone:   site.Timezone,
		Metrics:    metricsList,
		Dimensions: []string{},
	}
	qs := s.getQueryService()

	aggregateService := stats.NewAggregateService(qs)

	return aggregateService.GetTimeSeries(ctx, params)
}

// GetCurrentVisitors 获取实时在线访客数
func (s *StatsService) GetCurrentVisitors(ctx *gin.Context, domain string) (*stats.CurrentVisitors, error) {
	site, err := GetSiteService().GetSiteByDomain(ctx, domain)
	if err != nil {
		return nil, fmt.Errorf("site not found")
	}

	runner := stats.NewQueryRunner()
	cvService := stats.NewCurrentVisitorsService(runner)

	return cvService.GetCurrentVisitors(ctx, fmt.Sprintf("%d", site.ID), 0)
}

// parseMetrics 解析逗号分隔的指标列表
func parseMetrics(metricsStr string) []string {
	if metricsStr == "" {
		return []string{"visitors"}
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
		return []string{"visitors"}
	}
	return result
}

// getQueryService 创建并返回查询服务实例。
func (s *StatsService) getQueryService() *stats.QueryService {
	runner := stats.NewQueryRunner()
	return stats.NewQueryService(runner)
}

// getComparisonDateRange 计算对比时间范围（上一同期）
func (s *StatsService) getComparisonDateRange(req *atypes.StatsRequest, timezone string) (time.Time, time.Time) {
	switch req.Period {
	case "day":
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

// getDateRange 根据请求参数计算日期范围
func (s *StatsService) getDateRange(req *atypes.StatsRequest, timezone string, offsetDays int) (startDate, endDate time.Time) {
	switch req.Period {
	case "day":
		date := carbon.Parse(req.Date, timezone).AddDays(offsetDays)
		startDate = date.StartOfDay().StdTime()
		endDate = date.EndOfDay().StdTime()
	case "p7":
		baseDate := carbon.Parse(req.Date, timezone).AddDays(offsetDays)
		endDate = baseDate.EndOfDay().StdTime()
		startDate = baseDate.SubDays(6).StartOfDay().StdTime()
	case "p14":
		baseDate := carbon.Parse(req.Date, timezone).AddDays(offsetDays)
		endDate = baseDate.EndOfDay().StdTime()
		startDate = baseDate.SubDays(13).StartOfDay().StdTime()
	case "p30":
		baseDate := carbon.Parse(req.Date, timezone).AddDays(offsetDays)
		endDate = baseDate.EndOfDay().StdTime()
		startDate = baseDate.SubDays(29).StartOfDay().StdTime()
	case "w":
		baseDate := carbon.Parse(req.Date, timezone).AddDays(offsetDays)
		startDate = baseDate.StartOfWeek().StartOfDay().StdTime()
		endDate = baseDate.EndOfWeek().EndOfDay().StdTime()
	case "m":
		baseDate := carbon.Parse(req.Date, timezone).AddDays(offsetDays)
		startDate = baseDate.StartOfMonth().StartOfDay().StdTime()
		endDate = baseDate.EndOfMonth().EndOfDay().StdTime()
	case "custom":
		startDate = carbon.Parse(req.From, timezone).StartOfDay().AddDays(offsetDays).StdTime()
		endDate = carbon.Parse(req.To, timezone).EndOfDay().AddDays(offsetDays).StdTime()
	}

	return
}
