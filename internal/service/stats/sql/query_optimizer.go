package sql

import (
	"strings"
	"time"

	"github.com/zenstats/zenstats/internal/service/stats/types"
)

// QueryOptimizer handles query optimization and splitting logic
type QueryOptimizer struct{}

// Optimize applies a series of transformations to the query
func (qo *QueryOptimizer) Optimize(query *types.Query) *types.Query {
	pipeline := []func(*types.Query) *types.Query{
		qo.updateGroupByTime,
		qo.addMissingOrderBy,
		qo.updateTimeInOrderBy,
		qo.extendHostnameFiltersToVisit,
		qo.setTimeOnPageData,
	}

	return qo.applyPipeline(query, pipeline)
}

// Split separates the query into event and session components
func (qo *QueryOptimizer) Split(query *types.Query) (*types.Query, *types.Query) {
	// Add visitors metric if needed
	metricsWithVisitors := qo.maybeAddVisitorsMetric(query.Metrics)

	// Partition metrics using TableDecider logic
	eventMetrics, sessionMetrics, _ := qo.partitionMetrics(metricsWithVisitors, query)

	// Create event query
	eventQuery := types.Query{
		SiteID:                 query.SiteID,
		Metrics:                eventMetrics,
		Dimensions:             query.Dimensions,
		Filters:                query.Filters,
		UTCTimeRange:           query.UTCTimeRange,
		ComparisonUTCTimeRange: query.ComparisonUTCTimeRange,
		Interval:               query.Interval,
		Period:                 query.Period,
		Date:                   query.Date,
		From:                   query.From,
		To:                     query.To,
		Now:                    query.Now,
		Timezone:               query.Timezone,
		Pagination:             query.Pagination,
		OrderBy:                query.OrderBy,
	}

	// Create session query with modified dimensions
	sessionQuery := qo.splitSessionsQuery(query, sessionMetrics)

	return &eventQuery, sessionQuery
}

// maybeAddVisitorsMetric ensures visitors metric is present when needed
func (qo *QueryOptimizer) maybeAddVisitorsMetric(metrics []string) []string {
	// Check if visitors metric already exists
	if containsMetric(metrics, "visitors") {
		return metrics
	}

	// Add visitors metric as first metric for other cases
	return append([]string{"visitors"}, metrics...)
}

// containsMetric checks if a metric exists in the list
func containsMetric(metrics []string, name string) bool {
	for _, m := range metrics {
		if m == name {
			return true
		}
	}
	return false
}

