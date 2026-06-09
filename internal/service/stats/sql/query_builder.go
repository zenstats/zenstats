package sql

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/zenstats/zenstats/internal/service/stats/types"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent"
	"github.com/zenstats/zenstats/pkg/globals"
)

// searchEngineCache caches the search engine list from PostgreSQL
var (
	searchEngineCache     []*ent.SearchEngines
	searchEngineCacheTime time.Time
	searchEngineCacheMu   sync.RWMutex
	searchEngineCacheTTL  = 5 * time.Minute
)

// QueryBuilder builds SQL queries based on the analytics query specification
type QueryBuilder struct {
	fragmentGenerator *SQLFragmentGenerator
}

// NewQueryBuilder creates a new QueryBuilder instance
func NewQueryBuilder() *QueryBuilder {
	return &QueryBuilder{
		fragmentGenerator: NewFragmentGenerator(),
	}
}

// Build constructs the complete SQL query based on the input query and site
func (qb *QueryBuilder) Build(query *types.Query, site *types.Site) (string, []any, error) {
	qo := &QueryOptimizer{}
	query = qo.Optimize(query)
	splits := qo.Split(query)
	joinedSQL, params, err := qb.buildSplitQueries(site, query, splits)
	if err != nil {
		return "", nil, err
	}

	// Apply pagination
	paginatedSQL := qb.paginate(joinedSQL, query)

	// Add total rows selection only when pagination is requested
	finalSQL := qb.selectTotalRows(paginatedSQL, query)

	return finalSQL, params, nil
}

func (qb *QueryBuilder) buildSplitQueries(site *types.Site, original *types.Query, splits []*SplitQuery) (string, []any, error) {
	if len(splits) == 0 {
		return "", nil, nil
	}

	sqlParts := make([]string, 0, len(splits))
	queries := make([]*types.Query, 0, len(splits))
	params := []any{}
	aliases := []string{"e", "s", "ss"}

	for _, split := range splits {
		var (
			sqlPart    string
			partParams []any
			err        error
		)
		switch split.TableType {
		case TableEvents:
			sqlPart, partParams, err = qb.buildEventsQuery(site, split.Query)
		case TableSessions:
			sqlPart, partParams, err = qb.buildSessionsQuery(site, split.Query)
		case TableSessionsSmeared:
			sqlPart, partParams, err = qb.buildSessionsSmearedQuery(site, split.Query)
		default:
			err = fmt.Errorf("unsupported split table type: %s", split.TableType)
		}
		if err != nil {
			return "", nil, err
		}
		if sqlPart == "" {
			continue
		}
		sqlParts = append(sqlParts, sqlPart)
		queries = append(queries, split.Query)
		params = append(params, partParams...)
	}

	if len(sqlParts) == 0 {
		return "", nil, nil
	}
	if len(sqlParts) == 1 {
		return qb.buildOrderBy(sqlParts[0], original), params, nil
	}

	joinType := original.SQLJoinType
	if joinType == "" {
		joinType = "LEFT"
	}
	selectClause := qb.buildMultiJoinSelectClause(aliases[:len(sqlParts)], queries)
	joined := fmt.Sprintf("(%s) AS %s", sqlParts[0], aliases[0])
	for i := 1; i < len(sqlParts); i++ {
		joined = fmt.Sprintf("%s %s JOIN (%s) AS %s ON %s", joined, joinType, sqlParts[i], aliases[i], qb.buildJoinCondition(aliases[0], aliases[i], original))
	}

	return qb.buildOrderBy(fmt.Sprintf("SELECT %s FROM %s", selectClause, joined), original), params, nil
}

// sampleClause 返回 SAMPLE 子句，当采样未启用时返回空字符串。
func (qb *QueryBuilder) sampleClause(query *types.Query) string {
	if query.SampleThreshold <= 0 {
		return ""
	}
	return fmt.Sprintf(" SAMPLE %d", query.SampleThreshold)
}

