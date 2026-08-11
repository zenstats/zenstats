package stats

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/zenstats/zenstats/internal/service/stats/sql"
	"github.com/zenstats/zenstats/internal/service/stats/types"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent/goal"
	"github.com/zenstats/zenstats/pkg/globals"
)

// ValidationError represents a validation error with a status code and message
type ValidationError struct {
	StatusCode int
	Message    string
}

// Error returns the error message
func (e *ValidationError) Error() string {
	return e.Message
}

// validatePeriod checks if the period parameter is valid
// periodShorthandRe 匹配 Plausible 风格的相对区间简写：Nd（最近 N 天）、Nmo（最近 N 个月）。
var periodShorthandRe = regexp.MustCompile(`^\d+(d|mo)$`)

func validatePeriod(params *types.Params) error {
	validPeriods := map[string]bool{
		"day": true, "yesterday": true, "w": true, "m": true,
		"p7": true, "p14": true, "p30": true, "custom": true, "realtime": true,
		// 与 Plausible 对齐的日历/相对区间
		"month": true, "6mo": true, "12mo": true, "year": true, "all": true,
	}
	if params.Period == "" || validPeriods[params.Period] || periodShorthandRe.MatchString(params.Period) {
		return nil
	}
	return &ValidationError{
		StatusCode: 400,
		Message:    "Error parsing `period` parameter: invalid period `" + params.Period + "`",
	}
}

// validateIntervals checks if the intervals parameter is valid
func validateIntervals(params *types.Params) error {
	validIntervals := map[string]bool{"minute": true, "hourly": true, "daily": true, "weekly": true, "monthly": true, "yearly": true}
	if params.Interval != "" && !validIntervals[params.Interval] {
		return errors.New("invalid interval: " + params.Interval + ", must be one of 'minute' 'hourly', 'daily', 'weekly', 'monthly', 'yearly'")
	}
	return nil
}

// validatePagination checks if the pagination parameter is valid
func validatePagination(params *types.Params) error {
	if params.Pagination != nil {
		if params.Pagination.Limit < 0 || params.Pagination.Limit > 100000 {
			return errors.New("pagination limit must be between 0 and 100000")
		}
		if params.Pagination.Offset < 0 {
			return errors.New("pagination offset cannot be negative")
		}
	}

	return nil
}

// validateDate checks if the date parameter is valid
func validateDate(params *types.Params) error {
	// Validate custom period date format
	if params.Period == "custom" {
		fromDate := strings.TrimSpace(params.From)
		toDate := strings.TrimSpace(params.To)

		// 支持日期时间格式（如 "2026-06-25 14:00"），提取日期部分校验
		if idx := strings.Index(fromDate, " "); idx > 0 {
			fromDate = fromDate[:idx]
		}
		if idx := strings.Index(toDate, " "); idx > 0 {
			toDate = toDate[:idx]
		}

		if _, err := time.Parse(time.DateOnly, fromDate); err != nil {
			return &ValidationError{
				StatusCode: 400,
				Message:    "Invalid format for `date` parameter. When using a custom period, please include two ISO-8601 formatted dates joined by a comma.",
			}
		}

		if _, err := time.Parse(time.DateOnly, toDate); err != nil {
			return &ValidationError{
				StatusCode: 400,
				Message:    "Invalid format for `date` parameter. When using a custom period, please include two ISO-8601 formatted dates joined by a comma.",
			}
		}

		return nil
	}
	if params.Period == "realtime" {
		return nil
	}

	// Validate single date format
	if _, err := time.Parse(time.DateOnly, params.Date); err != nil {
		return &ValidationError{
			StatusCode: 400,
			Message:    "Error parsing `date` parameter: " + err.Error() + ". Please specify a valid date in ISO-8601 format.",
		}
	}

	return nil
}

// validateDimensions checks if the dimensions parameter is valid
func validateDimensions(params *types.Params) error {
	// Check if property is valid
	for _, dim := range params.Dimensions {
		if !isValidDimension(dim) {
			return &ValidationError{
				StatusCode: 400,
				Message:    "Invalid property '#" + dim,
			}
		}
	}

	return nil
}

