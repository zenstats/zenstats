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
	"github.com/zenstats/zenstats/internal/store/postgresql/ent"
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
	where, prevWhere := s.getTimestampWhere(site, req)

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

	if math.IsNaN(stats.AvgDuration) {
		stats.AvgDuration = 0
	}

	prevAvgDurationQuery := fmt.Sprintf(`SELECT avg(duration) FROM zenstats_events_db.sessions WHERE %s`, strings.ReplaceAll(prevWhere, "timestamp", "start"))
	err = s.cl.QueryRow(context.Background(), prevAvgDurationQuery).Scan(
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

// GetMetaStats 获取带meta筛选条件的统计数据
func (s *StateService) GetMetaStats(ctx *gin.Context, domain string, req *types.StatsRequest, meta map[string]string) (*TopStats, error) {
	site, err := GetSiteService().GetSiteByDomain(ctx, domain)
	if err != nil {
		return nil, fmt.Errorf("site not found")
	}
	s.getWhereWithMeta(site, req)

	return nil, nil
}

// GetCurve 获取各个时间段的 UV 数量
func (s *StateService) GetCurve(ctx *gin.Context, domain string, req *types.StatsRequest) ([]TimeRangeUV, error) {
	site, err := GetSiteService().GetSiteByDomain(ctx, domain)
	if err != nil {
		return nil, fmt.Errorf("site not found")
	}
	where, _ := s.getTimestampWhere(site, req)

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
	rows, err := s.cl.Query(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var truv TimeRangeUV
		err := rows.Scan(&truv.Time, &truv.UV)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		timeRangeUVs = append(timeRangeUVs, truv)
	}

	if err := rows.Err(); err != nil {
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
		fmt.Println("error:", err.Error())
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
	where, _ := s.getTimestampWhere(site, req)

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
	rows, err := s.cl.Query(context.Background(), query)
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
	where, _ := s.getTimestampWhere(site, req)

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
	rows, err := s.cl.Query(context.Background(), query)
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
	where, _ := s.getTimestampWhere(site, req)

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
	rows, err := s.cl.Query(context.Background(), query)
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

func (s *StateService) getTimestampWhere(site *ent.Site, req *types.StatsRequest) (string, string) {

	// time range
	// where := fmt.Sprintf(" site_id = %d", site.ID)
	// prevWhere := fmt.Sprintf(" site_id = %d", site.ID)
	where, prevWhere := " 1 ", " 1 "

	switch req.Period {
	case "realtime":
		// 实时数据
		where += fmt.Sprintf(" AND toTimeZone(timestamp, '%s') >= now() - INTERVAL 30 MINUTE ", site.Timezone)
		prevWhere += fmt.Sprintf(" AND toTimeZone(timestamp, '%s') BETWEEN now() - INTERVAL 60 MINUTE AND now() - INTERVAL 30 MINUTE ", site.Timezone)
	case "day":
		startDate, _ := s.getDateRange(req, site.Timezone, 0)
		where += fmt.Sprintf(" AND toDate(timestamp, '%s') = '%s' ", site.Timezone, startDate)
		prevStartDate, _ := s.getDateRange(req, site.Timezone, -1)
		prevWhere += fmt.Sprintf(" AND toDate(timestamp, '%s') = '%s' ", site.Timezone, prevStartDate)
	case "p7":
		startDate, endDate := s.getDateRange(req, site.Timezone, 0)
		where += fmt.Sprintf(" AND toDate(timestamp, '%s') BETWEEN '%s' AND '%s' ", site.Timezone, startDate, endDate)

		prevStartDate, prevEndDate := s.getDateRange(req, site.Timezone, -7)
		prevWhere += fmt.Sprintf(" AND toDate(timestamp, '%s') BETWEEN '%s' AND '%s' ", site.Timezone, prevStartDate, prevEndDate)
	case "p14":
		startDate, endDate := s.getDateRange(req, site.Timezone, 0)
		where += fmt.Sprintf(" AND toDate(timestamp, '%s') BETWEEN '%s' AND '%s' ", site.Timezone, startDate, endDate)

		prevStartDate, prevEndDate := s.getDateRange(req, site.Timezone, -14)
		prevWhere += fmt.Sprintf(" AND toDate(timestamp, '%s') BETWEEN '%s' AND '%s' ", site.Timezone, prevStartDate, prevEndDate)
	case "p30":
		startDate, endDate := s.getDateRange(req, site.Timezone, 0)
		where += fmt.Sprintf(" AND toDate(timestamp, '%s') BETWEEN '%s' AND '%s' ", site.Timezone, startDate, endDate)

		prevStartDate, prevEndDate := s.getDateRange(req, site.Timezone, -30)
		prevWhere += fmt.Sprintf(" AND toDate(timestamp, '%s') BETWEEN '%s' AND '%s' ", site.Timezone, prevStartDate, prevEndDate)
	case "custom":
		startDate, endDate := s.getDateRange(req, site.Timezone, 0)
		where += fmt.Sprintf(" AND toDate(timestamp, '%s') BETWEEN '%s' AND '%s' ", site.Timezone, startDate, endDate)
		// 计算上一个周期的日期范围
		days := carbon.Parse(endDate, site.Timezone).AddDays(1).DiffInDays(carbon.Parse(startDate, site.Timezone))
		// 计算上一个周期的偏移天数
		prevOffsetDays := -int(math.Abs(float64(days)))
		prevStartDate, prevEndDate := s.getDateRange(req, site.Timezone, prevOffsetDays)
		prevWhere += fmt.Sprintf(" AND toDate(timestamp, '%s') BETWEEN '%s' AND '%s' ", site.Timezone, prevStartDate, prevEndDate)
	default:
		return "", ""
	}

	return where, prevWhere
}

// getWhereWithMeta 获取带meta条件的查询条件
func (s *StateService) getWhereWithMeta(site *ent.Site, req *types.StatsRequest) string {
	where, _ := s.getTimestampWhere(site, req)
	// 添加meta过滤条件
	metaWhere := fmt.Sprintf("arrayExists(pair -> pair.1 = '%s' AND pair.2 = '%s', arrayZip(meta.key, meta.value))", req.MetaKey, req.MetaValue)

	return where + " AND " + metaWhere
}

// buildSearchEngineCase 构建搜索引擎的 CASE 语句
func (s *StateService) buildSearchEngineCase(searchEngines []*ent.SearchEngines) string {
	var conditions []string
	for _, searchEngine := range searchEngines {
		conditions = append(conditions, fmt.Sprintf("WHEN positionCaseInsensitive(referrer_source, '%s') > 0 THEN '%s'", searchEngine.Domain, searchEngine.Name))
	}
	return strings.Join(conditions, "\n")
}

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
