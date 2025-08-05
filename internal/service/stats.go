package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"sync"

	"github.com/dromara/carbon/v2"
	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/api/types"
	cl "github.com/zenstats/zenstats/internal/store/clickhouse"
	"github.com/zenstats/zenstats/internal/store/postgresql"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent/searchengines"
	"github.com/zenstats/zenstats/pkg/globals"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

var (
	stateServiceInstance *StateService
	stateOnce            sync.Once
)

type TopStats struct {
	PV                uint64  `json:"pv"`
	UV                uint64  `json:"uv"`
	Sessions          uint64  `json:"sessions"`
	AvgDuration       float64 `json:"avg_duration"`
	PrevPV            uint64  `json:"prev_pv"`
	PrevUV            uint64  `json:"prev_uv"`
	PrevSessions      uint64  `json:"prev_sessions"`
	PrevAvgDuration   float64 `json:"prev_avg_duration"`
	PVChange          float64 `json:"pv_change"`
	UVChange          float64 `json:"uv_change"`
	SessionChange     float64 `json:"session_change"`
	AvgDurationChange float64 `json:"avg_duration_change"`
	AvgDurationFormat string  `json:"avg_duration_format"`
	BounceRate        float64 `json:"bounce_rate"`
}

type TimeRangeUV struct {
	Time string `json:"time"`
	UV   uint64 `json:"uv"`
}