// validateFilters checks if the filters are valid
func validateFilters(site *types.Site, filters []*types.Filter) error {
	validOperators := map[string]bool{"is": true, "is_not": true, "contains": true, "contains_not": true, "matches": true, "matches_not": true, "matches_wildcard": true, "matches_wildcard_not": true}
	for _, f := range filters {

		if len(f.SubFilters) == 0 && f.Dimension == "" {
			return errors.New("filter dimension cannot be empty")
		}
		if len(f.SubFilters) == 0 && !validOperators[f.Operator] {
			return errors.New("invalid filter operator: " + f.Operator)
		}
		if len(f.SubFilters) == 0 && len(f.Values) == 0 {
			return errors.New("filter values cannot be empty")
		}
		if len(f.SubFilters) == 0 && !isValidDimension(f.Dimension) {
			return &ValidationError{
				StatusCode: 400,
				Message:    "Invalid filter property '#" + f.Dimension,
			}
		}
		// Special validation for event:goal filter
		if len(f.SubFilters) == 0 && strings.HasPrefix(f.Dimension, "event:goal") {
			if err := validateGoalFilter(site, f.Values); err != nil {
				return err
			}
		}
		if len(f.SubFilters) > 0 {
			if err := validateFilters(site, f.SubFilters); err != nil {
				return err
			}
		}

		// 验证过滤器值格式
		// prefixEnd := strings.Index(f.Dimension, ":")
		// if prefixEnd != -1 {
		// 	dimType := f.Dimension[:prefixEnd]
		// 	for _, val := range f.Values {
		// 		switch dimType {
		// 		case "numeric":
		// 			if strVal, ok := val.(string); ok {
		// 				if _, err := strconv.ParseFloat(strVal, 64); err != nil {
		// 					return errors.New("invalid numeric filter value: " + strVal)
		// 				}
		// 			}
		// 		case "date":
		// 			if strVal, ok := val.(string); ok {
		// 				if _, err := time.Parse("2006-01-02", strVal); err != nil {
		// 					return errors.New("invalid date filter value: " + strVal)
		// 				}
		// 			}
		// 		case "boolean":
		// 			if strVal, ok := val.(string); ok {
		// 				if strVal != "true" && strVal != "false" {
		// 					return errors.New("invalid boolean filter value: " + strVal)
		// 				}
		// 			}
		// 		}
		// 	}
		// }
	}

	return nil
}

// parseAndValidateMetrics parses and validates metrics from parameters
func parseAndValidateMetrics(params *types.Params) ([]string, error) {
	// Get metrics string, default to "visitors"
	metrics := params.Metrics
	if len(metrics) == 0 {
		metrics = []string{"visitors"}
	}
	// Validate metrics
	validatedMetrics, err := validateMetrics(metrics, params)
	if err != nil {
		return nil, &ValidationError{
			StatusCode: 400,
			Message:    err.Error(),
		}
	}

	// Convert to Metric objects
	result := make([]string, len(validatedMetrics))
	for i, m := range validatedMetrics {
		metric, err := MetricFromString(m)
		if err != nil {
			return nil, &ValidationError{
				StatusCode: 400,
				Message:    err.Error(),
			}
		}
		result[i] = metric
	}

	return result, nil
}

// ensureCustomPropsAccess checks if custom properties access is allowed
func ensureCustomPropsAccess(site *types.Site, params *types.Params) error {
	allowedProps := GetAllowedProps(site, true)
	propFilter := GetToplevelFilter(params, "event:props:")

	queryAllowed := false

	// Check if all properties are allowed
	if allowedProps == "all" {
		queryAllowed = true
	} else if len(propFilter) >= 3 {
		// Check prop filter
		prop, ok := propFilter[2].(string)
		if ok && strings.HasPrefix(prop, "event:props:") {
			propName := strings.TrimPrefix(prop, "event:props:")
			queryAllowed = isPropAllowed(allowedProps, propName)
		}
	} else if len(params.Dimensions) > 0 {
		// Check dimensions
		for _, dim := range params.Dimensions {
			if strings.HasPrefix(dim, "event:props:") {
				propName := strings.TrimPrefix(dim, "event:props:")
				queryAllowed = isPropAllowed(allowedProps, propName)
				break
			}
		}
	} else {
		// No custom props used
		queryAllowed = true
	}

	if !queryAllowed {
		return &ValidationError{
			StatusCode: 402,
			Message:    "The owner of this site does not have access to the custom properties feature",
		}
	}

	return nil
}

