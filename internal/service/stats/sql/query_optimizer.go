package sql

import (
	"strings"
	"time"

	"github.com/zenstats/zenstats/internal/service/stats/types"
)

const (
	TableEvents          = "events"
	TableSessions        = "sessions"
	TableSessionsSmeared = "sessions_smeared"
)

// SplitQuery is one physical ClickHouse table query produced by the pipeline.
type SplitQuery struct {
	TableType string
	Query     *types.Query
}

// QueryOptimizer handles query pipeline and table splitting logic.
type QueryOptimizer struct{}

// Optimize applies a series of transformations to the query.
func (qo *QueryOptimizer) Optimize(query *types.Query) *types.Query {
	if query.InputUTCTimeRange.Start.IsZero() && query.InputUTCTimeRange.End.IsZero() {
		query.InputUTCTimeRange = query.UTCTimeRange
	}

	pipeline := []func(*types.Query) *types.Query{
		qo.updateGroupByTime,
		qo.addMissingOrderBy,
		qo.updateTimeInOrderBy,
		qo.extendHostnameFiltersToVisit,
		qo.trimRelativeDateRange,
		qo.setTimeOnPageData,
		qo.removeTimeOnPageIfUnavailable,
		qo.setSQLJoinType,
	}

	return qo.applyPipeline(query, pipeline)
}

// Split separates the query into event/session/session-smeared components.
func (qo *QueryOptimizer) Split(query *types.Query) []*SplitQuery {
	metricsWithVisitors := qo.maybeAddVisitorsMetric(query.Metrics)
	eventMetrics, sessionMetrics, _ := qo.partitionMetrics(metricsWithVisitors, query)

	splits := make([]*SplitQuery, 0, 3)
	if len(eventMetrics) > 0 || qo.eventsJoinSessions(query) {
		eq := qo.copyQuery(query)
		eq.Metrics = eventMetrics
		splits = append(splits, &SplitQuery{TableType: TableEvents, Query: eq})
	}

	if len(sessionMetrics) > 0 || qo.sessionsJoinEvents(query) {
		sq := qo.splitSessionsQuery(query, sessionMetrics)
		tableType := TableSessions
		if query.SmearSessionMetrics && qo.canUseSessionsSmeared(sq) {
			tableType = TableSessionsSmeared
		}
		splits = append(splits, &SplitQuery{TableType: tableType, Query: sq})
	}

	return splits
}

func (qo *QueryOptimizer) copyQuery(query *types.Query) *types.Query {
	clone := *query
	clone.Metrics = append([]string(nil), query.Metrics...)
	clone.Dimensions = append([]string(nil), query.Dimensions...)
	clone.OrderBy = append([]*types.OrderBy(nil), query.OrderBy...)
	clone.Filters = make([]*types.Filter, 0, len(query.Filters))
	for _, f := range query.Filters {
		clone.Filters = append(clone.Filters, f.Clone())
	}
	return &clone
}

func (qo *QueryOptimizer) maybeAddVisitorsMetric(metrics []string) []string {
	if containsMetric(metrics, "visitors") {
		return metrics
	}
	return append([]string{"visitors"}, metrics...)
}

func containsMetric(metrics []string, name string) bool {
	for _, m := range metrics {
		if m == name {
			return true
		}
	}
	return false
}

func (qo *QueryOptimizer) partitionMetrics(metrics []string, query *types.Query) ([]string, []string, []string) {
	var eventMetrics, sessionMetrics, otherMetrics, eitherMetrics []string
	for _, metric := range metrics {
		switch qo.metricCategory(metric, query) {
		case "event":
			eventMetrics = append(eventMetrics, metric)
		case "session":
			sessionMetrics = append(sessionMetrics, metric)
		case "either":
			eitherMetrics = append(eitherMetrics, metric)
		default:
			otherMetrics = append(otherMetrics, metric)
		}
	}

	eventOnlyFilters, sessionOnlyFilters := qo.classifyFilters(query.Filters)
	eventOnlyDimensions, sessionOnlyDimensions := qo.classifyDimensions(query.Dimensions)
	onlySession := len(eventMetrics) == 0 && len(eventOnlyFilters) == 0 && len(eventOnlyDimensions) == 0
	onlyEvent := len(sessionMetrics) == 0 && len(sessionOnlyFilters) == 0 && len(sessionOnlyDimensions) == 0

	if onlySession {
		return nil, append(sessionMetrics, eitherMetrics...), otherMetrics
	}
	if onlyEvent {
		return append(eventMetrics, eitherMetrics...), nil, otherMetrics
	}
	if len(eventMetrics) == 0 && len(eventOnlyDimensions) == 0 {
		return nil, append(sessionMetrics, eitherMetrics...), otherMetrics
	}
	if len(sessionMetrics) == 0 && len(sessionOnlyDimensions) == 0 {
		return append(eventMetrics, eitherMetrics...), nil, otherMetrics
	}

	return append(eventMetrics, eitherMetrics...), sessionMetrics, otherMetrics
}