// partitionMetrics separates metrics into event, session, either, other and sample_percent categories
func (qo *QueryOptimizer) partitionMetrics(metrics []string, query *types.Query) ([]string, []string, []string) {
	var eventMetrics, sessionMetrics, otherMetrics []string
	var eitherMetrics []string
	//views_per_visit bounce_rate visit_duration events visitors pageviews
	for _, metric := range metrics {
		category := qo.metricCategory(metric, query)

		switch category {
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

	// Determine final metric assignment based on query context
	eventOnlyFilters, sessionOnlyFilters := qo.classifyFilters(query.Filters)
	eventOnlyDimensions, sessionOnlyDimensions := qo.classifyDimensions(query.Dimensions)

	// Check if only one table needs to be queried
	onlySession := len(eventMetrics) == 0 && len(eventOnlyFilters) == 0 && len(eventOnlyDimensions) == 0
	onlyEvent := len(sessionMetrics) == 0 && len(sessionOnlyFilters) == 0 && len(sessionOnlyDimensions) == 0

	if onlySession {
		return []string{}, append(sessionMetrics, eitherMetrics...), otherMetrics
	} else if onlyEvent {
		return append(eventMetrics, eitherMetrics...), []string{}, otherMetrics
	}

	// Handle mixed cases with filters and dimensions
	if len(eventMetrics) == 0 && len(eventOnlyDimensions) == 0 {
		return []string{}, append(sessionMetrics, eitherMetrics...), otherMetrics
	} else if len(sessionMetrics) == 0 && len(sessionOnlyDimensions) == 0 {
		return append(eventMetrics, eitherMetrics...), []string{}, otherMetrics
	}

	// Default: prefer events for mixed cases
	return append(eventMetrics, eitherMetrics...), sessionMetrics, otherMetrics
}

// metricCategory determines which category a metric belongs to
func (qo *QueryOptimizer) metricCategory(metricName string, query *types.Query) string {

	// Time:minute dimension special case
	if metricName == "visitors" || metricName == "sessions" {
		for _, dim := range query.Dimensions {
			if dim == "time:minute" {
				return "session"
			}
		}
		return "either"
	}

	// Handle prefixed metrics
	if strings.HasPrefix(metricName, "event:") {
		return "event"
	} else if strings.HasPrefix(metricName, "session:") {
		return "session"
	}
	// Specific metric classifications
	switch metricName {
	case "conversion_rate", "group_conversion_rate", "percentage":
		return "either"
	case "visitors", "conversions", "revenue", "time_on_page":
		return "event"
	case "events", "pageviews", "visit_duration", "views_per_visit", "bounce_rate":
		return "session"
	case "total_visitors":
		return "other"
	default:
		return "either"
	}
}

// classifyFilters determines which filters belong to event or session tables
func (qo *QueryOptimizer) classifyFilters(filters []*types.Filter) (eventFilters, sessionFilters []*types.Filter) {
	for _, filter := range filters {
		if strings.HasPrefix(filter.Dimension, "event:") {
			eventFilters = append(eventFilters, filter)
		} else if filter.Dimension == "visit:entry_page" ||
			filter.Dimension == "visit:entry_page_hostname" ||
			filter.Dimension == "visit:exit_page" ||
			filter.Dimension == "visit:exit_page_hostname" {
			sessionFilters = append(sessionFilters, filter)
		}
	}
	return eventFilters, sessionFilters
}

// classifyDimensions determines which dimensions belong to event or session tables
func (qo *QueryOptimizer) classifyDimensions(dimensions []string) (eventDimensions, sessionDimensions []string) {
	for _, dim := range dimensions {
		if strings.HasPrefix(dim, "event:") {
			eventDimensions = append(eventDimensions, dim)
		} else if dim == "visit:entry_page" ||
			dim == "visit:entry_page_hostname" ||
			dim == "visit:exit_page" ||
			dim == "visit:exit_page_hostname" {
			sessionDimensions = append(sessionDimensions, dim)
		}
	}
	return eventDimensions, sessionDimensions
}

// splitSessionsQuery creates a session-specific query
func (qo *QueryOptimizer) splitSessionsQuery(query *types.Query, sessionMetrics []string) *types.Query {
	// Replace event:page dimension with visit:entry_page
	dimensions := make([]string, len(query.Dimensions))
	for i, dim := range query.Dimensions {
		if dim == "event:page" {
			dimensions[i] = "visit:entry_page"
		} else {
			dimensions[i] = dim
		}
	}

	// Update filters for session query
	filters := qo.updateSessionFilters(query.Filters)

	return &types.Query{
		SiteID:                 query.SiteID,
		Metrics:                sessionMetrics,
		Dimensions:             dimensions,
		Filters:                filters,
		UTCTimeRange:           query.UTCTimeRange,
		ComparisonUTCTimeRange: query.ComparisonUTCTimeRange,
		Interval:               query.Interval,
		Period:                 query.Period,
		Date:                   query.Date,
		From:                   query.From,
		To:                     query.To,
		Now:                    query.Now,
		Timezone:               query.Timezone,
		Pagination:             query.Pagination,
		OrderBy:                query.OrderBy,
	}
}

// updateSessionFilters updates filters for session queries
func (qo *QueryOptimizer) updateSessionFilters(filters []*types.Filter) []*types.Filter {
	updatedFilters := make([]*types.Filter, len(filters))
	for i, filter := range filters {
		if filter.Dimension == "event:page" {
			updatedFilter := *filter
			updatedFilter.Dimension = "visit:entry_page"
			updatedFilters[i] = &updatedFilter
		} else {
			updatedFilters[i] = filter
		}
	}
	return updatedFilters
}

// applyPipeline applies a series of transformations to the query
func (qo *QueryOptimizer) applyPipeline(query *types.Query, pipeline []func(*types.Query) *types.Query) *types.Query {
	result := query
	for _, transform := range pipeline {
		result = transform(result)
	}
	return result
}

// setTimeOnPageData configures time on page data handling
func (qo *QueryOptimizer) setTimeOnPageData(query *types.Query) *types.Query {
	if query.TimeOnPageData == (types.TimeOnPageData{}) {
		return query
	}

	// Check if time_on_page metric is requested
	timeOnPageRequested := false
	for _, metric := range query.Metrics {
		if metric == "time_on_page" {
			timeOnPageRequested = true
			break
		}
	}

	if timeOnPageRequested && query.TimeOnPageData.NewMetricVisible {
		// Convert cutoff date to UTC based on site timezone
		location, err := time.LoadLocation(query.Timezone)
		if err != nil {
			return query
		}

		// Create cutoff datetime in site timezone
		cutoffLocal := time.Date(
			query.TimeOnPageData.CutoffDate.Year(),
			query.TimeOnPageData.CutoffDate.Month(),
			query.TimeOnPageData.CutoffDate.Day(),
			0, 0, 0, 0,
			location,
		)

		// Convert to UTC and truncate
		cutoffUTC := cutoffLocal.In(time.UTC).Truncate(time.Second)

		// Update time on page data flags
		query.TimeOnPageData.IncludeNewMetric = query.UTCTimeRange.End.After(cutoffUTC)
		query.TimeOnPageData.Cutoff = cutoffUTC
	}

	return query
}

// updateGroupByTime determines the appropriate time granularity
func (qo *QueryOptimizer) updateGroupByTime(query *types.Query) *types.Query {
	if len(query.Dimensions) == 0 {
		return query
	}

	newDimensions := make([]string, len(query.Dimensions))
	for i, dim := range query.Dimensions {
		if dim == "time" {
			newDimensions[i] = qo.resolveTimeDimension(query.UTCTimeRange.Start, query.UTCTimeRange.End)
		} else {
			newDimensions[i] = dim
		}
	}

	query.Dimensions = newDimensions
	return query
}

// resolveTimeDimension determines the appropriate time granularity
func (qo *QueryOptimizer) resolveTimeDimension(first, last time.Time) string {
	duration := last.Sub(first)

	switch {
	case duration.Hours() <= 48:
		return "time:hour"
	case duration.Hours() <= 960: // 40 days
		return "time:day"
	case duration.Hours() <= 8760: // 365 days
		return "time:week"
	default:
		return "time:month"
	}
}

// addMissingOrderBy adds default ordering if not specified
func (qo *QueryOptimizer) addMissingOrderBy(query *types.Query) *types.Query {
	if len(query.OrderBy) == 0 && len(query.Metrics) > 0 {
		var orderBy []*types.OrderBy
		timeDim := qo.timeDimension(query)

		if timeDim != "" {
			orderBy = append(orderBy, &types.OrderBy{
				Dimension: timeDim,
				Direction: "asc",
			})
		}

		orderBy = append(orderBy, &types.OrderBy{
			Dimension: query.Metrics[0],
			Direction: "desc",
		})

		query.OrderBy = orderBy
	}
	return query
}

// timeDimension finds the time dimension in the query
func (qo *QueryOptimizer) timeDimension(query *types.Query) string {
	for _, dim := range query.Dimensions {
		if strings.HasPrefix(dim, "time:") {
			return dim
		}
	}
	return ""
}

// updateTimeInOrderBy updates time dimension in order by clause
func (qo *QueryOptimizer) updateTimeInOrderBy(query *types.Query) *types.Query {
	if len(query.OrderBy) == 0 {
		return query
	}

	timeDim := qo.timeDimension(query)
	if timeDim == "" {
		return query
	}

	updatedOrderBy := make([]*types.OrderBy, len(query.OrderBy))
	for i, ob := range query.OrderBy {
		if ob.Dimension == "time" {
			updatedOrderBy[i] = &types.OrderBy{
				Dimension: timeDim,
				Direction: ob.Direction,
			}
		} else {
			updatedOrderBy[i] = &types.OrderBy{
				Dimension: ob.Dimension,
				Direction: ob.Direction,
			}
		}
	}

	query.OrderBy = updatedOrderBy
	return query
}

// extendHostnameFiltersToVisit extends hostname filters to visit level
func (qo *QueryOptimizer) extendHostnameFiltersToVisit(query *types.Query) *types.Query {
	hostnameFilters := []*types.Filter{}
	for _, f := range query.Filters {
		if f.Dimension == "event:hostname" {
			hostnameFilters = append(hostnameFilters, f)
		}
	}

	if len(hostnameFilters) == 0 {
		return query
	}

	dimensionsHostnameMap := map[string]string{
		"visit:source":       "visit:entry_page_hostname",
		"visit:entry_page":   "visit:entry_page_hostname",
		"visit:utm_medium":   "visit:entry_page_hostname",
		"visit:utm_source":   "visit:entry_page_hostname",
		"visit:utm_campaign": "visit:entry_page_hostname",
		"visit:utm_content":  "visit:entry_page_hostname",
		"visit:utm_term":     "visit:entry_page_hostname",
		"visit:referrer":     "visit:entry_page_hostname",
		"visit:exit_page":    "visit:exit_page_hostname",
	}

	extraFilters := []*types.Filter{}
	for _, dim := range query.Dimensions {
		if mappedDim, ok := dimensionsHostnameMap[dim]; ok {
			for _, hf := range hostnameFilters {
				extraFilters = append(extraFilters, &types.Filter{
					Dimension: mappedDim,
					Operator:  hf.Operator,
					Values:    hf.Values,
				})
			}
		}
	}

	query.Filters = append(query.Filters, extraFilters...)
	return query
}
