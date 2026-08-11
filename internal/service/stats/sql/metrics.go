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
	"conversions": {
		Name:        "conversions",
		Description: "Unique visitors who completed the goal (numerator of conversion_rate)",
		Valid:       true,
	},
	"percentage": {
		Name:        "percentage",
		Description: "Each group's visitors as a percentage of total visitors (breakdown only, computed in Go)",
		Valid:       true,
	},
	"total_revenue": {
		Name:        "total_revenue",
		Description: "Sum of event revenue (ecommerce)",
		Valid:       true,
	},
	"average_revenue": {
		Name:        "average_revenue",
		Description: "Average revenue per revenue-bearing event (ecommerce)",
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
	"time_on_page": {
		Name:        "time_on_page",
		Description: "Average time spent on page in seconds (from engagement events)",
		Valid:       true,
	},
	"exit_rate": {
		Name:        "exit_rate",
		Description: "Percentage of sessions that exited on this page",
		Valid:       true,
	},
	"conversion_rate": {
		Name:        "conversion_rate",
		Description: "Percentage of visitors who completed a goal",
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
		// visits 是会话计数，采样开启时需按 _sample_factor 还原真实值，
		// 否则 visits 被低估、且 Go 侧 views_per_visit=pageviews/visits 会被放大。
		frag := gen.ScaleSample(SQLFragment{SQL: "sum(sign)"}, samplingEnabled)
		return fmt.Sprintf("%s as visits", frag.ToSql()), nil
	case "total_revenue":
		// 收入总额：NULL(非收入事件) 记为 0；采样开启时按 _sample_factor 放大后取两位小数。
		expr := "sum(ifNull(revenue, 0))"
		if samplingEnabled {
			expr = fmt.Sprintf("%s * any(_sample_factor)", expr)
		}
		return fmt.Sprintf("toFloat64(round(%s, 2)) as total_revenue", expr), nil
	case "average_revenue":
		// 每笔收入事件的平均收入：avg 自动忽略 NULL；无收入事件时 ifNotFinite 兜底为 0。
		return "toFloat64(ifNotFinite(round(avg(revenue), 2), 0)) as average_revenue", nil
	case "events":
		frag := gen.EventsForEvent(samplingEnabled)
		return fmt.Sprintf("%s as events", frag.ToSql()), nil
	case "conversions":
		// 转化人数 = 命中目标的独立访客（查询在 goal 过滤下执行）。作为 conversion_rate 的分子，
		// 分母（基线访客）由服务层单独查询后在 Go 侧计算比值。
		frag := gen.Uniq("user_id", samplingEnabled)
		return fmt.Sprintf("%s as conversions", frag.ToSql()), nil
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
	case "time_on_page":
		frag := gen.TimeOnPageForEvent(samplingEnabled)
		return fmt.Sprintf("%s as time_on_page", frag.ToSql()), nil
	case "exit_rate":
		frag := gen.ExitRateForSession()
		return fmt.Sprintf("%s as exit_rate", frag.ToSql()), nil
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