// buildEventsQuery constructs the SQL for event-based metrics
func (qb *QueryBuilder) buildEventsQuery(site *types.Site, eventQuery *types.Query) (string, []any, error) {
	if len(eventQuery.Metrics) == 0 {
		return "", nil, nil
	}

	// Initialize WHERE clause using WhereBuilder
	whereBuilder := NewWhereBuilder(site.ID)
	whereBuilder.FilterSiteTimeRange("events", eventQuery.UTCTimeRange.Start, eventQuery.UTCTimeRange.End)

	// Add filters
	for _, filter := range eventQuery.Filters {
		if err := whereBuilder.AddFilter("events", filter); err != nil {
			return "", nil, err
		}
	}

	whereClause, params := whereBuilder.Build()

	// Base event query
	sqlBuilder := strings.Builder{}
	sqlBuilder.WriteString("SELECT ")
	sqlBuilder.WriteString(qb.selectEventMetrics(eventQuery))
	sqlBuilder.WriteString(" FROM events e")
	sqlBuilder.WriteString(qb.sampleClause(eventQuery))

	// // Add goal join if needed
	// goalJoinAdded := false
	// for _, dim := range eventQuery.Dimensions {
	// 	if dim == "event:goal" {
	// 		sqlBuilder.WriteString(" JOIN goals g ON e.goal_id = g.id")
	// 		goalJoinAdded = true
	// 		break
	// 	}
	// }

	// // Add hostname filter join if needed
	// if qb.needsHostnameFilter(eventQuery) && !goalJoinAdded {
	// 	sqlBuilder.WriteString(" JOIN sites s ON e.site_id = s.id")
	// }

	// Add session join if needed
	if qb.eventsJoinSessions(eventQuery) {
		sessionSubquery, subqParams, err := qb.buildSessionSubquery(eventQuery)
		if err != nil {
			return "", nil, err
		}
		sqlBuilder.WriteString(" JOIN (" + sessionSubquery + ") AS sq ON e.session_id = sq.session_id")
		// Session subquery params must come first since the subquery's ? placeholders appear first in SQL
		params = append(subqParams, params...)
	}

	if whereClause != "" {
		sqlBuilder.WriteString(" WHERE " + whereClause)
	}

	// Add grouping
	groupBy := qb.buildGroupBy("events", eventQuery)
	if groupBy != "" {
		sqlBuilder.WriteString(groupBy)
	}

	// Apply enterprise edition query hints
	sql := sqlBuilder.String()
	// Add special metrics
	sql = qb.addSpecialMetrics(sql, site, eventQuery)

	// Add time on page metrics
	sql = qb.addTimeOnPageMetrics(sql, eventQuery)

	return sql, params, nil
}

// selectEventMetrics generates the SELECT clause for event metrics
func (qb *QueryBuilder) selectEventMetrics(query *types.Query) string {
	samplingEnabled := query.SampleThreshold > 0
	metrics := make([]string, 0, len(query.Metrics))

	for _, metric := range query.Metrics {
		sql, err := AvailableMetrics.GetMetricSQL(metric, samplingEnabled)
		if err != nil {
			slog.Error("GetMetricSQL error", "metric", metric, "error", err)
		}
		metrics = append(metrics, sql)
	}

	// 添加维度字段到SELECT子句
	dimensions := make([]string, 0, len(query.Dimensions))
	for _, dim := range query.Dimensions {
		column := qb.DimensionToColumn(dim, "events", "select")
		if column != "" {
			dimensions = append(dimensions, column)
		}
	}

	// 合并维度和指标
	selectParts := append(dimensions, metrics...)
	return strings.Join(selectParts, ", ")
}

// selectSessionMetrics generates the SELECT clause for session metrics
func (qb *QueryBuilder) selectSessionMetrics(query *types.Query) string {
	if len(query.Metrics) == 0 {
		return "count(*) as visitors"
	}
	samplingEnabled := query.SampleThreshold > 0
	metrics := make([]string, 0, len(query.Metrics))
	for _, metric := range query.Metrics {
		sql, err := qb.getMetricSQLForTable(metric, "sessions", samplingEnabled)
		if err != nil {
			slog.Error("GetMetricSQL error", "metric", metric, "error", err)
		}
		metrics = append(metrics, sql)
	}
	dimensions := make([]string, 0, len(query.Dimensions))
	for _, dim := range query.Dimensions {
		column := qb.DimensionToColumn(dim, "sessions", "select")
		if column != "" {
			dimensions = append(dimensions, column)
		}
	}

	// 合并维度和指标
	metrics = append(dimensions, metrics...)

	return strings.Join(metrics, ", ")
}

