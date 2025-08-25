package sql

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/zenstats/zenstats/internal/service/stats/types"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent"
	"github.com/zenstats/zenstats/pkg/globals"
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
	// Split query into event and session components using QueryOptimizer
	qo := &QueryOptimizer{}
	query = qo.Optimize(query)
	eventQuery, sessionQuery := qo.Split(query)
	// Build individual queries
	eventSQL, eventParams, err := qb.buildEventsQuery(site, eventQuery)
	if err != nil {
		return "", nil, err
	}

	sessionSQL, sessionParams, err := qb.buildSessionsQuery(site, sessionQuery)
	if err != nil {
		return "", nil, err
	}

	// Join query results
	joinedSQL, joinParams, err := qb.joinQueryResults(eventSQL, eventQuery, sessionSQL, sessionQuery)
	if err != nil {
		return "", nil, err
	}

	// Apply pagination
	paginatedSQL := qb.paginate(joinedSQL, query)

	// Add total rows selection if needed
	finalSQL := qb.selectTotalRows(paginatedSQL)

	// Combine parameters from all parts of the query
	allParams := append(eventParams, sessionParams...)
	allParams = append(allParams, joinParams...)

	return finalSQL, allParams, nil
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

	if whereClause != "" {
		sqlBuilder.WriteString(" WHERE " + whereClause)
	}

	// Add session join if needed
	if qb.eventsJoinSessions(eventQuery) {
		sessionSubquery, subqParams, err := qb.buildSessionSubquery(eventQuery)
		if err != nil {
			return "", nil, err
		}
		sqlBuilder.WriteString(" JOIN (" + sessionSubquery + ") AS sq ON e.session_id = sq.session_id")
		params = append(params, subqParams...)
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
	metrics := make([]string, 0, len(query.Metrics))

	for _, metric := range query.Metrics {
		sql, err := Metrics.GetMetricSQL(AvailableMetrics, metric)
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
	metrics := make([]string, 0, len(query.Metrics))
	for _, metric := range query.Metrics {
		sql, err := Metrics.GetMetricSQL(AvailableMetrics, metric)
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

// addSpecialMetrics includes special metrics in the query
func (qb *QueryBuilder) addSpecialMetrics(sql string, site *types.Site, query *types.Query) string {
	// Add bounce rate calculation
	for _, metric := range query.Metrics {
		if metric == "bounce_rate" {
			// Add bounce rate subquery
			return strings.Replace(sql, "FROM events e", `FROM (
				SELECT
					e.*,
					COUNT(DISTINCT e2.event_id) OVER (PARTITION BY e.session_id) as session_event_count
				FROM events e
				LEFT JOIN events e2 ON e.session_id = e2.session_id
			) e WHERE session_event_count = 1`, 1)
		}
	}
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
	sqlBuilder.WriteString(qb.selectSessionMetrics(sessionsQuery))
	sqlBuilder.WriteString(" FROM sessions s")

	if whereClause != "" {
		sqlBuilder.WriteString(" WHERE " + whereClause)
	}

	// Add event join if needed
	if qb.sessionsJoinEvents(sessionsQuery) {
		eventSubquery, _, err := qb.buildEventSubquery(sessionsQuery)
		if err != nil {
			return "", nil, err
		}
		sqlBuilder.WriteString(" JOIN (" + eventSubquery + ") AS eq ON s.session_id = eq.session_id")
	}

	// Add grouping
	groupBy := qb.buildGroupBy("sessions", sessionsQuery)
	if groupBy != "" {
		sqlBuilder.WriteString(groupBy)
	}

	// Apply enterprise edition query hints
	sql := sqlBuilder.String()

	// Add special metrics
	sql = qb.addSpecialMetrics(sql, site, sessionsQuery)

	return sql, params, nil
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

// buildEventSubquery creates a subquery for event joins in session queries
func (qb *QueryBuilder) buildEventSubquery(query *types.Query) (string, []any, error) {
	whereBuilder := NewWhereBuilder(query.SiteID)
	whereBuilder.FilterSiteTimeRange("events", query.UTCTimeRange.Start, query.UTCTimeRange.End)

	for _, filter := range query.Filters {
		if err := whereBuilder.AddFilter("events", filter); err != nil {
			return "", nil, err
		}
	}

	whereClause, params := whereBuilder.Build()

	sql := fmt.Sprintf(
		"SELECT DISTINCT session_id FROM events WHERE %s",
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

// selectTotalRows adds total row count selection
func (qb *QueryBuilder) selectTotalRows(sql string) string {
	if !strings.Contains(sql, "SELECT") {
		return sql
	}

	// Add total_rows column using window function
	selectPos := strings.Index(sql, "SELECT") + 6
	return sql[:selectPos] + " COUNT(*) OVER () AS total_rows, " + sql[selectPos:]
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

// shortName generates a short alias for metrics/dimensions
func (qb *QueryBuilder) shortName(dimension string) string {
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

// DimensionToColumn converts dimension names to database column names
func (qb *QueryBuilder) DimensionToColumn(dimension, tableType, purpose string) string {
	switch dimension {
	case "event:name":
		return "name"
	case "event:page":
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
		return "device_type"
	case "event:screen_size":
		return "screen_size"
	case "event:country":
		return "country"
	case "event:region":
		return "region"
	case "event:city":
		return "city"
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
		return "device_type"
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
	case "visit:region":
		return "region"
	case "visit:city":
		return "city"
	case "visit:screen_size":
		return "screen_size"
	case "time:minute":
		if purpose == "group" {
			return "minute"
		}
		return "formatDateTime(toStartOfInterval(timestamp, INTERVAL 1 minute), '%Y-%m-%d %H:%i') as minute"
	case "time:hour":
		if purpose == "group" {
			return "hour"
		}
		return "formatDateTime(toStartOfInterval(timestamp, INTERVAL 1 HOUR), '%Y-%m-%d %H') as hour"
	case "time:day":
		if purpose == "group" {
			return "day"
		}
		return "formatDateTime(toStartOfInterval(timestamp, INTERVAL 1 DAY), '%Y-%m-%d') as day"
	case "time:week":
		if purpose == "group" {
			return "week"
		}
		return "formatDateTime(toStartOfInterval(timestamp, INTERVAL 1 WEEK), '%Y-%u') as week"
	case "time:month":
		if purpose == "group" {
			return "month"
		}
		return "formatDateTime(toStartOfInterval(timestamp, INTERVAL 1 MONTH), '%Y-%m') as month"
	case "time:quarter":
		if purpose == "group" {
			return "quarter"
		}
		return "formatDateTime(toStartOfInterval(timestamp, INTERVAL 3 MONTH), '%Y-Q%q') as quarter"
	case "time:year":
		if purpose == "group" {
			return "year"
		}
		return "formatDateTime(toStartOfInterval(timestamp, INTERVAL 1 YEAR), '%Y') as year"
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

// getSourceClause generates the CASE statement for source classification
func (qs *QueryBuilder) getSourceClause() (string, error) {
	db := globals.GetDB()
	if db == nil {
		panic("DB is not initialized")
	}
	searchEngines := db.Client.SearchEngines.Query().AllX(context.Background())

	clause := fmt.Sprintf(`
			CASE
				WHEN referrer_source = '' THEN 'Direct'
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
