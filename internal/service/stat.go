package service

import (
	"fmt"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/dromara/carbon/v2"
	"github.com/gin-gonic/gin"
	atypes "github.com/zenstats/zenstats/internal/api/types"
	"github.com/zenstats/zenstats/internal/service/stats"
	ss "github.com/zenstats/zenstats/internal/service/stats"
	"github.com/zenstats/zenstats/internal/service/stats/types"
	cl "github.com/zenstats/zenstats/internal/store/clickhouse"
	"github.com/zenstats/zenstats/internal/store/postgresql"
	"github.com/zenstats/zenstats/pkg/globals"
)

var (
	statsServiceInstance *StatsService
	statsOnce            sync.Once
)

type StatsService struct {
	db *postgresql.Client
	cl driver.Conn
}

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

// GetSourceRank 获取来源排名统计
func (s *StatsService) GetSourceRank(ctx *gin.Context, domain string, req *atypes.StatsRequest) (*types.QueryResult, error) {
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

	// 构建查询条件
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
		Metrics:    []string{"visitors"},
		Dimensions: []string{"visit:source"},
		Filters:    filters,
		Pagination: &types.Pagination{
			Limit:  req.Limit,
			Offset: offset,
		},
	}
	qs := s.getQueryServicr()

	query, err := qs.CreateQuery(params)
	if err != nil {
		return nil, err
	}
	psite := &types.Site{ID: query.SiteID, Timezone: query.Timezone}

	return qs.Execute(ctx, query, psite)
}

func (s *StatsService) GetAggregate(ctx *gin.Context, domain string, req *atypes.StatsRequest) (*stats.AggregateResult, error) {
	site, err := GetSiteService().GetSiteByDomain(ctx, domain)
	if err != nil {
		return nil, fmt.Errorf("site not found")
	}
	start, end := s.getDateRange(req, site.Timezone, 0)

	// 构建查询条件
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
	qs := s.getQueryServicr()

	aggregateService := stats.NewAggregateService(qs)

	return aggregateService.GetAggregate(ctx, params)
}

func (s *StatsService) GetTimeSeries(ctx *gin.Context, domain string, req *atypes.StatsRequest) ([]stats.TimeSeriesPoint, error) {
	site, err := GetSiteService().GetSiteByDomain(ctx, domain)
	if err != nil {
		return nil, fmt.Errorf("site not found")
	}
	start, end := s.getDateRange(req, site.Timezone, 0)

	// 构建查询条件
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
	qs := s.getQueryServicr()

	aggregateService := stats.NewAggregateService(qs)

	return aggregateService.GetTimeSeries(ctx, params)
}

func (s *StatsService) getQueryServicr() *stats.QueryService {
	runner := ss.NewQueryRunner()
	return ss.NewQueryService(runner)
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