// selectSessionMetricsWithJoinAlias 生成带 JOIN 别名前缀的 session SELECT 子句。
// 当 session 查询 JOIN events 子查询时，event 维度需要从 eq 别名获取。
func (qb *QueryBuilder) selectSessionMetricsWithJoinAlias(query *types.Query, joinAlias string) string {
	if len(query.Metrics) == 0 {
		return "count(*) as visitors"
	}
	samplingEnabled := query.SampleThreshold > 0
	metrics := make([]string, 0, len(query.Metrics))
	for _, metric := range query.Metrics {
		sql, err := qb.getMetricSQLForTable(metric, "sessions", samplingEnabled)
		if err != nil {
			slog.Error("GetMetricSQL error", "metric", metric, "error", err)
		}
		metrics = append(metrics, sql)
	}
	dimensions := make([]string, 0, len(query.Dimensions))
	for _, dim := range query.Dimensions {
		column := qb.DimensionToColumn(dim, "sessions", "select")
		if column != "" {
			if isEventDimension(dim) {
				column = fmt.Sprintf("%s.%s AS %s", joinAlias, column, qb.DimensionAlias(dim))
			}
			dimensions = append(dimensions, column)
		}
	}

	metrics = append(dimensions, metrics...)
	return strings.Join(metrics, ", ")
}

func (qb *QueryBuilder) getMetricSQLForTable(metric, tableType string, samplingEnabled bool) (string, error) {
	if tableType == "sessions" && metric == "events" {
		fragment := qb.fragmentGenerator.EventsForSession(samplingEnabled)
		return fmt.Sprintf("%s as events", fragment.ToSql()), nil
	}

	return AvailableMetrics.GetMetricSQL(metric, samplingEnabled)
}

// buildGroupBy generates the GROUP BY clause based on dimensions
func (qb *QueryBuilder) buildGroupBy(tableType string, query *types.Query) string {
	if len(query.Dimensions) == 0 {
		return ""
	}

	groups := make([]string, 0, len(query.Dimensions))

	for _, dim := range query.Dimensions {
		column := qb.DimensionToColumn(dim, tableType, "group")
		if column != "" {
			groups = append(groups, column)
		}
	}

	if len(groups) == 0 {
		return ""
	}

	return " GROUP BY " + strings.Join(groups, ", ")
}

// buildGroupByWithJoinAlias 生成带 JOIN 别名前缀的 GROUP BY 子句。
// 当 session 查询 JOIN events 子查询时，event 维度需要从 joinAlias 获取。
func (qb *QueryBuilder) buildGroupByWithJoinAlias(tableType string, query *types.Query, joinAlias string) string {
	if len(query.Dimensions) == 0 {
		return ""
	}

	groups := make([]string, 0, len(query.Dimensions))

	for _, dim := range query.Dimensions {
		column := qb.DimensionToColumn(dim, tableType, "group")
		if column != "" {
			if isEventDimension(dim) {
				column = fmt.Sprintf("%s.%s", joinAlias, column)
			}
			groups = append(groups, column)
		}
	}

	if len(groups) == 0 {
		return ""
	}

	return " GROUP BY " + strings.Join(groups, ", ")
}

// isEventDimension 判断维度是否属于事件表
func isEventDimension(dim string) bool {
	return strings.HasPrefix(dim, "event:")
}

// addSpecialMetrics includes special metrics in the query
// Note: bounce_rate is handled by the metric SQL expression using sessions table (is_bounce * sign),
// so no special subquery transformation is needed here.
func (qb *QueryBuilder) addSpecialMetrics(sql string, site *types.Site, query *types.Query) string {
	return sql
}

// buildSessionsQuery constructs the SQL for session-based metrics
func (qb *QueryBuilder) buildSessionsQuery(site *types.Site, sessionsQuery *types.Query) (string, []any, error) {
	if len(sessionsQuery.Metrics) == 0 {
		return "", nil, nil
	}

	// Initialize WHERE clause using WhereBuilder
	whereBuilder := NewWhereBuilder(site.ID)
	whereBuilder.FilterSiteTimeRange("sessions", sessionsQuery.UTCTimeRange.Start, sessionsQuery.UTCTimeRange.End)

	// Add filters
	for _, filter := range sessionsQuery.Filters {
		if err := whereBuilder.AddFilter("sessions", filter); err != nil {
			return "", nil, err
		}
	}

	whereClause, params := whereBuilder.Build()
	// TODO: Handle parameterized queries properly - for now we use string interpolation
	// In production, this should use proper parameter binding

	// Base session query
	sqlBuilder := strings.Builder{}
	sqlBuilder.WriteString("SELECT ")

	// Add event join if needed
	joinWithEvents := qb.sessionsJoinEvents(sessionsQuery)
	if joinWithEvents {
		sqlBuilder.WriteString(qb.selectSessionMetricsWithJoinAlias(sessionsQuery, "eq"))
	} else {
		sqlBuilder.WriteString(qb.selectSessionMetrics(sessionsQuery))
	}
	sqlBuilder.WriteString(" FROM sessions s")
	sqlBuilder.WriteString(qb.sampleClause(sessionsQuery))

	if joinWithEvents {
		eventSubquery, subqParams, err := qb.buildEventSubquery(sessionsQuery)
		if err != nil {
			return "", nil, err
		}
		sqlBuilder.WriteString(" JOIN (" + eventSubquery + ") AS eq ON s.session_id = eq.session_id")
		// Event subquery params must come first since the subquery's ? placeholders appear first in SQL
		params = append(subqParams, params...)
	}

	if whereClause != "" {
		sqlBuilder.WriteString(" WHERE " + whereClause)
	}

	// Add grouping
	var groupBy string
	if joinWithEvents {
		groupBy = qb.buildGroupByWithJoinAlias("sessions", sessionsQuery, "eq")
	} else {
		groupBy = qb.buildGroupBy("sessions", sessionsQuery)
	}
	if groupBy != "" {
		sqlBuilder.WriteString(groupBy)
	}

	// Apply enterprise edition query hints
	sql := sqlBuilder.String()

	// Add special metrics
	sql = qb.addSpecialMetrics(sql, site, sessionsQuery)

	return sql, params, nil
}

