package service

import (
	"fmt"
	"log/slog"
	"sort"
	"strconv"
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

// GetStatsService 获取 StatsService 单例instance.
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
		Metrics:         dropSessionMetricsIfEventFiltered(parseMetricsWithDefault(req.Metrics, []string{"visitors", "pageviews", "bounce_rate", "visit_duration", "views_per_visit"}), filters),
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
		Metrics:         dropSessionMetricsIfEventFiltered(parseMetricsWithDefault(req.Metrics, []string{"visitors"}), filters),
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
	// event:goal 维度需服务层编排（CH 事件表无 goal_id，目标定义在 PG）
	if property == "event:goal" {
		return s.goalBreakdown(ctx, params)
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

	metricsList := dropSessionMetricsIfEventFiltered(parseMetrics(metricsStr), filters)

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

// QueryV2 POST /api/v2/query 通用查询，支持多维度、排序、offset 分页。
func (s *StatsService) QueryV2(ctx *gin.Context, siteID int64, period, from, to, date,
	metricsStr, dimensionStr, filtersStr, orderByStr string,
	limit, offset int, allDimensions []string, sampleThreshold int64,
	compare, compareFrom, compareTo string) (any, error) {

	site, err := GetSiteService().GetSiteByID(ctx, int(siteID))
	if err != nil {
		return nil, fmt.Errorf("site not found")
	}

	req := &atypes.StatsRequest{
		Period: period, From: from, To: to, Date: date,
		Metrics: metricsStr, Filters: filtersStr,
		Limit: limit, Page: 1,
		SampleThreshold: sampleThreshold,
	}
	start, end := s.getDateRange(req, site.Timezone, 0)
	filters, err := parseRequestFilters(filtersStr)
	if err != nil {
		return nil, err
	}

	metricsList := parseMetricsWithDefault(metricsStr, []string{"visitors"})

	// 解析 order_by 字符串 "metric:direction,..."
	orderBy := parseOrderByString(orderByStr, metricsList)

	// 无维度：聚合查询
	if len(allDimensions) == 0 {
		params := &types.Params{
			UserID: ctx.GetInt64("user_id"),
			SiteID: fmt.Sprintf("%d", site.ID),
			UTCTimeRange: types.TimeRange{Start: start, End: end},
			Period: period, Date: date, From: from, To: to,
			Timezone: site.Timezone,
			Metrics:  metricsList, Dimensions: []string{},
			Filters: filters, SampleThreshold: sampleThreshold,
		}
		// 对比区间：仅聚合查询支持。设置后 GetAggregate 会自动跑对比查询并返回 comparison_value/change。
		if cmp := comparisonRange(start, end, site.Timezone, compare, compareFrom, compareTo); cmp != nil {
			params.ComparisonUTCTimeRange = cmp
		}
		qs := s.getQueryService()
		agg := stats.NewAggregateService(qs)
		return agg.GetAggregate(ctx, params)
	}

	// percentage 在 Go 侧按分组访客占比计算：从 SQL 指标中剔除，并确保 visitors 参与查询。
	sqlMetrics, wantPercentage := stripPercentageMetric(metricsList)

	// 单维度：breakdown 查询
	params := &types.Params{
		UserID: ctx.GetInt64("user_id"),
		SiteID: fmt.Sprintf("%d", site.ID),
		UTCTimeRange: types.TimeRange{Start: start, End: end},
		Period: period, Date: date, From: from, To: to,
		Timezone:        site.Timezone,
		Metrics:         sqlMetrics,
		Dimensions:      allDimensions,
		Filters:         filters,
		SampleThreshold: sampleThreshold,
		Pagination:      &types.Pagination{Limit: limit, Offset: offset},
		OrderBy:         orderBy,
	}
	// event:goal 维度需服务层编排（CH 事件表无 goal_id，目标定义在 PG）
	if len(allDimensions) == 1 && allDimensions[0] == "event:goal" {
		return s.goalBreakdown(ctx, params)
	}
	qs := s.getQueryService()
	query, err := qs.CreateQuery(params)
	if err != nil {
		return nil, err
	}
	psite := &types.Site{ID: query.SiteID, Timezone: query.Timezone}
	result, err := qs.Execute(ctx, query, psite)
	if err != nil {
		return nil, err
	}
	if wantPercentage && result != nil {
		addPercentageColumn(result)
	}
	return result, nil
}

// stripPercentageMetric 从指标列表中剔除 percentage（Go 侧计算），并保证 visitors 存在。
// 返回用于 SQL 的指标列表和是否请求了 percentage。
func stripPercentageMetric(metrics []string) (sqlMetrics []string, wantPercentage bool) {
	sqlMetrics = make([]string, 0, len(metrics)+1)
	hasVisitors := false
	for _, m := range metrics {
		if m == "percentage" {
			wantPercentage = true
			continue
		}
		if m == "visitors" {
			hasVisitors = true
		}
		sqlMetrics = append(sqlMetrics, m)
	}
	if wantPercentage && !hasVisitors {
		sqlMetrics = append([]string{"visitors"}, sqlMetrics...)
	}
	if len(sqlMetrics) == 0 {
		sqlMetrics = []string{"visitors"}
	}
	return sqlMetrics, wantPercentage
}

// addPercentageColumn 为细分结果每行追加 percentage = 该组 visitors / 总 visitors * 100（保留 1 位小数）。
func addPercentageColumn(result *types.QueryResult) {
	var total float64
	for _, row := range result.Data {
		total += toFloatValue(row["visitors"])
	}
	for _, row := range result.Data {
		if total > 0 {
			row["percentage"] = float64(int(toFloatValue(row["visitors"])/total*1000+0.5)) / 10
		} else {
			row["percentage"] = float64(0)
		}
	}
	// 在列元数据中登记 percentage（若尚未存在）
	found := false
	for _, c := range result.Columns {
		if c == "percentage" {
			found = true
			break
		}
	}
	if !found {
		result.Columns = append(result.Columns, "percentage")
	}
}

// goalBreakdown 处理按 event:goal 维度的细分。ClickHouse 事件表没有 goal_id，
// 目标定义存在 PostgreSQL 且按 name/pathname(+custom_props) 匹配，无法用 SQL JOIN 实现。
// 因此对站点的每个目标各跑一次带该目标过滤的聚合查询，再组装为细分结果——
// 复用已验证的 goal 过滤路径（addGoalFilter），避免脆弱的跨库/arrayJoin SQL。
func (s *StatsService) goalBreakdown(ctx *gin.Context, params *types.Params) (*types.QueryResult, error) {
	siteID, _ := strconv.ParseInt(params.SiteID, 10, 64)
	goals, err := GetGoalService().ListGoals(ctx.Request.Context(), siteID)
	if err != nil {
		return nil, err
	}

	qs := s.getQueryService()
	agg := stats.NewAggregateService(qs)

	// 每个目标一次独立聚合查询；改为有界并发（此前串行，N 个目标 = N 次串行 CH 扫描）。
	// 结果按索引写入避免竞争；捕获首个错误。ClickHouse 连接池与只读的 *gin.Context 支持并发。
	rows := make([]map[string]any, len(goals))
	const goalConcurrency = 8
	sem := make(chan struct{}, goalConcurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error
	for i := range goals {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()

			g := goals[i]
			// 复制 params：去维度/分页/排序/对比，追加该目标的过滤条件
			p := *params
			p.Dimensions = []string{}
			p.Pagination = nil
			p.OrderBy = nil
			p.ComparisonUTCTimeRange = nil
			goalFilter := &types.Filter{Operator: "is", Dimension: "event:goal", Values: []any{g.DisplayName}}
			p.Filters = append(append([]*types.Filter{}, params.Filters...), goalFilter)

			res, err := agg.GetAggregate(ctx, &p)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				return
			}
			row := map[string]any{"name": g.DisplayName}
			for _, m := range params.Metrics {
				if mr, ok := res.Results[m]; ok {
					row[m] = mr.Value
				} else {
					row[m] = 0
				}
			}
			rows[i] = row
		}(i)
	}
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}

	// 排序：默认按第一个指标降序，遵循 OrderBy
	sortMetric := ""
	desc := true
	if len(params.OrderBy) > 0 {
		sortMetric = params.OrderBy[0].Dimension
		desc = !strings.EqualFold(params.OrderBy[0].Direction, "asc")
	} else if len(params.Metrics) > 0 {
		sortMetric = params.Metrics[0]
	}
	if sortMetric != "" {
		sort.SliceStable(rows, func(i, j int) bool {
			vi, vj := toFloatValue(rows[i][sortMetric]), toFloatValue(rows[j][sortMetric])
			if desc {
				return vi > vj
			}
			return vi < vj
		})
	}

	total := len(rows)
	// 分页
	if params.Pagination != nil {
		off := params.Pagination.Offset
		if off < 0 {
			off = 0
		}
		if off > len(rows) {
			off = len(rows)
		}
		rows = rows[off:]
		if lim := params.Pagination.Limit; lim > 0 && lim < len(rows) {
			rows = rows[:lim]
		}
	}

	columns := append([]string{"name"}, params.Metrics...)
	return &types.QueryResult{Columns: columns, Data: rows, TotalRows: total}, nil
}