// Helper function to validate goal filter
func validateGoalFilter(site *types.Site, goalsInFilter []any) error {
	// When site is nil (CreateQuery pre-validation), skip goal existence check.
	// Goal resolution and validation is handled by the WHERE builder's addGoalFilter
	// via PostgreSQL subqueries at query execution time.
	if site == nil {
		return nil
	}

	configuredGoals := GetGoalsForSite(site)

	for _, goal := range goalsInFilter {
		goalStr, ok := goal.(string)
		if !ok {
			continue
		}
		goalStr = strings.TrimSpace(goalStr)
		if goalStr == "" {
			continue
		}

		found := false
		for _, configuredGoal := range configuredGoals {
			if configuredGoal == goalStr {
				found = true
				break
			}
		}

		if !found {
			var msg string
			if strings.HasPrefix(goalStr, "Visit ") {
				pagePath := strings.TrimPrefix(goalStr, "Visit ")
				msg = "The pageview goal for the pathname `" + pagePath + "` is not configured for this site."
			} else {
				msg = "The goal `" + goalStr + "` is not configured for this site."
			}

			return &ValidationError{
				StatusCode: 400,
				Message:    msg,
			}
		}
	}

	return nil
}

// isValidDimension checks if a dimension is valid
func isValidDimension(dimension string) bool {
	validDimensions := map[string]bool{
		"event:name":                true,
		"event:page":                true,
		"event:hostname":            true,
		"event:goal":                true,
		"event:referrer":            true,
		"event:referrerdomain":      true,
		"event:utm_source":          true,
		"event:utm_medium":          true,
		"event:utm_campaign":        true,
		"event:utm_content":         true,
		"event:utm_term":            true,
		"event:browser":             true,
		"event:browser_version":     true,
		"event:os":                  true,
		"event:os_version":          true,
		"event:device":              true,
		"event:screen_size":         true,
		"event:country":             true,
		"event:region":              true,
		"event:city":                true,
		"visit:source":              true,
		"visit:medium":              true,
		"visit:referrer":            true,
		"visit:channel":             true, // 获客渠道（采集时计算，存于 channel 列）
		// Plausible 规范的 UTM 维度名（数据与 event:utm_* 同列）
		"visit:utm_source":   true,
		"visit:utm_medium":   true,
		"visit:utm_campaign": true,
		"visit:utm_content":  true,
		"visit:utm_term":     true,
		"visit:device":              true,
		"visit:browser":             true,
		"visit:browser_version":     true,
		"visit:os":                  true,
		"visit:os_version":          true,
		"visit:country":             true,
		"visit:region":              true,
		"visit:city":                true,
		"visit:screen_size":         true,
		"visit:page":                 true, // alias for event:page
		"visit:entry_page":          true,
		"visit:entry_page_hostname": true,
		"visit:exit_page":           true,
		"visit:exit_page_hostname":  true,
		"time:minute":               true,
		"time:hour":                 true,
		"time:day":                  true,
		"time:week":                 true,
		"time:month":                true,
		"time:quarter":              true,
		"time:year":                 true,
		"time:day_of_week":          true,
		"time:day_of_year":          true,
	}

	return validDimensions[dimension] || strings.HasPrefix(dimension, "event:props:")
}

// validateMetrics validates a list of metrics
func validateMetrics(metrics []string, params *types.Params) ([]string, error) {
	// check for meterics in sql.Metrics
	err := sql.ValidateMetrics(metrics)
	if err != nil {
		return nil, err
	}

	// Check for duplicate metrics
	seen := make(map[string]bool)
	for _, m := range metrics {
		name := strings.TrimSpace(m)
		if name == "" {
			continue
		}
		if seen[name] {
			return nil, errors.New("metrics cannot be queried multiple times")
		}
		seen[name] = true
	}

	// Validate each metric
	validated := make([]string, 0, len(metrics))
	for _, m := range metrics {
		name := strings.TrimSpace(m)
		if name == "" {
			continue
		}
		if err := validateMetric(name, params); err != nil {
			return nil, err
		}

		validated = append(validated, name)
	}

	if err := validateBreakdownMetricCompatibility(validated, params); err != nil {
		return nil, err
	}

	if len(validated) == 0 {
		return nil, errors.New("at least one valid metric is required")
	}

	return validated, nil
}