func (qb *QueryBuilder) buildSessionsSmearedQuery(site *types.Site, sessionsQuery *types.Query) (string, []any, error) {
	if len(sessionsQuery.Metrics) == 0 {
		return "", nil, nil
	}
	whereBuilder := NewWhereBuilder(site.ID)
	whereBuilder.FilterSiteTimeRange("sessions", sessionsQuery.UTCTimeRange.Start, sessionsQuery.UTCTimeRange.End)
	for _, filter := range sessionsQuery.Filters {
		if err := whereBuilder.AddFilter("sessions", filter); err != nil {
			return "", nil, err
		}
	}
	whereClause, params := whereBuilder.Build()

	joinWithEvents := qb.sessionsJoinEvents(sessionsQuery)

	base := strings.Builder{}
	base.WriteString("SELECT ")
	if joinWithEvents {
		base.WriteString(qb.selectSessionMetricsWithJoinAlias(sessionsQuery, "eq"))
	} else {
		base.WriteString(qb.selectSessionMetrics(sessionsQuery))
	}
	base.WriteString(" FROM (SELECT s.*, arrayJoin(")
	base.WriteString(qb.sessionTimeSlotsExpr(sessionsQuery))
	base.WriteString(") AS timestamp FROM sessions s")
	base.WriteString(qb.sampleClause(sessionsQuery))
	if joinWithEvents {
		eventSubquery, subqParams, err := qb.buildEventSubquery(sessionsQuery)
		if err != nil {
			return "", nil, err
		}
		base.WriteString(" JOIN (" + eventSubquery + ") AS eq ON s.session_id = eq.session_id")
		// Event subquery params must come first since the subquery's ? placeholders appear first in SQL
		params = append(subqParams, params...)
	}
	if whereClause != "" {
		base.WriteString(" WHERE " + whereClause)
	}
	base.WriteString(") s")
	var groupBy string
	if joinWithEvents {
		groupBy = qb.buildGroupByWithJoinAlias("sessions", sessionsQuery, "eq")
	} else {
		groupBy = qb.buildGroupBy("sessions", sessionsQuery)
	}
	if groupBy != "" {
		base.WriteString(groupBy)
	}
	return base.String(), params, nil
}

func (qb *QueryBuilder) sessionTimeSlotsExpr(query *types.Query) string {
	step := "3600"
	if qb.hasDimension(query, "time:minute") {
		step = "60"
	}
	return fmt.Sprintf("arrayMap(x -> toDateTime(x), range(toUInt32(toStartOfInterval(start, INTERVAL %s second)), toUInt32(toStartOfInterval(greatest(timestamp, start), INTERVAL %s second)) + %s, %s))", step, step, step, step)
}

// buildSessionSubquery creates a subquery for session joins in event queries
func (qb *QueryBuilder) buildSessionSubquery(query *types.Query) (string, []any, error) {
	whereBuilder := NewWhereBuilder(query.SiteID)
	whereBuilder.FilterSiteTimeRange("sessions", query.UTCTimeRange.Start, query.UTCTimeRange.End)

	for _, filter := range query.Filters {
		if err := whereBuilder.AddFilter("sessions", filter); err != nil {
			return "", nil, err
		}
	}

	whereClause, _ := whereBuilder.Build()

	sql := fmt.Sprintf(
		"SELECT session_id FROM sessions WHERE %s AND sign = 1 GROUP BY session_id",
		whereClause,
	)
	return sql, nil, nil
}

