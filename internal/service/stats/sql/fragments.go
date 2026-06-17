package sql

import (
	"fmt"
	"strings"
)

// SQLFragment 表示带参数的SQL片段
type SQLFragment struct {
	SQL  string
	Args []any
}

func (f *SQLFragment) ToSql() string {
	sql := f.SQL
	sql = strings.ReplaceAll(sql, "?", "%s")
	if len(f.Args) > 0 {
		return fmt.Sprintf(sql, f.Args...)
	}
	return sql
}

// SQLFragmentGenerator 生成统计查询中使用的常见SQL片段
type SQLFragmentGenerator struct{}

// NewFragmentGenerator 创建SQL片段生成器实例
func NewFragmentGenerator() *SQLFragmentGenerator {
	return &SQLFragmentGenerator{}
}

// ScaleSample 对采样数据进行缩放。
// 当采样启用时用 _sample_factor 还原真实值；未启用时直接返回原始表达式。
func (g *SQLFragmentGenerator) ScaleSample(fragment SQLFragment, samplingEnabled bool) SQLFragment {
	if !samplingEnabled {
		return fragment
	}
	sql := fmt.Sprintf("toUInt64(round(%s * any(_sample_factor)))", fragment.SQL)
	return SQLFragment{
		SQL:  sql,
		Args: fragment.Args,
	}
}

// Uniq 计算唯一用户数，带采样缩放
func (g *SQLFragmentGenerator) Uniq(userID string, samplingEnabled bool) SQLFragment {
	return g.ScaleSample(SQLFragment{
		SQL:  "uniq(?)",
		Args: []any{userID},
	}, samplingEnabled)
}

// Total 计算总数，带采样缩放
func (g *SQLFragmentGenerator) Total(samplingEnabled bool) SQLFragment {
	return g.ScaleSample(SQLFragment{
		SQL:  "sum(pageviews * sign)",
		Args: nil,
	}, samplingEnabled)
}

func (g *SQLFragmentGenerator) EventsForEvent(samplingEnabled bool) SQLFragment {
	return g.ScaleSample(SQLFragment{
		SQL:  "countIf(name != 'engagement')",
		Args: nil,
	}, samplingEnabled)
}

// PageViewsForEvent events 表的 pageviews 统计，直接计数 pageview 事件。
func (g *SQLFragmentGenerator) PageViewsForEvent(samplingEnabled bool) SQLFragment {
	return g.ScaleSample(SQLFragment{
		SQL:  "countIf(name = 'pageview')",
		Args: nil,
	}, samplingEnabled)
}

// VisitsForEvent events 表的 visits 统计，按 session 去重。
func (g *SQLFragmentGenerator) VisitsForEvent(samplingEnabled bool) SQLFragment {
	return g.ScaleSample(SQLFragment{
		SQL:  "uniq(session_id)",
		Args: nil,
	}, samplingEnabled)
}

// ScrollDepthForEvent events 表的滚动深度统计，取 engagement 事件的平均滚动深度。
func (g *SQLFragmentGenerator) ScrollDepthForEvent(samplingEnabled bool) SQLFragment {
	return g.ScaleSample(SQLFragment{
		SQL:  "toFloat64(avgIf(scroll_depth, name = 'engagement'))",
		Args: nil,
	}, samplingEnabled)
}

// EventsForSession events总数
func (g *SQLFragmentGenerator) EventsForSession(samplingEnabled bool) SQLFragment {
	return g.ScaleSample(SQLFragment{
		SQL:  "sum(sign * events)",
		Args: nil,
	}, samplingEnabled)
}

// SamplePercent 计算采样百分比。
// 当采样启用时返回实际采样率，否则返回 100。
func (g *SQLFragmentGenerator) SamplePercent(samplingEnabled bool) SQLFragment {
	if !samplingEnabled {
		return SQLFragment{SQL: "100"}
	}
	return SQLFragment{
		SQL:  "if(any(_sample_factor) > 1, round(100 / any(_sample_factor)), 100)",
		Args: nil,
	}
}

