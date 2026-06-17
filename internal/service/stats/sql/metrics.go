package sql

import (
	"fmt"
	"strings"

	"github.com/zenstats/zenstats/internal/service/stats/types"
)

// Metric 定义统计指标元数据

// 所有支持的指标
type Metrics map[string]types.Metric

// 全局指标定义（不含采样表达式，SQLExpr 由 GetMetricSQL 动态生成）
var AvailableMetrics = Metrics{
	"visitors": {
		Name:        "visitors",
		Description: "Unique visitors to the site",
		Valid:       true,
	},
	"pageviews": {
		Name:        "pageviews",
		Description: "Total page views",
		Valid:       true,
	},
	"visits": {
		Name:        "visits",
		Description: "Total number of visits (sessions)",
		Valid:       true,
	},
	"views_per_visit": {
		Name:        "views_per_visit",
		Description: "The number of pageviews divided by the number of visits.",
		Valid:       true,
	},
	"events": {
		Name:        "events",
		Description: "Total number of events",
		Valid:       true,
	},
	"bounce_rate": {
		Name:        "bounce_rate",
		Description: "Bounce rate percentage",
		Valid:       true,
	},
	"visit_duration": {
		Name:        "visit_duration",
		Description: "Visit duration in seconds",
		Valid:       true,
	},
	"scroll_depth": {
		Name:        "scroll_depth",
		Description: "Average scroll depth percentage from engagement events",
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

// GetMetricSQL 根据指标名称和采样配置返回 SQL 表达式。
// samplingEnabled 控制是否使用 _sample_factor 进行采样缩放。
func (m Metrics) GetMetricSQL(metric string, samplingEnabled bool) (string, error) {
	gen := NewFragmentGenerator()
	if _, exists := m[metric]; !exists {
		return "", fmt.Errorf("metric %s is not available", metric)
	}

	switch metric {
	case "visitors":
		frag := gen.Uniq("user_id", samplingEnabled)
		return fmt.Sprintf("%s as visitors", frag.ToSql()), nil
	case "pageviews":
		frag := gen.PageViewsForEvent(samplingEnabled)
		return fmt.Sprintf("%s as pageviews", frag.ToSql()), nil
	case "visits":
		return "sum(sign) as visits", nil
	case "events":
		frag := gen.EventsForEvent(samplingEnabled)
		return fmt.Sprintf("%s as events", frag.ToSql()), nil
	case "bounce_rate":
		frag := gen.BounceRate()
		return fmt.Sprintf("%s as bounce_rate", frag.ToSql()), nil
	case "visit_duration":
		frag := gen.VisitDuration()
		return fmt.Sprintf("%s as visit_duration", frag.ToSql()), nil
	case "views_per_visit":
		frag := gen.ViewsPerVisit()
		return fmt.Sprintf("%s as views_per_visit", frag.ToSql()), nil
	case "scroll_depth":
		frag := gen.ScrollDepthForEvent(samplingEnabled)
		return fmt.Sprintf("%s as scroll_depth", frag.ToSql()), nil
	default:
		return "", fmt.Errorf("metric %s is not available", metric)
	}
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
func (m Metrics) Descriptions() map[string]map[string]string {
	descs := make(map[string]map[string]string)
	for name, metric := range m {
		descs[name] = map[string]string{
			"name":        metric.Name,
			"description": metric.Description,
		}
	}
	return descs
}