// buildEventSubquery creates a subquery for event joins in session queries.
// It selects both session_id and any event dimension columns needed by the outer query.
func (qb *QueryBuilder) buildEventSubquery(query *types.Query) (string, []any, error) {
	whereBuilder := NewWhereBuilder(query.SiteID)
	whereBuilder.FilterSiteTimeRange("events", query.UTCTimeRange.Start, query.UTCTimeRange.End)

	// Only apply event-specific filters (not visit-specific) to the events subquery
	for _, filter := range query.Filters {
		if err := whereBuilder.AddFilter("events", filter); err != nil {
			return "", nil, err
		}
	}

	whereClause, params := whereBuilder.Build()

	// Build SELECT clause: always include session_id, plus any event dimension columns
	selectCols := []string{"session_id"}
	for _, dim := range query.Dimensions {
		if isEventDimension(dim) {
			column := qb.DimensionToColumn(dim, "events", "select")
			if column != "" {
				selectCols = append(selectCols, column)
			}
		}
	}

	sql := fmt.Sprintf(
		"SELECT DISTINCT %s FROM events WHERE %s",
		strings.Join(selectCols, ", "),
		whereClause,
	)
	return sql, params, nil
}

// joinQueryResults combines event and session query results
func (qb *QueryBuilder) joinQueryResults(eventSQL string, eventQuery *types.Query, sessionSQL string, sessionQuery *types.Query) (string, []any, error) {
	// Handle cases where one of the queries is nil
	if eventSQL == "" && sessionSQL == "" {
		return "", nil, nil
	} else if eventSQL != "" && sessionSQL == "" {
		return qb.buildOrderBy(eventSQL, eventQuery), nil, nil
	} else if eventSQL == "" && sessionSQL != "" {
		return qb.buildOrderBy(sessionSQL, sessionQuery), nil, nil
	}

	tableDecider := &TableDecider{}
	// Determine join type and fields using TableDecider
	joinType := tableDecider.JoinType(eventQuery, sessionQuery)
	eFields := tableDecider.EventFields(eventQuery)
	sFields := tableDecider.SessionFields(sessionQuery)

	// Build SELECT clause with distinct fields
	selectClause := qb.buildJoinSelectClause(eFields, sFields)
	// Construct join query with dynamic join type
	joinSQL := fmt.Sprintf(
		"SELECT %s FROM (%s) AS e %s JOIN (%s) AS s ON %s",
		selectClause, eventSQL, joinType, sessionSQL, qb.buildGroupByJoin(eventQuery),
	)

	return qb.buildOrderBy(joinSQL, eventQuery), nil, nil
}

// buildOrderBy adds ORDER BY clause to the query
func (qb *QueryBuilder) buildOrderBy(sql string, query *types.Query) string {
	if len(query.OrderBy) == 0 {
		return sql
	}

	orderClauses := make([]string, len(query.OrderBy))
	for i, ob := range query.OrderBy {
		dimension := ob.Dimension
		direction := ob.Direction
		orderClauses[i] = fmt.Sprintf("%s %s", qb.shortName(dimension), direction)
	}

	if strings.Contains(sql, "ORDER BY") {
		return sql + ", " + strings.Join(orderClauses, ", ")
	}
	return sql + " ORDER BY " + strings.Join(orderClauses, ", ")
}

// paginate applies LIMIT and OFFSET to the query
func (qb *QueryBuilder) paginate(sql string, query *types.Query) string {
	if query.Pagination == nil || query.Pagination.Limit <= 0 {
		return sql
	}

	sql += fmt.Sprintf(" LIMIT %d", query.Pagination.Limit)
	if query.Pagination.Offset > 0 {
		sql += fmt.Sprintf(" OFFSET %d", query.Pagination.Offset)
	}

	return sql
}

// selectTotalRows wraps the query with a total_rows count column when pagination is active.
// It returns the original SQL unchanged; total_rows is handled separately in processResults.
func (qb *QueryBuilder) selectTotalRows(sql string, query *types.Query) string {
	return sql
}

// NeedsTotalRows returns true when pagination is active and total_rows metadata should be collected.
func (qb *QueryBuilder) NeedsTotalRows(query *types.Query) bool {
	return query.Pagination != nil && query.Pagination.Limit > 0
}