func (qo *QueryOptimizer) metricCategory(metricName string, query *types.Query) string {
	if metricName == "visitors" || metricName == "visits" {
		if qo.hasTimeDimension(query, "time:minute") || qo.hasTimeDimension(query, "time:hour") {
			return "session"
		}
		return "either"
	}
	if strings.HasPrefix(metricName, "event:") {
		return "event"
	}
	if strings.HasPrefix(metricName, "session:") {
		return "session"
	}
	switch metricName {
	case "conversion_rate", "group_conversion_rate", "percentage":
		return "either"
	case "conversions", "revenue", "total_revenue", "average_revenue", "time_on_page", "scroll_depth", "pageviews", "events":
		return "event"
	case "visit_duration", "views_per_visit", "bounce_rate", "exit_rate":
		// exit_rate 基于 sum(sign)，只能在 sessions 表计算；归为 session 可避免其被错误
		// 放入 events 表查询而报 "Unknown identifier sign"。
		return "session"
	case "total_visitors":
		return "other"
	default:
		return "either"
	}
}

func (qo *QueryOptimizer) classifyFilters(filters []*types.Filter) (eventFilters, sessionFilters []*types.Filter) {
	for _, filter := range filters {
		if filter.AnyDimension(func(dim string) bool { return strings.HasPrefix(dim, "event:") }) {
			eventFilters = append(eventFilters, filter)
		}
		if filter.AnyDimension(isSessionOnlyDimension) {
			sessionFilters = append(sessionFilters, filter)
		}
	}
	return eventFilters, sessionFilters
}

func (qo *QueryOptimizer) classifyDimensions(dimensions []string) (eventDimensions, sessionDimensions []string) {
	for _, dim := range dimensions {
		if strings.HasPrefix(dim, "event:") {
			eventDimensions = append(eventDimensions, dim)
		} else if isSessionOnlyDimension(dim) {
			sessionDimensions = append(sessionDimensions, dim)
		}
	}
	return eventDimensions, sessionDimensions
}

func (qo *QueryOptimizer) splitSessionsQuery(query *types.Query, sessionMetrics []string) *types.Query {
	sq := qo.copyQuery(query)
	sq.Metrics = sessionMetrics
	for i, dim := range sq.Dimensions {
		if dim == "event:page" {
			sq.Dimensions[i] = "visit:entry_page"
		}
	}
	// exit_rate 语义是"结束在该页的会话占比"，需按退出页而非入口页归组。
	// 维度仍用 visit:entry_page（保证别名/JOIN/结果列名统一为 page），仅把底层列换成 exit_page。
	if containsMetric(sessionMetrics, "exit_rate") {
		sq.SessionPageColumn = "exit_page"
	}
	sq.Filters = qo.updateSessionFilters(sq.Filters)
	return sq
}

func (qo *QueryOptimizer) updateSessionFilters(filters []*types.Filter) []*types.Filter {
	updated := make([]*types.Filter, 0, len(filters))
	for _, filter := range filters {
		updated = append(updated, filter.Clone())
	}
	return updated
}

func (qo *QueryOptimizer) applyPipeline(query *types.Query, pipeline []func(*types.Query) *types.Query) *types.Query {
	result := query
	for _, transform := range pipeline {
		result = transform(result)
	}
	return result
}

func (qo *QueryOptimizer) setTimeOnPageData(query *types.Query) *types.Query {
	if query.TimeOnPageData == (types.TimeOnPageData{}) || !containsMetric(query.Metrics, "time_on_page") {
		return query
	}
	if query.TimeOnPageData.NewMetricVisible {
		location, err := time.LoadLocation(query.Timezone)
		if err != nil {
			return query
		}
		cutoffLocal := time.Date(query.TimeOnPageData.CutoffDate.Year(), query.TimeOnPageData.CutoffDate.Month(), query.TimeOnPageData.CutoffDate.Day(), 0, 0, 0, 0, location)
		cutoffUTC := cutoffLocal.In(time.UTC).Truncate(time.Second)
		query.TimeOnPageData.IncludeNewMetric = query.UTCTimeRange.End.After(cutoffUTC)
		query.TimeOnPageData.Cutoff = cutoffUTC
	}
	return query
}