func validateBreakdownMetricCompatibility(metrics []string, params *types.Params) error {
	if len(params.Dimensions) != 1 {
		return nil
	}

	dimension := params.Dimensions[0]
	if !isEventOnlyProperty(dimension) {
		return nil
	}

	for _, metric := range metrics {
		if isSessionOnlyMetric(metric) {
			return errors.New("session metric `" + metric + "` cannot be queried for breakdown by `" + dimension + "`.")
		}
	}

	return nil
}

func isSessionOnlyMetric(metric string) bool {
	switch metric {
	case "pageviews", "bounce_rate", "visit_duration", "views_per_visit":
		return true
	default:
		return false
	}
}

// validateMetric validates a single metric
func validateMetric(metric string, params *types.Params) error {

	// Special case for time_on_page
	if metric == "time_on_page" {
		// Check if filtering by event:goal
		if isFilteringOnDimension(params, "event:goal") {
			return errors.New("metric `time_on_page` cannot be queried when filtering by `event:goal`")
		}

		// Check if filtering by event:name
		if isFilteringOnDimension(params, "event:name") {
			return errors.New("metric `time_on_page` cannot be queried when filtering by `event:name`")
		}

		// Check if breakdown by dimension other than event:page
		if len(params.Dimensions) > 0 {
			if len(params.Dimensions) != 1 || params.Dimensions[0] != "event:page" {
				return errors.New("metric `time_on_page` is not supported in breakdown queries (except `event:page` breakdown)")
			}
		} else if !isFilteringOnDimension(params, "event:page") {
			// Not breakdown and not filtering by event:page
			return errors.New("metric `time_on_page` can only be queried in a page breakdown or with a page filter")
		}

		return nil
	}

	// Special case for conversion_rate
	if metric == "conversion_rate" || metric == "group_conversion_rate" {
		// Check if breakdown by event:goal or filtering by event:goal
		if !(len(params.Dimensions) == 1 && params.Dimensions[0] == "event:goal") && !isFilteringOnDimension(params, "event:goal") {
			return errors.New("metric `" + metric + "` can only be queried in a goal breakdown or with a goal filter")
		}

		return nil
	}

	// Special case for exit_rate
	// exit_rate 语义是"结束在各页面的会话占比"，只有按 event:page 拆分才有意义。
	// 仅带页面过滤（无维度）时无 GROUP BY，over() 分母=分子恒为 100%，且过滤会被改写到入口页，
	// 均为退化/错误结果，故收紧为必须按 event:page 拆分。
	if metric == "exit_rate" {
		if len(params.Dimensions) != 1 || params.Dimensions[0] != "event:page" {
			return errors.New("metric `exit_rate` is only supported with an `event:page` breakdown")
		}
		return nil
	}

	// Special case for scroll_depth
	if metric == "scroll_depth" {
		// Scroll depth only works with event:page dimension
		if len(params.Dimensions) > 0 && !(len(params.Dimensions) == 1 && params.Dimensions[0] == "event:page") {
			return errors.New("metric `scroll_depth` is only supported with `event:page` breakdown")
		}

		return nil
	}

	// percentage 只在细分（breakdown）下有意义：每个分组访客数占总访客的百分比。
	// 聚合查询下拒绝，与 Plausible 一致。
	if metric == "percentage" {
		if len(params.Dimensions) == 0 {
			return &ValidationError{
				StatusCode: 400,
				Message:    "metric `percentage` can only be queried with a dimension (breakdown)",
			}
		}
		return nil
	}

	// Basic metrics that don't need special validation
	// conversions 是事件维度指标（uniq user_id），在 goal 过滤下即转化人数，
	// 必须在此放行，否则会落入 validateSessionMetric 因 event:goal 过滤被误拒。
	// total_revenue/average_revenue 是事件级收入指标，同样在 events 表计算，直接放行。
	if metric == "visitors" || metric == "events" || metric == "conversions" ||
		metric == "total_revenue" || metric == "average_revenue" {
		return nil
	}

	// Special case for views_per_visit
	if metric == "views_per_visit" {
		// Check if filtering by event:page
		if isFilteringOnDimension(params, "event:page") {
			return errors.New("metric `views_per_visit` cannot be queried with a filter on `event:page`")
		}

		// Check if breakdown
		if len(params.Dimensions) > 0 {
			return errors.New("metric `views_per_visit` is not supported in breakdown queries")
		}
	}

	return validateSessionMetric(metric, params)
}