type RankItem struct {
	Key        string  `json:"key"`
	Visits     uint64  `json:"visits"`
	Percentage float64 `json:"percentage"`
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

// 顶部指标数据
func (s *StateService) GetTopStats(ctx *gin.Context, domain string, req *types.StatsRequest) (*TopStats, error) {
	site, err := GetSiteService().GetSiteByDomain(ctx, domain)
	if err != nil {
		return nil, fmt.Errorf("site not found")
	}
	where, prevWhere, err := s.getWhere(ctx, site, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get where clause: %w", err)
	}
	// uv pv 数据
	query := fmt.Sprintf(`SELECT count(*) as pv, count(distinct user_id) as uv, count(distinct session_id) as sessions FROM zenstats_events_db.events WHERE %s AND name = 'pageview'`, where)
	var stats TopStats
	err = s.cl.QueryRow(ctx, query).Scan(
		&stats.PV,
		&stats.UV,
		&stats.Sessions,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	avgDurationQuery := fmt.Sprintf(`SELECT avg(duration) FROM zenstats_events_db.sessions WHERE %s`, strings.ReplaceAll(where, "timestamp", "start"))
	err = s.cl.QueryRow(ctx, avgDurationQuery).Scan(
		&stats.AvgDuration,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	if math.IsNaN(stats.AvgDuration) {
		stats.AvgDuration = 0
	}

	prevAvgDurationQuery := fmt.Sprintf(`SELECT avg(duration) FROM zenstats_events_db.sessions WHERE %s`, strings.ReplaceAll(prevWhere, "timestamp", "start"))
	err = s.cl.QueryRow(ctx, prevAvgDurationQuery).Scan(
		&stats.PrevAvgDuration,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	if math.IsNaN(stats.PrevAvgDuration) {
		stats.PrevAvgDuration = 0
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
	} else {
		stats.PVChange = 0
	}
	if stats.PrevUV > 0 {
		stats.UVChange = (float64(stats.UV) - float64(stats.PrevUV)) / float64(stats.PrevUV) * 100
	} else {
		stats.UVChange = 0
	}
	if stats.PrevSessions > 0 {
		stats.SessionChange = (float64(stats.Sessions) - float64(stats.PrevSessions)) / float64(stats.PrevSessions) * 100
	} else {
		stats.SessionChange = 0
	}
	if stats.PrevAvgDuration > 0 {
		stats.AvgDurationChange = (float64(stats.AvgDuration) - float64(stats.PrevAvgDuration)) / float64(stats.PrevAvgDuration) * 100
	} else {
		stats.AvgDurationChange = 0
	}
	stats.AvgDuration = math.Round(stats.AvgDuration*100) / 100
	stats.PVChange = math.Round(stats.PVChange*100) / 100
	stats.UVChange = math.Round(stats.UVChange*100) / 100
	stats.SessionChange = math.Round(stats.SessionChange*100) / 100
	stats.AvgDurationChange = math.Round(stats.AvgDurationChange*100) / 100
	stats.AvgDurationFormat = s.formatDuration(stats.AvgDuration)

	// 计算 跳出率
	var bounceSessions uint64
	bounceQuery := fmt.Sprintf(`SELECT count(distinct session_id) FROM zenstats_events_db.sessions WHERE %s AND is_bounce = 1`, where)
	err = s.cl.QueryRow(context.Background(), bounceQuery).Scan(&bounceSessions)
	if err != nil {
		return nil, fmt.Errorf("failed to query bounce sessions: %w", err)
	}

	// 计算跳出率（保留两位小数）
	if stats.Sessions > 0 {
		stats.BounceRate = math.Round((float64(bounceSessions)/float64(stats.Sessions))*100*100) / 100
	} else {
		stats.BounceRate = 0
	}

	return &stats, nil
}

// GetCurve 获取各个时间段的 UV 数量
func (s *StateService) GetCurve(ctx *gin.Context, domain string, req *types.StatsRequest) ([]TimeRangeUV, error) {
	site, err := GetSiteService().GetSiteByDomain(ctx, domain)
	if err != nil {
		return nil, fmt.Errorf("site not found")
	}
	where, _, err := s.getWhere(ctx, site, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get where clause: %w", err)
	}

	// 确定时间间隔
	interval := s.getInterval(req)

	query := fmt.Sprintf(`
		SELECT
			formatDateTime(toStartOfInterval(toTimeZone(timestamp, '%s'), INTERVAL %s), '%s') as time,
			count(distinct user_id) as uv
		FROM zenstats_events_db.events
		WHERE %s AND name = 'pageview'
		GROUP BY time
		ORDER BY time
	`, site.Timezone, interval, s.getFormat(interval), where)
	var timeRangeUVs []TimeRangeUV
	rows, err := s.cl.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var truv TimeRangeUV
		err = rows.Scan(&truv.Time, &truv.UV)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		timeRangeUVs = append(timeRangeUVs, truv)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	var startDate, endDate string
	if req.Period == "realtime" {
		startDate = carbon.Now(site.Timezone).SubMinutes(30).Format("Y-m-d H:i:s")
		endDate = carbon.Now(site.Timezone).Format("Y-m-d H:i:s")
	} else {
		startDate, endDate = s.getDateRange(req, site.Timezone, 0)
		if startDate != "" && endDate != "" {
			startDate = carbon.Parse(startDate).StartOfDay().Format("Y-m-d H:i:s")
			endDate = carbon.Parse(endDate).EndOfDay().Format("Y-m-d H:i:s")
		}
	}
	slot, err := s.GenerateTimeIntervals(startDate, endDate, interval, site.Timezone)
	if err != nil {
		return nil, err
	}

	// 将 timeRangeUVs 转换为 map，方便查找
	uvMap := make(map[string]uint64)
	for _, truv := range timeRangeUVs {
		uvMap[truv.Time] = truv.UV
	}

	// 重新填充 timeRangeUVs
	timeRangeUVs = make([]TimeRangeUV, 0, len(slot))
	for _, time := range slot {
		uv, exists := uvMap[time]
		if !exists {
			uv = 0
		}
		timeRangeUVs = append(timeRangeUVs, TimeRangeUV{
			Time: time,
			UV:   uv,
		})
	}

	return timeRangeUVs, nil
}

// getInterval 根据时间段长短确定时间间隔
func (s *StateService) getInterval(req *types.StatsRequest) string {

	if req.Interval != "" {
		return req.Interval
	}

	var start, end carbon.Carbon
	switch req.Period {
	case "realtime":
		start = *carbon.Now().SubMinutes(30)
		end = *carbon.Now()
	case "day":
		start = *carbon.Parse(req.Date).StartOfDay()
		end = *carbon.Parse(req.Date).EndOfDay()
	case "p7":
		start = *carbon.Parse(req.Date).SubDays(6).StartOfDay()
		end = *carbon.Parse(req.Date).EndOfDay()
	case "p14":
		start = *carbon.Parse(req.Date).SubDays(13).StartOfDay()
		end = *carbon.Parse(req.Date).EndOfDay()
	case "p30":
		start = *carbon.Parse(req.Date).SubDays(29).StartOfDay()
		end = *carbon.Parse(req.Date).EndOfDay()
	case "custom":
		start = *carbon.Parse(req.From)
		end = *carbon.Parse(req.To)
	default:
		return "1 HOUR"
	}

	duration := start.DiffInSeconds(&end)
	hours := duration / 3600

	switch {
	case hours <= 1:
		return "1 MINUTE"
	case hours <= 6:
		return "15 MINUTE"
	case hours <= 72:
		return "1 HOUR"
	case hours <= 168:
		return "1 HOUR"
	default:
		return "1 DAY"
	}
}

// getFormat 根据时间间隔确定日期格式
func (s *StateService) getFormat(interval string) string {
	switch interval {
	case "5 MINUTE", "15 MINUTE", "1 HOUR":
		return "%Y-%m-%d %H:%i"
	case "1 DAY":
		return "%Y-%m-%d"
	case "1 WEEK":
		return "%Y-%m-%d"
	default:
		return "%Y-%m-%d %H:%i"
	}
}

// GenerateTimeIntervals 生成指定时间范围内按间隔划分的时间节点列表
func (s *StateService) GenerateTimeIntervals(startTimeStr, endTimeStr, interval, timezone string) ([]string, error) {
	// 解析开始时间
	start := carbon.Parse(startTimeStr)
	if start == nil {
		return nil, fmt.Errorf("解析开始时间失败")
	}
	// 解析结束时间
	end := carbon.Parse(endTimeStr)
	if end == nil {
		return nil, fmt.Errorf("解析结束时间失败")
	}

	// 获取间隔秒数
	intervalSeconds := getIntervalSeconds(interval)
	if intervalSeconds <= 0 {
		return nil, fmt.Errorf("不支持的时间间隔: %s", interval)
	}

	// 调整开始时间到间隔起始点
	adjustedStart := adjustStartTimeToInterval(start, interval, intervalSeconds)
	if adjustedStart == nil {
		return nil, fmt.Errorf("调整开始时间失败")
	}
	start = adjustedStart

	// 生成时间节点
	var timePoints []string
	current := start.Copy()
	if current == nil {
		return nil, fmt.Errorf("无法复制当前时间")
	}
	endCopy := end.Copy()
	if endCopy == nil {
		return nil, fmt.Errorf("无法复制结束时间")
	}

	for current.Lte(endCopy) {
		// 根据间隔类型格式化时间
		format := getTimeFormatByInterval(interval)
		timePoints = append(timePoints, current.Format(format))
		next := current.AddSeconds(intervalSeconds)
		if next == nil || next.Gt(endCopy) {
			break
		}
		current = next
	}

	return timePoints, nil
}

func (s *StateService) GetDeviceRank(ctx *gin.Context, domain string, req *types.StatsRequest) ([]RankItem, error) {
	site, err := GetSiteService().GetSiteByDomain(ctx, domain)
	if err != nil {
		return nil, fmt.Errorf("site not found")
	}
	where, _, err := s.getWhere(ctx, site, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get where clause: %w", err)
	}

	query := fmt.Sprintf(`
		SELECT
			device,
			count(distinct user_id) as visits
		FROM zenstats_events_db.events
		WHERE %s AND name = 'pageview'
		GROUP BY device
		ORDER BY visits DESC
		LIMIT 10
	`, where)

	var deviceRanks []RankItem
	rows, err := s.cl.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var dr RankItem
		err := rows.Scan(&dr.Key, &dr.Visits)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		if dr.Key == "" {
			dr.Key = "Others"
		}
		deviceRanks = append(deviceRanks, dr)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	visits, _ := s.GetVisits(ctx, where)

	deviceRanks = s.handlePercentage(visits, deviceRanks)

	return deviceRanks, nil
}

func (s *StateService) GetPageRank(ctx *gin.Context, domain string, req *types.StatsRequest) ([]RankItem, error) {
	site, err := GetSiteService().GetSiteByDomain(ctx, domain)
	if err != nil {
		return nil, fmt.Errorf("site not found")
	}
	where, _, err := s.getWhere(ctx, site, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get where clause: %w", err)
	}

	query := fmt.Sprintf(`
		SELECT
			pathname,
			count(distinct user_id) as visits
		FROM zenstats_events_db.events
		WHERE %s AND name = 'pageview'
		GROUP BY pathname
		ORDER BY visits DESC
		LIMIT 20
	`, where)

	var rankItems []RankItem
	rows, err := s.cl.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var dr RankItem
		err := rows.Scan(&dr.Key, &dr.Visits)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		if dr.Key == "" {
			dr.Key = "Others"
		}
		rankItems = append(rankItems, dr)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	visits, _ := s.GetVisits(ctx, where)

	rankItems = s.handlePercentage(visits, rankItems)

	return rankItems, nil
}

// GetTrafficSourceRank 查询流量来源分布数据
func (s *StateService) GetSourceRank(ctx *gin.Context, domain string, req *types.StatsRequest) ([]RankItem, error) {
	site, err := GetSiteService().GetSiteByDomain(ctx, domain)
	if err != nil {
		return nil, fmt.Errorf("site not found")
	}
	where, _, err := s.getWhere(ctx, site, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get where clause: %w", err)
	}

	// 定义搜索引擎列表
	searchEngines := s.db.Client.SearchEngines.Query().AllX(ctx)

	// 构建查询语句
	searchEngineConditions := []string{}
	for _, searchEngine := range searchEngines {
		searchEngineConditions = append(searchEngineConditions, fmt.Sprintf("positionCaseInsensitive(referrer_source, '%s') > 0", searchEngine.Domain))
	}

	searchEngineCondition := strings.Join(searchEngineConditions, " OR ")

	query := fmt.Sprintf(`
		SELECT
			CASE
				WHEN referrer_source = '' THEN 'Direct'
				WHEN %s THEN
					CASE
						%s
						ELSE 'Other'
					END
				ELSE referrer_source
			END as source,
			count(distinct user_id) as visits
		FROM zenstats_events_db.events
		WHERE %s AND name = 'pageview'
		GROUP BY source
		ORDER BY visits DESC
		LIMIT 10
	`, searchEngineCondition, s.buildSearchEngineCase(searchEngines), where)
	var trafficSourceRanks []RankItem
	rows, err := s.cl.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var tsr RankItem
		err := rows.Scan(&tsr.Key, &tsr.Visits)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		trafficSourceRanks = append(trafficSourceRanks, tsr)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	visits, _ := s.GetVisits(ctx, where)

	trafficSourceRanks = s.handlePercentage(visits, trafficSourceRanks)

	return trafficSourceRanks, nil
}

// GetVisits 查询符合条件的独立访客数量
func (s *StateService) GetVisits(ctx *gin.Context, where string) (uint64, error) {
	var visits uint64
	query := fmt.Sprintf(`
		SELECT
			count(distinct user_id) as visits
		FROM zenstats_events_db.events
		WHERE %s AND name = 'pageview'
	`, where)

	err := s.cl.QueryRow(context.Background(), query).Scan(
		&visits,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to execute query: %w", err)
	}

	return visits, nil
}

// getDateRange 根据请求周期计算开始和结束日期
// getDateRange 根据请求周期和偏移天数计算开始和结束日期
// offsetDays: 日期偏移天数，正数为未来日期，负数为过去日期
func (s *StateService) getDateRange(req *types.StatsRequest, timezone string, offsetDays int) (startDate, endDate string) {
	switch req.Period {
	case "day":
		date := carbon.Parse(req.Date, timezone).AddDays(offsetDays)
		startDate = date.Format("Y-m-d")
		endDate = startDate
	case "p7":
		baseDate := carbon.Parse(req.Date, timezone).AddDays(offsetDays)
		endDate = baseDate.Format("Y-m-d")
		startDate = baseDate.SubDays(6).Format("Y-m-d")
	case "p14":
		baseDate := carbon.Parse(req.Date, timezone).AddDays(offsetDays)
		endDate = baseDate.Format("Y-m-d")
		startDate = baseDate.SubDays(13).Format("Y-m-d")
	case "p30":
		baseDate := carbon.Parse(req.Date, timezone).AddDays(offsetDays)
		endDate = baseDate.Format("Y-m-d")
		startDate = baseDate.SubDays(29).Format("Y-m-d")
	case "custom":
		startDate = carbon.Parse(req.From, timezone).AddDays(offsetDays).Format("Y-m-d")
		endDate = carbon.Parse(req.To, timezone).AddDays(offsetDays).Format("Y-m-d")
	default:
		startDate = ""
		endDate = ""
	}

	return
}

// buildSearchEngineCase 构建搜索引擎的 CASE 语句
func (s *StateService) buildSearchEngineCase(searchEngines []*ent.SearchEngines) string {
	var conditions []string
	for _, searchEngine := range searchEngines {
		conditions = append(conditions, fmt.Sprintf("WHEN positionCaseInsensitive(referrer_source, '%s') > 0 THEN '%s'", searchEngine.Domain, searchEngine.Name))
	}
	return strings.Join(conditions, "\n")
}

// formatDuration 格式化持续时间格式
func (s *StateService) formatDuration(duration float64) string {
	if duration == 0 {
		return "0S"
	}

	hours := int(duration / 3600)
	remainingSeconds := math.Mod(duration, 3600)

	minutes := int(remainingSeconds / 60)
	seconds := math.Mod(remainingSeconds, 60)

	var result string

	if hours > 0 {
		result += fmt.Sprintf("%dH", hours)
	}
	if minutes > 0 || hours > 0 {
		result += fmt.Sprintf(" %dM", minutes)
	}
	result += fmt.Sprintf(" %.1fS", seconds)

	if seconds == float64(int(seconds)) {
		result = fmt.Sprintf("%s%dS", result[:len(result)-3], int(seconds))
	}

	return result
}

// handlePercentage 计算每个项目的占比
func (s *StateService) handlePercentage(visits uint64, items []RankItem) []RankItem {
	// 修改每个 item 的 Percentage
	for i := range items {
		if visits > 0 {
			items[i].Percentage = math.Round(float64(items[i].Visits)/float64(visits)*100*100) / 100
		} else {
			items[i].Percentage = 0
		}
	}

	return items
}

// getWhere 构建 WHERE 子句
func (s *StateService) getWhere(ctx context.Context, site *ent.Site, req *types.StatsRequest) (string, string, error) {

	// where := fmt.Sprintf(" site_id = %d", site.ID)
	// prevWhere := fmt.Sprintf(" site_id = %d", site.ID)

	where := fmt.Sprintf(" site_id = %d", 1)
	prevWhere := fmt.Sprintf(" site_id = %d", 1)
	// 周期
	period, prevPeriod, err := s.getPeriod(site, req)
	if err != nil {
		return "", "", err
	}
	where += fmt.Sprintf(" AND %s", period)
	prevWhere += fmt.Sprintf(" AND %s", prevPeriod)
	// 筛选
	filter, err := s.getFilter(ctx, req)
	if err != nil {
		return "", "", err
	}
	if filter != "" {
		where += fmt.Sprintf(" AND (%s)", filter)
		prevWhere += fmt.Sprintf(" AND (%s)", filter)
	}

	return where, prevWhere, nil
}

// getPeriod 根据请求周期计算 WHERE 子句
func (s *StateService) getPeriod(site *ent.Site, req *types.StatsRequest) (string, string, error) {
	var where, prevWhere string
	switch req.Period {
	case "realtime":
		// 实时数据
		where = fmt.Sprintf(" toTimeZone(timestamp, '%s') >= now() - INTERVAL 30 MINUTE ", site.Timezone)
		prevWhere = fmt.Sprintf(" toTimeZone(timestamp, '%s') BETWEEN now() - INTERVAL 60 MINUTE AND now() - INTERVAL 30 MINUTE ", site.Timezone)
	case "day":
		startDate, _ := s.getDateRange(req, site.Timezone, 0)
		where = fmt.Sprintf(" toDate(timestamp, '%s') = '%s' ", site.Timezone, startDate)
		prevStartDate, _ := s.getDateRange(req, site.Timezone, -1)
		prevWhere = fmt.Sprintf(" toDate(timestamp, '%s') = '%s' ", site.Timezone, prevStartDate)
	case "p7":
		startDate, endDate := s.getDateRange(req, site.Timezone, 0)
		where = fmt.Sprintf(" toDate(timestamp, '%s') BETWEEN '%s' AND '%s' ", site.Timezone, startDate, endDate)

		prevStartDate, prevEndDate := s.getDateRange(req, site.Timezone, -7)
		prevWhere = fmt.Sprintf(" toDate(timestamp, '%s') BETWEEN '%s' AND '%s' ", site.Timezone, prevStartDate, prevEndDate)
	case "p14":
		startDate, endDate := s.getDateRange(req, site.Timezone, 0)
		where = fmt.Sprintf(" toDate(timestamp, '%s') BETWEEN '%s' AND '%s' ", site.Timezone, startDate, endDate)

		prevStartDate, prevEndDate := s.getDateRange(req, site.Timezone, -14)
		prevWhere = fmt.Sprintf(" toDate(timestamp, '%s') BETWEEN '%s' AND '%s' ", site.Timezone, prevStartDate, prevEndDate)
	case "p30":
		startDate, endDate := s.getDateRange(req, site.Timezone, 0)
		where = fmt.Sprintf(" toDate(timestamp, '%s') BETWEEN '%s' AND '%s' ", site.Timezone, startDate, endDate)

		prevStartDate, prevEndDate := s.getDateRange(req, site.Timezone, -30)
		prevWhere = fmt.Sprintf(" toDate(timestamp, '%s') BETWEEN '%s' AND '%s' ", site.Timezone, prevStartDate, prevEndDate)
	case "custom":
		startDate, endDate := s.getDateRange(req, site.Timezone, 0)
		where = fmt.Sprintf(" toDate(timestamp, '%s') BETWEEN '%s' AND '%s' ", site.Timezone, startDate, endDate)
		// 计算上一个周期的日期范围
		days := carbon.Parse(endDate, site.Timezone).AddDays(1).DiffInDays(carbon.Parse(startDate, site.Timezone))
		// 计算上一个周期的偏移天数
		prevOffsetDays := -int(math.Abs(float64(days)))
		prevStartDate, prevEndDate := s.getDateRange(req, site.Timezone, prevOffsetDays)
		prevWhere = fmt.Sprintf(" toDate(timestamp, '%s') BETWEEN '%s' AND '%s' ", site.Timezone, prevStartDate, prevEndDate)
	default:
		return "", "", errors.New("invalid period")
	}

	return where, prevWhere, nil
}

// getFilter 根据请求筛选条件计算 WHERE 子句
func (s *StateService) getFilter(ctx context.Context, req *types.StatsRequest) (string, error) {
	if len(req.Filters) == 0 {
		return "", nil
	}
	operator := "AND"
	var conditions []string

	for _, filter := range req.Filters {
		if filterStr, ok := filter.(string); ok {
			if filterStr != "and" && filterStr != "or" {
				return "", fmt.Errorf("invalid filter operator: %s", filterStr)
			}
			operator = strings.ToUpper(filterStr)
			continue
		}

		condition, err := s.parseFilter(ctx, filter)
		if err != nil {
			return "", err
		}
		if condition != "" {
			conditions = append(conditions, condition)
		}
	}

	if len(conditions) == 0 {
		return "", nil
	}

	// 顶级条件默认使用 AND 组合
	return strings.Join(conditions, " "+operator+" "), nil
}

// parseFilter 递归解析过滤条件，支持 AND/OR 逻辑运算符
func (s *StateService) parseFilter(ctx context.Context, filter any) (string, error) {

	filterArr, ok := filter.([]any)
	if !ok {
		return "", fmt.Errorf("invalid filter type: expected array, got %T", filter)
	}

	if len(filterArr) == 0 {
		return "", fmt.Errorf("empty filter array")
	}

	// 检查是否为逻辑运算符 (and/or)
	operator, ok := filterArr[0].(string)
	if ok && (operator == "and" || operator == "or") {
		if len(filterArr) != 2 {
			return "", fmt.Errorf("invalid %s operator format: expected [operator, conditions]", operator)
		}
		// 获取嵌套条件数组
		nestedConditions, nestedOk := filterArr[1].([]any)
		if !nestedOk {
			return "", fmt.Errorf("invalid %s conditions type: expected array, got %T", operator, filterArr[1])
		}

		// 解析每个嵌套条件
		var parsedConditions []string
		for _, nestedFilter := range nestedConditions {
			parsedCondition, err := s.parseFilter(ctx, nestedFilter)
			if err != nil {
				return "", err
			}
			if parsedCondition != "" {
				parsedConditions = append(parsedConditions, parsedCondition)
			}
		}

		if len(parsedConditions) == 0 {
			return "", nil
		}

		// 使用适当的逻辑运算符组合条件
		logicalOperator := "AND"
		if operator == "or" {
			logicalOperator = "OR"
		}

		return fmt.Sprintf(" (%s) ", strings.Join(parsedConditions, " "+logicalOperator+" ")), nil
	}

	// 处理简单过滤器 [operator, dimension, clauses]
	if len(filterArr) != 3 {
		return "", fmt.Errorf("invalid filter format: expected [operator, dimension, clauses], got %v", filterArr)
	}

	// 解析简单过滤器组件
	filterOperator, ok := filterArr[0].(string)
	if !ok {
		return "", fmt.Errorf("invalid operator type: expected string, got %T", filterArr[0])
	}

	dimension, ok := filterArr[1].(string)
	if !ok {
		return "", fmt.Errorf("invalid dimension type: expected string, got %T", filterArr[1])
	}

	clauses := filterArr[2]
	clausesArr, isArray := clauses.([]interface{})
	if !isArray {
		// 如果不是数组，包装成单元素数组
		clausesArr = []interface{}{clauses}
	}

	// 转义数组中的每个值
	escapedClauses := make([]string, len(clausesArr))
	for i, v := range clausesArr {
		escaped, err := s.escapeClauses(v)
		if err != nil {
			return "", err
		}
		escapedClauses[i] = escaped
	}

	// 将 API 属性映射到数据库列
	column, err := s.mapDimensionToColumn(ctx, dimension, escapedClauses)
	if err != nil {
		return "", err
	}
	// 根据运算符生成条件
	return s.generateCondition(filterOperator, column, escapedClauses)
}

// mapDimensionToColumn converts API dimension to database column
func (s *StateService) mapDimensionToColumn(ctx context.Context, dimension string, clauses []string) (string, error) {
	if strings.HasPrefix(dimension, "event:props:") {
		propName := strings.TrimPrefix(dimension, "event:props:")
		if !regexp.MustCompile(`^[a-zA-Z0-9_]+$`).MatchString(propName) {
			return "", fmt.Errorf("invalid custom property name: %s", propName)
		}
		if len(clauses) == 1 {
			return fmt.Sprintf("arrayExists(pair -> pair.1 = '%s' AND pair.2 = %%s, arrayZip(meta.key, meta.value))", propName), nil
		}
		return fmt.Sprintf("arrayExists(pair -> pair.1 = '%s' AND pair.2 IN (%%s), arrayZip(meta.key, meta.value))", propName), nil
	}

	if dimension == "visit:source" {
		if len(clauses) != 1 {
			return "", fmt.Errorf("invalid visit:source clauses")
		}
		// 查询所有匹配的搜索引擎数据
		searchEngines := s.db.Client.SearchEngines.Query().Where(searchengines.NameEQ(strings.Trim(clauses[0], "'"))).AllX(context.Background())
		if len(searchEngines) > 0 {
			// 构建包含所有域名的查询条件
			var conditions []string
			for _, engine := range searchEngines {
				conditions = append(conditions, fmt.Sprintf("position(source, '%s') > 0", engine.Domain))
			}
			// 使用 OR 连接所有条件
			return strings.Join(conditions, " OR "), nil
		}
	}

	// 标准维度映射
	dimensionMap := map[string]string{
		"visit:country_name":   "country_name",
		"visit:region_name":    "region_name",
		"visit:city_name":      "city_name",
		"visit:continent_name": "continent_name",
		"visit:source":         "source",
		"event:medium":         "medium",
		"event:campaign":       "campaign",
		"event:term":           "term",
		"event:content":        "content",
		"event:browser":        "browser",
		"event:os":             "os",
		"event:device":         "device",
		"event:channel":        "channel",
		"event:device_type":    "device_type",
		"event:page":           "pathname",
	}

	column, ok := dimensionMap[dimension]
	if !ok {
		// 尝试直接使用维度作为列名（适用于未明确定义映射但数据库中存在的列）
		if regexp.MustCompile(`^[a-zA-Z0-9_]+$`).MatchString(dimension) {
			return dimension, nil
		}
		return "", fmt.Errorf("unsupported dimension: %s", dimension)
	}

	return column, nil
}

// escapeClauses handles clauses escaping based on type
func (s *StateService) escapeClauses(clauses any) (string, error) {
	// Implement proper escaping based on clauses type
	switch v := clauses.(type) {
	case string:
		// SQL注入防护 - 转义单引号和其他特殊字符
		escaped := strings.ReplaceAll(v, "'", "''")
		escaped = strings.ReplaceAll(escaped, "\\", "\\\\")
		escaped = strings.ReplaceAll(escaped, ";", ";")
		escaped = strings.ReplaceAll(escaped, "--", "--")
		return fmt.Sprintf("'%s'", escaped), nil
	case []any:
		// Handle array values
		var escapedClauses []string
		for _, item := range v {
			escapedItem, err := s.escapeClauses(item)
			if err != nil {
				return "", err
			}
			escapedClauses = append(escapedClauses, escapedItem)
		}
		return fmt.Sprintf("(%s)", strings.Join(escapedClauses, ",")), nil
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return fmt.Sprintf("%v", v), nil
	case bool:
		if v {
			return "1", nil
		}
		return "0", nil
	default:
		// 其他类型转换为字符串并转义
		strValue := fmt.Sprintf("%v", v)
		escaped := strings.ReplaceAll(strValue, "'", "''")
		escaped = strings.ReplaceAll(escaped, "\\", "\\\\")
		return fmt.Sprintf("'%s'", escaped), nil
	}
}

// props
// func (s *StateService) props(key, value string) string {
// 	return fmt.Sprintf("arrayExists(pair -> pair.1 = '%s' AND pair.2 = '%s', arrayZip(meta.key, meta.value))", key, value)
// }

// generateCondition 根据运算符和值生成SQL条件
func (s *StateService) generateCondition(operator, column string, clauses []string) (string, error) {

	// 处理arrayExists模板
	if strings.Contains(column, "arrayExists") {
		if len(clauses) == 0 {
			return "", fmt.Errorf("arrayExists条件需要至少包含一个值")
		}
		return fmt.Sprintf(column, strings.Join(clauses, ", ")), nil
	}

	// 根据运算符生成SQL条件
	switch operator {
	case "is":
		if len(clauses) == 1 {
			// 特殊处理NULL值
			if strings.ToLower(clauses[0]) == "'null'" {
				return fmt.Sprintf("%s IS NULL", column), nil
			}
			return fmt.Sprintf("%s = %s", column, clauses[0]), nil
		}
		return fmt.Sprintf("%s IN (%s)", column, strings.Join(clauses, ", ")), nil

	case "is_not":
		if len(clauses) == 1 {
			// 特殊处理NULL值
			if strings.ToLower(clauses[0]) == "'null'" {
				return fmt.Sprintf("%s IS NOT NULL", column), nil
			}
			return fmt.Sprintf("%s != %s", column, clauses[0]), nil
		}
		return fmt.Sprintf("%s NOT IN (%s)", column, strings.Join(clauses, ", ")), nil

	case "contains":
		conditions := make([]string, len(clauses))
		for i, val := range clauses {
			conditions[i] = fmt.Sprintf("position(%s, %s) > 0", column, val)
		}
		return strings.Join(conditions, " OR "), nil

	case "contains_not":
		conditions := make([]string, len(clauses))
		for i, val := range clauses {
			conditions[i] = fmt.Sprintf("position(%s, %s) = 0", column, val)
		}
		return strings.Join(conditions, " AND "), nil

	case "matches":
		if len(clauses) != 1 {
			return "", fmt.Errorf("matches operator requires exactly one regex value")
		}
		return fmt.Sprintf("match(%s, %s)", column, clauses[0]), nil

	case "matches_not":
		if len(clauses) != 1 {
			return "", fmt.Errorf("matches_not operator requires exactly one regex value")
		}
		return fmt.Sprintf("NOT match(%s, %s)", column, clauses[0]), nil

	case "gt", "greater_than":
		if len(clauses) != 1 {
			return "", fmt.Errorf("%s operator requires exactly one value", operator)
		}
		return fmt.Sprintf("%s > %s", column, clauses[0]), nil

	case "lt", "less_than":
		if len(clauses) != 1 {
			return "", fmt.Errorf("%s operator requires exactly one value", operator)
		}
		return fmt.Sprintf("%s < %s", column, clauses[0]), nil

	case "gte", "greater_than_or_equal":
		if len(clauses) != 1 {
			return "", fmt.Errorf("%s operator requires exactly one value", operator)
		}
		return fmt.Sprintf("%s >= %s", column, clauses[0]), nil

	case "lte", "less_than_or_equal":
		if len(clauses) != 1 {
			return "", fmt.Errorf("%s operator requires exactly one value", operator)
		}
		return fmt.Sprintf("%s <= %s", column, clauses[0]), nil

	case "between":
		if len(clauses) != 2 {
			return "", fmt.Errorf("between operator requires exactly two values")
		}
		return fmt.Sprintf("%s BETWEEN %s AND %s", column, clauses[0], clauses[1]), nil

	default:
		return "", fmt.Errorf("unsupported operator: %s", operator)
	}
}