func (qo *QueryOptimizer) removeTimeOnPageIfUnavailable(query *types.Query) *types.Query {
	if !query.DropTimeOnPageMetric || !containsMetric(query.Metrics, "time_on_page") {
		return query
	}
	metrics := make([]string, 0, len(query.Metrics))
	for _, metric := range query.Metrics {
		if metric != "time_on_page" {
			metrics = append(metrics, metric)
		}
	}
	query.Metrics = metrics
	return query
}

func (qo *QueryOptimizer) trimRelativeDateRange(query *types.Query) *types.Query {
	if query.Now.IsZero() || query.UTCTimeRange.End.Before(query.Now) {
		return query
	}
	query.UTCTimeRange.End = query.Now
	return query
}

func (qo *QueryOptimizer) setSQLJoinType(query *types.Query) *types.Query {
	if query.SQLJoinType != "" {
		return query
	}
	if qo.hasTimeDimension(query, "time:minute") || qo.hasTimeDimension(query, "time:hour") {
		query.SQLJoinType = "FULL OUTER"
		query.SmearSessionMetrics = !types.HasEventGoalFilter(query.Filters)
	} else {
		query.SQLJoinType = "LEFT"
	}
	return query
}

func (qo *QueryOptimizer) updateGroupByTime(query *types.Query) *types.Query {
	for i, dim := range query.Dimensions {
		if dim == "time" {
			query.Dimensions[i] = qo.resolveTimeDimension(query.UTCTimeRange.Start, query.UTCTimeRange.End)
		}
	}
	return query
}

func (qo *QueryOptimizer) resolveTimeDimension(first, last time.Time) string {
	duration := last.Sub(first)
	switch {
	case duration.Hours() <= 48:
		return "time:hour"
	case duration.Hours() <= 960:
		return "time:day"
	case duration.Hours() <= 8760:
		return "time:week"
	default:
		return "time:month"
	}
}

func (qo *QueryOptimizer) addMissingOrderBy(query *types.Query) *types.Query {
	if len(query.OrderBy) == 0 && len(query.Metrics) > 0 {
		if timeDim := qo.timeDimension(query); timeDim != "" {
			query.OrderBy = append(query.OrderBy, &types.OrderBy{Dimension: timeDim, Direction: "asc"})
		}
		query.OrderBy = append(query.OrderBy, &types.OrderBy{Dimension: query.Metrics[0], Direction: "desc"})
	}
	return query
}

func (qo *QueryOptimizer) timeDimension(query *types.Query) string {
	for _, dim := range query.Dimensions {
		if strings.HasPrefix(dim, "time:") {
			return dim
		}
	}
	return ""
}

func (qo *QueryOptimizer) updateTimeInOrderBy(query *types.Query) *types.Query {
	timeDim := qo.timeDimension(query)
	if timeDim == "" {
		return query
	}
	for _, ob := range query.OrderBy {
		if ob.Dimension == "time" {
			ob.Dimension = timeDim
		}
	}
	return query
}

func (qo *QueryOptimizer) extendHostnameFiltersToVisit(query *types.Query) *types.Query {
	// Foreign predicates are compiled as session membership filters. Adding an
	// entry-hostname predicate here would change OR and multi-host visit semantics.
	return query
}

func (qo *QueryOptimizer) eventsJoinSessions(query *types.Query) bool {
	return (&TableDecider{}).EventsJoinSessions(query)
}
func (qo *QueryOptimizer) sessionsJoinEvents(query *types.Query) bool {
	return (&TableDecider{}).SessionsJoinEvents(query)
}

func (qo *QueryOptimizer) hasTimeDimension(query *types.Query, dim string) bool {
	for _, d := range query.Dimensions {
		if d == dim {
			return true
		}
	}
	return false
}

func (qo *QueryOptimizer) canUseSessionsSmeared(query *types.Query) bool {
	return qo.hasTimeDimension(query, "time:minute") || qo.hasTimeDimension(query, "time:hour")
}

func isSessionOnlyDimension(dim string) bool {
	switch dim {
	case "visit:entry_page", "visit:entry_page_hostname", "visit:exit_page", "visit:exit_page_hostname":
		return true
	default:
		return strings.HasPrefix(dim, "visit:entry_props:")
	}
}
