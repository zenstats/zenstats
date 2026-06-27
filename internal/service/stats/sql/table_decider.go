package sql

import (
	"fmt"
	"strings"

	"github.com/zenstats/zenstats/internal/service/stats/types"
)

// TableDecider 决定事件查询和会话查询之间的连接方式及字段选择
type TableDecider struct{}

// JoinType 确定事件查询和会话查询之间的连接类型
func (td *TableDecider) JoinType(eventQuery, sessionQuery *types.Query) string {
	if eventQuery != nil && eventQuery.SQLJoinType != "" {
		return eventQuery.SQLJoinType
	}
	if sessionQuery != nil && sessionQuery.SQLJoinType != "" {
		return sessionQuery.SQLJoinType
	}
	if eventQuery == nil || sessionQuery == nil {
		return "LEFT"
	}
	if len(eventQuery.Dimensions) > 0 && len(sessionQuery.Dimensions) > 0 {
		// 如果两个查询都有维度，使用内连接
		return "INNER"
	} else if len(eventQuery.Dimensions) > 0 {
		// 只有事件查询有维度，使用左连接
		return "LEFT"
	} else if len(sessionQuery.Dimensions) > 0 {
		// 只有会话查询有维度，使用右连接
		return "RIGHT"
	}
	// 没有维度时使用全连接
	return "FULL OUTER"
}

// EventFields 选择事件查询的字段
// 验证指标与维度兼容性
func (td *TableDecider) ValidateConflicts(query *types.Query) error {
	var eventMetrics, sessionMetrics []string
	for _, m := range query.Metrics {
		if td.isEventMetric(query, m) {
			eventMetrics = append(eventMetrics, m)
		} else {
			sessionMetrics = append(sessionMetrics, m)
		}
	}

	var eventDims, sessionDims []string
	for _, d := range query.Dimensions {
		if td.isEventDimension(d) {
			eventDims = append(eventDims, d)
		} else {
			sessionDims = append(sessionDims, d)
		}
	}

	// 特殊处理事件页面维度
	if len(eventDims) == 1 && eventDims[0] == "event:page" {
		return nil
	}

	if len(sessionMetrics) > 0 && len(eventDims) > 0 {
		return fmt.Errorf("Session metric(s) %s cannot be queried with event dimension(s) %s", strings.Join(sessionMetrics, ", "), strings.Join(eventDims, ", "))
	}

	if len(eventMetrics) > 0 && len(sessionDims) > 0 {
		return fmt.Errorf("Event metric(s) %s cannot be queried with session dimension(s) %s", strings.Join(eventMetrics, ", "), strings.Join(sessionDims, ", "))
	}

	return nil
}

func (td *TableDecider) EventFields(query *types.Query) []string {
	fields := make([]string, 0)

	// 仅添加事件相关指标
	for _, metric := range query.Metrics {
		if td.isEventMetric(query, metric) {
			fields = append(fields, metric)
		}
	}

	// 仅添加事件相关维度
	for _, dim := range query.Dimensions {
		if td.isEventDimension(dim) {
			fields = append(fields, td.shortName(dim))
		}
	}

	return fields
}

// 判断指标是否属于事件表
func (td *TableDecider) isEventMetric(query *types.Query, metric string) bool {
	// 处理遗留逻辑
	if metric == "pageviews" || metric == "events" {
		return true
	}

	// 处理特殊指标逻辑
	if metric == "visitors" || metric == "visits" {
		for _, dim := range query.Dimensions {
			if dim == "time:minute" {
				return false
			}
		}
		return true
	}

	// 排除采样因子指标
	if metric == "sample_percent" {
		return false
	}

	switch metric {
	case "pageviews", "events", "scroll_depth", "time_on_page", "average_revenue", "total_revenue", "conversion_rate", "group_conversion_rate", "percentage":
		return true
	default:
		return false
	}
}

// 判断维度是否属于事件表
func (td *TableDecider) isEventDimension(dimension string) bool {
	// 特殊处理事件页面维度
	if dimension == "event:page" {
		return true
	}

	if strings.HasPrefix(dimension, "event:") {
		return true
	}
	// 处理visit:前缀维度
	if strings.HasPrefix(dimension, "visit:") {
		switch dimension {
		case "visit:entry_page", "visit:entry_page_hostname", "visit:exit_page", "visit:exit_page_hostname":
			return false
		default:
			return true
		}
	}
	return false
}

// SessionFields 选择会话查询的字段
func (td *TableDecider) SessionFields(query *types.Query) []string {
	fields := make([]string, 0)

	// 添加指标字段
	for _, metric := range query.Metrics {
		fields = append(fields, metric)
	}

	// 添加维度字段
	for _, dim := range query.Dimensions {
		fields = append(fields, td.shortName(dim))
	}

	return fields
}

// shortName 生成维度的短名称
func (td *TableDecider) shortName(dimension string) string {
	if dimension == "event:page" || dimension == "visit:entry_page" {
		return "page"
	}
	// Geographic dimensions use name columns for JOIN conditions
	switch dimension {
	case "event:country", "visit:country":
		return "country_name"
	case "event:region", "visit:region":
		return "continent_name"
	case "event:city", "visit:city":
		return "city_name"
	}
	parts := strings.Split(dimension, ":")
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return dimension
}

// 需要连接的表
func (td *TableDecider) NeedsJoin(eventQuery, sessionQuery *types.Query) bool {
	return len(eventQuery.Metrics) > 0 && len(sessionQuery.Metrics) > 0
}

// 事件查询是否需要连接会话表
func (td *TableDecider) EventsJoinSessions(query *types.Query) bool {
	for _, dim := range query.Dimensions {
		if strings.HasPrefix(dim, "visit:") && dim != "visit:entry_page" {
			return true
		}
	}
	for _, filter := range query.Filters {
		if filter.AnyDimension(func(dim string) bool { return isSessionOnlyDimension(dim) }) {
			return true
		}
	}
	return false
}

// 会话查询是否需要连接事件表
func (td *TableDecider) SessionsJoinEvents(query *types.Query) bool {
	for _, dim := range query.Dimensions {
		if strings.HasPrefix(dim, "event:") {
			return true
		}
	}
	for _, filter := range query.Filters {
		if filter.AnyDimension(func(dim string) bool { return strings.HasPrefix(dim, "event:") }) {
			return true
		}
	}
	return false
}
