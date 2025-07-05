package service

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"

	"github.com/dromara/carbon/v2"
	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/api/types"
	cl "github.com/zenstats/zenstats/internal/store/clickhouse"
	"github.com/zenstats/zenstats/internal/store/postgresql"
	"github.com/zenstats/zenstats/pkg/globals"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

var (
	stateServiceInstance *StateService
	stateOnce            sync.Once
)

type TopStats struct {
	PV            uint64 //pv
	UV            uint64 //uv
	Sessions      uint64 // total_visitors
	PrevPV        uint64
	PrevUV        uint64
	PrevSessions  uint64
	PVChange      float64
	UVChange      float64
	SessionChange float64
	AvgDuration   float64
}

type StateService struct {
	db *postgresql.Client
	cl driver.Conn
}

func GetStateService() *StateService {
	stateOnce.Do(func() {
		db := globals.GetDB()
		if db == nil {
			panic("DB is not initialized")
		}
		stateServiceInstance = &StateService{
			db: db,
			cl: cl.GetConnection(),
		}
	})
	return stateServiceInstance
}

func (s *StateService) GetTopStats(ctx *gin.Context, domain string, req *types.TopStatsRequest) (*TopStats, error) {
	site, err := GetSiteService().GetSiteByDomain(ctx, domain)
	if err != nil {
		return nil, fmt.Errorf("site not found")
	}
	// time range
	where := fmt.Sprintf(" site_id = %d", site.ID)
	var prevWhere string
	switch req.Period {
	case "R":
		// 实时数据
		where += fmt.Sprintf(" AND toTimeZone(timestamp, '%s') >= now() - INTERVAL 30 MINUTE ", site.Timezone)

		prevWhere = fmt.Sprintf(" site_id = %d AND toTimeZone(timestamp, '%s') BETWEEN now() - INTERVAL 60 MINUTE AND now() - INTERVAL 30 MINUTE ", 0, site.Timezone)
	case "T":
		where += fmt.Sprintf(" AND toDate(timestamp, '%s') = '%s' ", site.Timezone, req.Date)

		prevDate := carbon.Parse(req.Date, site.Timezone).SubDays(1).Format("Y-m-d")
		prevWhere = fmt.Sprintf(" site_id = %d AND toDate(timestamp, '%s') = '%s' ", 0, site.Timezone, prevDate)
	case "Y":
		date := carbon.Parse(req.Date, site.Timezone).SubDays(1).Format("Y-m-d")
		where += fmt.Sprintf(" AND toDate(timestamp, '%s') = '%s' ", site.Timezone, date)

		prevDate := carbon.Parse(req.Date, site.Timezone).SubDays(2).Format("Y-m-d")
		prevWhere = fmt.Sprintf(" site_id = %d AND toDate(timestamp, '%s') = '%s' ", 0, site.Timezone, prevDate)
	case "p7":
		startDate := carbon.Parse(req.Date, site.Timezone).SubDays(6).Format("Y-m-d")
		endDate := carbon.Parse(req.Date, site.Timezone).Format("Y-m-d")
		where += fmt.Sprintf(" AND toDate(timestamp, '%s') BETWEEN '%s' AND '%s' ", site.Timezone, startDate, endDate)

		prevStartDate := carbon.Parse(req.Date, site.Timezone).SubDays(13).Format("Y-m-d")
		prevEndDate := carbon.Parse(req.Date, site.Timezone).SubDays(7).Format("Y-m-d")
		prevWhere = fmt.Sprintf(" site_id = %d AND toDate(timestamp, '%s') BETWEEN '%s' AND '%s' ", 0, site.Timezone, prevStartDate, prevEndDate)
	case "p14":
		startDate := carbon.Parse(req.Date, site.Timezone).SubDays(13).Format("Y-m-d")
		endDate := carbon.Parse(req.Date, site.Timezone).Format("Y-m-d")
		where += fmt.Sprintf(" AND toDate(timestamp, '%s') BETWEEN '%s' AND '%s' ", site.Timezone, startDate, endDate)

		prevStartDate := carbon.Parse(req.Date, site.Timezone).SubDays(27).Format("Y-m-d")
		prevEndDate := carbon.Parse(req.Date, site.Timezone).SubDays(14).Format("Y-m-d")
		prevWhere = fmt.Sprintf(" site_id = %d AND toDate(timestamp, '%s') BETWEEN '%s' AND '%s' ", 0, site.Timezone, prevStartDate, prevEndDate)
	case "p30":
		startDate := carbon.Parse(req.Date, site.Timezone).SubDays(29).Format("Y-m-d")
		endDate := carbon.Parse(req.Date, site.Timezone).Format("Y-m-d")
		where += fmt.Sprintf(" AND toDate(timestamp, '%s') BETWEEN '%s' AND '%s' ", site.Timezone, startDate, endDate)

		prevStartDate := carbon.Parse(req.Date, site.Timezone).SubDays(59).Format("Y-m-d")
		prevEndDate := carbon.Parse(req.Date, site.Timezone).SubDays(30).Format("Y-m-d")
		prevWhere = fmt.Sprintf(" site_id = %d AND toDate(timestamp, '%s') BETWEEN '%s' AND '%s' ", 0, site.Timezone, prevStartDate, prevEndDate)
	case "cr": // custom range
		startDate := carbon.Parse(req.StartDate, site.Timezone).Format("Y-m-d")
		endDate := carbon.Parse(req.EndDate, site.Timezone).Format("Y-m-d")
		where += fmt.Sprintf(" AND toDate(timestamp, '%s') BETWEEN '%s' AND '%s' ", site.Timezone, startDate, endDate)
		// 计算上一个周期的时间范围
		days := carbon.Parse(endDate, site.Timezone).DiffInDays(carbon.Parse(startDate, site.Timezone))
		prevStartDate := carbon.Parse(startDate, site.Timezone).SubDays(int(days)).Format("Y-m-d")
		prevEndDate := carbon.Parse(startDate, site.Timezone).SubDays(1).Format("Y-m-d")
		prevWhere = fmt.Sprintf(" site_id = %d AND toDate(timestamp, '%s') BETWEEN '%s' AND '%s' ", 0, site.Timezone, prevStartDate, prevEndDate)
	default:
		return nil, fmt.Errorf("invalid period")
	}

	// uv pv 数据
	query := fmt.Sprintf(`SELECT count(*) as pv, count(distinct user_id) as uv, count(distinct session_id) as sessions FROM zenstats_events_db.events WHERE %s AND name = 'pageview'`, where)
	var stats TopStats
	err = s.cl.QueryRow(context.Background(), query).Scan(
		&stats.PV,
		&stats.UV,
		&stats.Sessions,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	avgDurationQuery := fmt.Sprintf(`SELECT avg(duration) FROM zenstats_events_db.sessions WHERE %s`, strings.ReplaceAll(where, "timestamp", "start"))
	err = s.cl.QueryRow(context.Background(), avgDurationQuery).Scan(
		&stats.AvgDuration,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	// 获取上一个周期的对比数据
	prevQuery := fmt.Sprintf(`SELECT count(*) as pv, count(distinct user_id) as uv, count(distinct session_id) as sessions FROM zenstats_events_db.events WHERE %s AND name = 'pageview'`, prevWhere)
	err = s.cl.QueryRow(context.Background(), prevQuery).Scan(
		&stats.PrevPV,
		&stats.PrevUV,
		&stats.PrevSessions,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to execute prev query: %w", err)
	}

	// 计算变化值
	if stats.PrevPV > 0 {
		stats.PVChange = (float64(stats.PV) - float64(stats.PrevPV)) / float64(stats.PrevPV) * 100

	}
	if stats.PrevUV > 0 {
		stats.UVChange = (float64(stats.UV) - float64(stats.PrevUV)) / float64(stats.PrevUV) * 100

	}
	if stats.PrevSessions > 0 {
		stats.SessionChange = (float64(stats.Sessions) - float64(stats.PrevSessions)) / float64(stats.PrevSessions) * 100

	}
	stats.AvgDuration = math.Round(stats.AvgDuration*100) / 100
	stats.PVChange = math.Round(stats.PVChange*100) / 100
	stats.UVChange = math.Round(stats.UVChange*100) / 100
	stats.SessionChange = math.Round(stats.SessionChange*100) / 100

	return &stats, nil
}
