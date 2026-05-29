package sql

import (
	"fmt"
	"strings"

	"github.com/zenstats/zenstats/internal/service/stats/types"
)

var (
	queryOptimizer = NewFragmentGenerator()
	uniq           = queryOptimizer.Uniq("user_id")
	total          = queryOptimizer.Total()
	bounceRate     = queryOptimizer.BounceRate()
	visitDuration  = queryOptimizer.VisitDuration()
	viewsPerVisit  = queryOptimizer.ViewsPerVisit()
	events         = queryOptimizer.EventsForEvent()
)

// Metric 定义统计指标元数据

// 所有支持的指标
type Metrics map[string]types.Metric

// 全局指标定义
var AvailableMetrics = Metrics{
	"visitors": {
		Name:        "visitors",
		Description: "Unique visitors to the site",
		SQLExpr:     fmt.Sprintf("%s as visitors", uniq.ToSql()),
		Valid:       true,
	},
	"pageviews": {
		Name:        "pageview",
		Description: "Total page views",
		SQLExpr:     fmt.Sprintf("%s as cur_pageviews", total.ToSql()),
		Valid:       true,
	},
	"views_per_visit": {
		Name:        "views_per_visit",
		Description: "The number of pageviews divided by the number of visits.",
		SQLExpr:     fmt.Sprintf("%s as views_per_visit", viewsPerVisit.ToSql()),
		Valid:       true,
	},
	"events": {
		Name:        "events",
		Description: "Total number of events",
		SQLExpr:     fmt.Sprintf("%s as events", events.ToSql()),
		Valid:       true,
	},
	"bounce_rate": {
		Name:        "bounce_rate",
		Description: "Bounce rate percentage",
		SQLExpr:     fmt.Sprintf("%s as bounce_rate", bounceRate.ToSql()),
		Valid:       true,
	},
	"visit_duration": {
		Name:        "visit_duration",
		Description: "Visit duration in seconds",
		SQLExpr:     fmt.Sprintf("%s as visit_duration", visitDuration.ToSql()),
		Valid:       true,
	},
}

// ValidateMetrics 验证指标是否有效
func ValidateMetrics(metrics []string) error {
	invalid := []string{}

	for _, m := range metrics {
		if _, exists := AvailableMetrics[m]; !exists {
			invalid = append(invalid, m)
		}
	}

	if len(invalid) > 0 {
		return fmt.Errorf("invalid metrics: %s. Available metrics: %s",
			strings.Join(invalid, ", "),
			strings.Join(AvailableMetrics.Names(), ", "))
	}

	return nil
}

// GetMetricSQL 获取指标对应的SQL表达式
func (m Metrics) GetMetricSQL(metric string) (string, error) {
	if metricDef, exists := m[metric]; exists && metricDef.Valid {
		return metricDef.SQLExpr, nil
	}
	return "", fmt.Errorf("metric %s is not available", metric)
}

// Names 返回所有可用指标名称
func (m Metrics) Names() []string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	return names
}

// Descriptions 返回所有指标的描述信息
func (m Metrics) Descriptions() map[string]string {
	descs := make(map[string]string)
	for name, metric := range m {
		descs[name] = metric.Description
	}
	return descs
}