// validateSessionMetric validates session-based metrics
func validateSessionMetric(metric string, params *types.Params) error {
	// Check if breakdown by event-only property
	if len(params.Dimensions) == 1 {
		if isEventOnlyProperty(params.Dimensions[0]) {
			return errors.New("session metric `" + metric + "` cannot be queried for breakdown by `" + params.Dimensions[0] + "`.")
		}
	}

	// 仅对"必须在 sessions 表计算"的指标拦截事件专属过滤器：
	// 这些指标在拆分时落到 sessions 表，而 sessions 表无法按 event:name/goal/props 过滤，
	// 过滤器会被静默丢弃导致返回未过滤的错误数值。visits/pageviews/visitors 等落在 events 表，
	// 过滤器能正常生效，不应拦截。
	if isStrictlySessionMetric(metric) {
		if filter := findEventOnlyFilter(params); filter != "" {
			return errors.New("session metric `" + metric + "` cannot be queried when using a filter on `" + filter + "`.")
		}
	}

	return nil
}

// isStrictlySessionMetric 判断指标是否只能在 sessions 表计算（拆分时不会落到 events 表）。
func isStrictlySessionMetric(metric string) bool {
	switch metric {
	case "bounce_rate", "visit_duration", "views_per_visit":
		return true
	default:
		return false
	}
}

// Helper functions

func isFilteringOnDimension(params *types.Params, dimension string) bool {
	// 递归检测过滤器树中是否存在针对该维度的过滤条件
	for _, f := range params.Filters {
		if f.AnyDimension(func(dim string) bool { return dim == dimension }) {
			return true
		}
	}
	return false
}

// findEventOnlyFilter 返回第一个"事件专属"维度（event:name / event:goal / event:props:*）的过滤器维度名，
// 若无则返回 ""。会话表无法按这些事件级维度过滤，若与会话指标同时出现，
// 拆分查询会静默丢弃该过滤器并返回未过滤的错误数值——故据此在校验层直接拒绝（返回清晰的 400）。
func findEventOnlyFilter(params *types.Params) string {
	found := ""
	for _, f := range params.Filters {
		f.AnyDimension(func(dim string) bool {
			if isEventOnlyProperty(dim) {
				found = dim
				return true
			}
			return false
		})
		if found != "" {
			return found
		}
	}
	return ""
}

func isEventOnlyProperty(property string) bool {
	return property == "event:name" || property == "event:goal" || strings.HasPrefix(property, "event:props:")
}

func GetGoalsForSite(site *types.Site) []string {
	if site == nil {
		return []string{}
	}
	siteID, err := strconv.ParseInt(site.ID, 10, 64)
	if err != nil {
		return []string{}
	}
	db := globals.GetDB()
	if db == nil {
		return []string{}
	}
	ctx := context.Background()
	goals, err := db.Client.Goal.Query().
		Where(goal.SiteID(siteID)).
		All(ctx)
	if err != nil {
		return []string{}
	}
	names := make([]string, len(goals))
	for i, g := range goals {
		names[i] = g.DisplayName
	}
	return names
}

func GetAllowedProps(site *types.Site, bypassSetup bool) any {
	// Implementation would retrieve allowed properties for the site
	// This is a placeholder implementation
	return "all"
}

func isPropAllowed(allowedProps any, prop string) bool {
	// Implementation would check if property is allowed
	// This is a placeholder implementation
	return true
}

func GetToplevelFilter(params *types.Params, prefix string) []any {
	// Implementation would retrieve top-level filter with given prefix
	// This is a placeholder implementation
	return nil
}

// MetricFromString parses a string into a Metric
func MetricFromString(s string) (string, error) {
	s = strings.TrimSpace(s)
	switch s {
	case "visitors", "pageviews", "events", "visits", "bounce_rate", "visit_duration", "time_on_page", "exit_rate", "conversion_rate", "conversions", "views_per_visit", "scroll_depth", "percentage", "total_revenue", "average_revenue":
		return s, nil
	default:
		return "", fmt.Errorf("invalid metric: %s.", s)
	}
}