// buildGroupByJoin creates join conditions based on query dimensions
func (qb *QueryBuilder) buildGroupByJoin(query *types.Query) string {
	if len(query.Dimensions) == 0 {
		return "1=1"
	}

	var conditions []string
	for _, dim := range query.Dimensions {
		shortName := qb.shortName(dim)
		condition := fmt.Sprintf("e.%s = s.%s", shortName, shortName)
		conditions = append(conditions, condition)
	}

	return strings.Join(conditions, " AND ")
}

func (qb *QueryBuilder) buildJoinCondition(leftAlias, rightAlias string, query *types.Query) string {
	if len(query.Dimensions) == 0 {
		return "1=1"
	}
	conditions := make([]string, 0, len(query.Dimensions))
	for _, dim := range query.Dimensions {
		shortName := qb.shortName(dim)
		conditions = append(conditions, fmt.Sprintf("%s.%s = %s.%s", leftAlias, shortName, rightAlias, shortName))
	}
	return strings.Join(conditions, " AND ")
}

func (qb *QueryBuilder) buildMultiJoinSelectClause(aliases []string, queries []*types.Query) string {
	fields := []string{}
	seen := map[string]bool{}
	for i, query := range queries {
		alias := aliases[i]
		for _, dim := range query.Dimensions {
			name := qb.shortName(dim)
			if seen[name] {
				continue
			}
			seen[name] = true
			fields = append(fields, fmt.Sprintf("%s.%s AS %s", alias, name, name))
		}
		for _, metric := range query.Metrics {
			name := qb.shortName(metric)
			if seen[name] {
				continue
			}
			seen[name] = true
			fields = append(fields, fmt.Sprintf("%s.%s AS %s", alias, name, name))
		}
	}
	return strings.Join(fields, ", ")
}

func (qb *QueryBuilder) hasDimension(query *types.Query, dimension string) bool {
	for _, dim := range query.Dimensions {
		if dim == dimension {
			return true
		}
	}
	return false
}

// shortName generates a short alias for metrics/dimensions
func (qb *QueryBuilder) shortName(dimension string) string {
	if dimension == "event:page" || dimension == "visit:entry_page" {
		return "page"
	}
	parts := strings.Split(dimension, ":")
	if len(parts) > 1 {
		return parts[1]
	}
	return dimension
}

// eventsJoinSessions determines if events need to join sessions
func (qb *QueryBuilder) eventsJoinSessions(query *types.Query) bool {
	// Check if any dimension requires session data
	for _, dim := range query.Dimensions {
		if strings.HasPrefix(dim, "visit:") {
			return true
		}
	}
	return false
}

// sessionsJoinEvents determines if sessions need to join events
func (qb *QueryBuilder) sessionsJoinEvents(query *types.Query) bool {
	// Check if any dimension requires event data
	for _, dim := range query.Dimensions {
		if strings.HasPrefix(dim, "event:") && dim != "event:goal" {
			return true
		}
	}
	return false
}

// DimensionAlias returns the ClickHouse result column alias for a dimension.
// This is the name ClickHouse uses in result metadata, not the full SQL expression.
func (qb *QueryBuilder) DimensionAlias(dimension string) string {
	switch dimension {
	case "event:name":
		return "name"
	case "event:page":
		return "page"
	case "event:hostname":
		return "hostname"
	case "event:goal":
		return "name"
	case "event:referrer":
		return "referrer"
	case "event:referrer:domain":
		return "referrer_domain"
	case "event:utm_source":
		return "utm_source"
	case "event:utm_medium":
		return "utm_medium"
	case "event:utm_campaign":
		return "utm_campaign"
	case "event:utm_content":
		return "utm_content"
	case "event:utm_term":
		return "utm_term"
	case "event:browser":
		return "browser"
	case "event:browser_version":
		return "browser_version"
	case "event:os":
		return "os"
	case "event:os_version":
		return "os_version"
	case "event:device":
		return "device"
	case "event:screen_size":
		return "screen_size"
	case "event:country":
		return "country"
	case "event:region", "visit:region":
		return "region"
	case "event:city", "visit:city":
		return "city"
	case "visit:source":
		return "referrer_source"
	case "visit:medium":
		return "referrer_medium"
	case "visit:referrer":
		return "referrer"
	case "visit:device":
		return "device"
	case "visit:browser":
		return "browser"
	case "visit:browser_version":
		return "browser_version"
	case "visit:os":
		return "os"
	case "visit:os_version":
		return "os_version"
	case "visit:country":
		return "country"
	case "visit:screen_size":
		return "screen_size"
	case "visit:entry_page":
		return "page"
	case "visit:entry_page_hostname":
		return "entry_page_hostname"
	case "visit:exit_page":
		return "exit_page"
	case "visit:exit_page_hostname":
		return "exit_page_hostname"
	case "time:minute":
		return "minute"
	case "time:hour":
		return "hour"
	case "time:day":
		return "day"
	case "time:week":
		return "week"
	case "time:month":
		return "month"
	case "time:quarter":
		return "quarter"
	case "time:year":
		return "year"
	case "time:day_of_week":
		return "day_of_week"
	case "time:day_of_year":
		return "day_of_year"
	default:
		if strings.HasPrefix(dimension, "event:props:") {
			return strings.TrimPrefix(dimension, "event:props:")
		}
		if strings.HasPrefix(dimension, "visit:entry_props:") {
			return strings.TrimPrefix(dimension, "visit:entry_props:")
		}
		parts := strings.Split(dimension, ":")
		if len(parts) > 1 {
			return parts[1]
		}
		return dimension
	}
}