// comparisonRange 根据对比模式计算对比区间的 UTC 时间范围，返回 nil 表示不做对比。
// previous_period：紧邻的等长前一区间；year_over_year：去年同期；custom：显式 compare_from/to。
func comparisonRange(start, end time.Time, timezone, mode, from, to string) *types.TimeRange {
	switch mode {
	case "":
		return nil
	case "custom":
		cs := carbon.Parse(from, timezone)
		ce := carbon.Parse(to, timezone)
		if cs.Error != nil || ce.Error != nil {
			return nil
		}
		return &types.TimeRange{
			Start: cs.StartOfDay().SetTimezone(carbon.UTC).StdTime(),
			End:   ce.EndOfDay().SetTimezone(carbon.UTC).StdTime(),
		}
	case "year_over_year":
		return &types.TimeRange{
			Start: start.AddDate(-1, 0, 0),
			End:   end.AddDate(-1, 0, 0),
		}
	default: // previous_period
		dur := end.Sub(start)
		return &types.TimeRange{
			Start: start.Add(-dur),
			End:   end.Add(-dur),
		}
	}
}

// hasEventOnlyFilter 判断过滤器中是否存在事件专属维度（event:name / event:goal / event:props:*）。
// 这类维度只在 events 表可过滤，无法作用于 sessions 表。
func hasEventOnlyFilter(filters []*types.Filter) bool {
	for _, f := range filters {
		if f != nil && f.AnyDimension(func(dim string) bool {
			return dim == "event:name" || dim == "event:goal" || strings.HasPrefix(dim, "event:props:")
		}) {
			return true
		}
	}
	return false
}