// BounceRate 计算跳出率（百分比，保留两位小数）。
func (g *SQLFragmentGenerator) BounceRate() SQLFragment {
	return SQLFragment{
		SQL:  "toFloat64(greatest(ifNotFinite(round(sum(is_bounce * sign) / nullIf(sum(sign), 0) * 100, 2), 0), 0))",
		Args: nil,
	}
}

// ViewsPerVisit 页面浏览量除以访问量。使用 greatest 防止 sign 负值导致的下溢。
func (g *SQLFragmentGenerator) ViewsPerVisit() SQLFragment {
	return SQLFragment{
		SQL:  "greatest(ifNotFinite(round(sum(pageviews * sign) / nullIf(sum(sign), 0), 2), 0), 0)",
		Args: nil,
	}
}

// VisitDuration 计算平均访问时长。使用 greatest 防止 sign 负值导致的下溢。
// VisitDuration 平均会话时长（秒，保留两位小数）。
func (g *SQLFragmentGenerator) VisitDuration() SQLFragment {
	return SQLFragment{
		SQL:  "toFloat64(greatest(ifNotFinite(round(avg(duration * sign)), 0), 0))",
		Args: nil,
	}
}

// CoalesceString 字符串合并，类似SQL的COALESCE
func (g *SQLFragmentGenerator) CoalesceString(fieldA, fieldB string) SQLFragment {
	return SQLFragment{
		SQL:  "if(empty(?), ?, ?)",
		Args: []any{fieldA, fieldB, fieldA},
	}
}

// ToTimezone 转换时间到指定时区
// 对应Elixir中的to_timezone宏
func (g *SQLFragmentGenerator) ToTimezone(date, timezone string) SQLFragment {
	return SQLFragment{
		SQL:  "toTimeZone(?, ?)",
		Args: []any{date, timezone},
	}
}

// WeekstartNotBefore 计算周起始日，不早于指定日期
func (g *SQLFragmentGenerator) WeekstartNotBefore(date, notBefore string) SQLFragment {
	return SQLFragment{
		SQL:  "if(toMonday(?) < toDate(?), toDate(?), toMonday(?))",
		Args: []any{date, notBefore, notBefore, date},
	}
}

// HasKey 检查是否存在指定的元数据键
func (g *SQLFragmentGenerator) HasKey(table, metaColumn, key string) SQLFragment {
	metaKeyColumn := metaKeyColumnName(metaColumn)
	return SQLFragment{
		SQL:  fmt.Sprintf("has(%s.%s, ?)", table, metaKeyColumn),
		Args: []any{key},
	}
}

// GetByKey 获取指定元数据键的值  需要在where中添加 has(metaKeyColumn, key);
func (g *SQLFragmentGenerator) GetByKey(table, metaColumn, key string) SQLFragment {
	metaKeyColumn := metaKeyColumnName(metaColumn)
	metaValueColumn := metaValueColumnName(metaColumn)
	return SQLFragment{
		SQL:  fmt.Sprintf("%s.%s[indexOf(%s.%s, '%s')]", table, metaValueColumn, table, metaKeyColumn, key),
		Args: nil,
	}
}

// TimeOnPage 计算平均页面停留时间
func (g *SQLFragmentGenerator) TimeOnPage(totalTimeOnPage, totalTimeOnPageVisits string) SQLFragment {
	return SQLFragment{
		SQL:  fmt.Sprintf("if(%s > 0, toInt32(round((%s) / (%s))), NULL)", totalTimeOnPageVisits, totalTimeOnPage, totalTimeOnPageVisits),
		Args: nil,
	}
}

// 辅助函数：获取元数据键列名
func metaKeyColumnName(metaColumn string) string {
	switch metaColumn {
	case "meta":
		return "meta.key"
	case "entry_meta":
		return "entry_meta.key"
	default:
		return fmt.Sprintf("%s.key", metaColumn)
	}
}

// 辅助函数：获取元数据值列名
func metaValueColumnName(metaColumn string) string {
	switch metaColumn {
	case "meta":
		return "meta.value"
	case "entry_meta":
		return "entry_meta.value"
	default:
		return fmt.Sprintf("%s.value", metaColumn)
	}
}