// DimensionToColumn converts dimension names to database column names
func (qb *QueryBuilder) DimensionToColumn(dimension, tableType, purpose string) string {
	switch dimension {
	case "event:name":
		return "name"
	case "event:page":
		if purpose == "select" {
			return "pathname as page"
		}
		return "pathname"
	case "event:hostname":
		return "hostname"
	case "event:goal":
		return "g.name"
	case "event:referrer":
		return "referrer"
	case "event:referrer:domain":
		return "referrer_domain"
	case "event:utm_source":
		return "utm_source"
	case "event:utm_medium":
		return "utm_medium"
	case "event:utm_campaign":
		return "utm_campaign"
	case "event:utm_content":
		return "utm_content"
	case "event:utm_term":
		return "utm_term"
	case "event:browser":
		return "browser"
	case "event:browser_version":
		return "browser_version"
	case "event:os":
		return "os"
	case "event:os_version":
		return "os_version"
	case "event:device":
		return "device"
	case "event:screen_size":
		return "screen_size"
	case "event:country":
		if purpose == "group" {
			return "country_name"
		}
		return "country_name as country"
	case "event:region":
		if purpose == "group" {
			return "continent_name"
		}
		return "continent_name as region"
	case "event:city":
		if purpose == "group" {
			return "city_name"
		}
		return "city_name as city"
	case "visit:source":
		if purpose == "group" {
			return "referrer_source"
		}
		sourceClause, err := qb.getSourceClause()
		if err != nil {
			return "referrer_source"
		}
		return sourceClause
	case "visit:medium":
		return "referrer_medium"
	case "visit:referrer":
		return "referrer"
	case "visit:device":
		return "device"
	case "visit:browser":
		return "browser"
	case "visit:browser_version":
		return "browser_version"
	case "visit:os":
		return "os"
	case "visit:os_version":
		return "os_version"
	case "visit:country":
		if purpose == "group" {
			return "country_name"
		}
		return "country_name as country"
	case "visit:region":
		if purpose == "group" {
			return "continent_name"
		}
		return "continent_name as region"
	case "visit:city":
		if purpose == "group" {
			return "city_name"
		}
		return "city_name as city"
	case "visit:screen_size":
		return "screen_size"
	case "visit:entry_page":
		if purpose == "select" {
			return "entry_page as page"
		}
		return "entry_page"
	case "visit:entry_page_hostname":
		return "entry_page_hostname"
	case "visit:exit_page":
		return "exit_page"
	case "visit:exit_page_hostname":
		return "exit_page_hostname"
	case "time:minute":
		if purpose == "group" {
			return "minute"
		}
		return "formatDateTime(toStartOfInterval(timestamp, INTERVAL 1 minute), '%Y-%m-%d %H:%i') as minute"
	case "time:hour":
		if purpose == "group" {
			return "hour"
		}
		return "formatDateTime(toStartOfHour(timestamp), '%Y-%m-%d %H') as hour"
	case "time:day":
		if purpose == "group" {
			return "day"
		}
		return "formatDateTime(toStartOfDay(timestamp), '%Y-%m-%d') as day"
	case "time:week":
		if purpose == "group" {
			return "week"
		}
		return "formatDateTime(toStartOfWeek(timestamp), '%Y-%u') as week"
	case "time:month":
		if purpose == "group" {
			return "month"
		}
		return "formatDateTime(toStartOfMonth(timestamp), '%Y-%m') as month"
	case "time:quarter":
		if purpose == "group" {
			return "quarter"
		}
		return "formatDateTime(toStartOfQuarter(timestamp), '%Y-Q%q') as quarter"
	case "time:year":
		if purpose == "group" {
			return "year"
		}
		return "formatDateTime(toStartOfYear(timestamp), '%Y') as year"
	case "time:day_of_week":
		if purpose == "group" {
			return "day_of_week"
		}
		return "formatDateTime(timestamp, '%w') as day_of_week"
	case "time:day_of_year":
		if purpose == "group" {
			return "day_of_year"
		}
		return "formatDateTime(timestamp, '%j') as day_of_year"
	default:
		if strings.HasPrefix(dimension, "event:props:") {
			propName := strings.TrimPrefix(dimension, "event:props:")
			return fmt.Sprintf("meta['%s']", propName)
		}
		if strings.HasPrefix(dimension, "visit:entry_props:") {
			propName := strings.TrimPrefix(dimension, "visit:entry_props:")
			return fmt.Sprintf("entry_meta['%s']", propName)
		}
		// if strings.HasPrefix(dimension, "visit:props:") {
		// 	propName := strings.TrimPrefix(dimension, "visit:props:")
		// 	return fmt.Sprintf("session_meta['%s']", propName)
		// }
		return dimension
	}
}