// dropSessionMetricsIfEventFiltered 当存在事件专属过滤器时，从固定指标集中剔除
// 只能在 sessions 表计算的指标（bounce_rate / visit_duration / views_per_visit）。
// 这些指标无法按事件维度过滤，若保留会让整个聚合/时序查询被后端校验拒绝（400）。
// 用于仪表盘这类"指标集固定、过滤器由用户添加"的路径：剔除后查询仍可返回 visitors/pageviews 等，
// 被剔除的会话指标在结果中缺省（前端显示为 0/空），而不是让整块面板报错。
func dropSessionMetricsIfEventFiltered(metrics []string, filters []*types.Filter) []string {
	if !hasEventOnlyFilter(filters) {
		return metrics
	}
	strictlySession := map[string]bool{"bounce_rate": true, "visit_duration": true, "views_per_visit": true}
	out := make([]string, 0, len(metrics))
	for _, m := range metrics {
		if !strictlySession[m] {
			out = append(out, m)
		}
	}
	// 若全部被剔除（如时序图仅请求了一个会话指标），回退到 visitors，避免空指标查询返回空图。
	if len(out) == 0 {
		return []string{"visitors"}
	}
	return out
}

// toFloatValue 将任意数值类型转为 float64，用于排序比较。
func toFloatValue(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int32:
		return float64(n)
	case int64:
		return float64(n)
	case uint32:
		return float64(n)
	case uint64:
		return float64(n)
	default:
		return 0
	}
}

func parseOrderByString(raw string, metrics []string) []*types.OrderBy {
	if raw == "" {
		if len(metrics) > 0 {
			return []*types.OrderBy{{Dimension: metrics[0], Direction: "desc"}}
		}
		return nil
	}
	parts := strings.Split(raw, ",")
	result := make([]*types.OrderBy, 0, len(parts))
	for _, p := range parts {
		kv := strings.SplitN(p, ":", 2)
		dir := "desc"
		if len(kv) == 2 {
			dir = kv[1]
		}
		result = append(result, &types.OrderBy{Dimension: kv[0], Direction: dir})
	}
	return result
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

// getQueryService 创建并返回查询服务instance.
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