// addTimeOnPageMetrics adds time on page calculations to the query
func (qb *QueryBuilder) addTimeOnPageMetrics(sql string, query *types.Query) string {
	for _, metric := range query.Metrics {
		if metric == "time_on_page" || metric == "avg_time_on_page" {
			// Add time on page subquery
			return strings.Replace(sql, "FROM events e", `FROM (
				SELECT
					e.*,
					LEAD(timestamp) OVER (PARTITION BY session_id ORDER BY timestamp) as next_event_time,
					TIMESTAMPDIFF(SECOND, timestamp, LEAD(timestamp) OVER (PARTITION BY session_id ORDER BY timestamp)) as time_on_page
				FROM events
			) e`, 1)
		}
	}
	return sql
}

// buildJoinSelectClause creates a SELECT clause combining event and session fields
func (qb *QueryBuilder) buildJoinSelectClause(eventFields, sessionFields []string) string {
	// Prefix event fields with 'e.' and session fields with 's.'
	prefixedEvent := make([]string, len(eventFields))
	for i, field := range eventFields {
		prefixedEvent[i] = fmt.Sprintf("e.%s", field)
	}

	prefixedSession := make([]string, len(sessionFields))
	for i, field := range sessionFields {
		prefixedSession[i] = fmt.Sprintf("s.%s", field)
	}

	// Combine all fields, removing duplicates
	allFields := append(prefixedEvent, prefixedSession...)
	seen := make(map[string]bool)
	uniqueFields := []string{}

	for _, field := range allFields {
		if !seen[field] {
			seen[field] = true
			uniqueFields = append(uniqueFields, field)
		}
	}

	return strings.Join(uniqueFields, ", ")
}

// getSearchEngines returns the cached search engine list, refreshing if TTL expired
func getSearchEngines() []*ent.SearchEngines {
	searchEngineCacheMu.RLock()
	if searchEngineCache != nil && time.Since(searchEngineCacheTime) < searchEngineCacheTTL {
		defer searchEngineCacheMu.RUnlock()
		return searchEngineCache
	}
	searchEngineCacheMu.RUnlock()

	searchEngineCacheMu.Lock()
	defer searchEngineCacheMu.Unlock()

	// Double-check after acquiring write lock
	if searchEngineCache != nil && time.Since(searchEngineCacheTime) < searchEngineCacheTTL {
		return searchEngineCache
	}

	db := globals.GetDB()
	if db == nil {
		return searchEngineCache
	}
	searchEngineCache = db.Client.SearchEngines.Query().AllX(context.Background())
	searchEngineCacheTime = time.Now()
	return searchEngineCache
}

// getSourceClause generates the CASE statement for source classification
func (qs *QueryBuilder) getSourceClause() (string, error) {
	searchEngines := getSearchEngines()

	clause := fmt.Sprintf(`
			CASE
				WHEN referrer_source = '' THEN 'Direct / None'
				%s
				ELSE referrer_source
			END as referrer_source
	`, qs.buildSearchEngineCase(searchEngines))
	return clause, nil
}

func (qs *QueryBuilder) buildSearchEngineCase(searchEngines []*ent.SearchEngines) string {
	var conditions []string
	for _, searchEngine := range searchEngines {
		conditions = append(conditions, fmt.Sprintf("WHEN positionCaseInsensitive(referrer_source, '%s') > 0 THEN '%s'", searchEngine.Domain, searchEngine.Name))
	}
	return strings.Join(conditions, "\n")
}
