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

// ScaleSample 对采样数据进行缩放
func (g *SQLFragmentGenerator) ScaleSample(fragment SQLFragment) SQLFragment {
	sql := fmt.Sprintf("toUInt64(round(%s * any(_sample_factor)))", fragment.SQL)
	return SQLFragment{
		SQL:  sql,
		Args: fragment.Args,
	}
}

// Uniq 计算唯一用户数，带采样缩放
func (g *SQLFragmentGenerator) Uniq(userID string) SQLFragment {
	return g.ScaleSample(SQLFragment{
		SQL:  "uniq(?)",
		Args: []any{userID},
	})
}

// Total 计算总数，带采样缩放
func (g *SQLFragmentGenerator) Total() SQLFragment {
	return g.ScaleSample(SQLFragment{
		SQL:  "sum(pageviews * sign)",
		Args: nil,
	})
}

func (g *SQLFragmentGenerator) EventsForEvent() SQLFragment {
	return g.ScaleSample(SQLFragment{
		SQL:  "countIf(name != 'engagement')",
		Args: nil,
	})
}

// EventsForSession events总数
func (g *SQLFragmentGenerator) EventsForSession() SQLFragment {
	return g.ScaleSample(SQLFragment{
		SQL:  "sum(sign * events)",
		Args: nil,
	})
}

// SamplePercent 计算采样百分比
func (g *SQLFragmentGenerator) SamplePercent() SQLFragment {
	return SQLFragment{
		SQL:  "if(any(_sample_factor) > 1, round(100 / any(_sample_factor)), 100)",
		Args: nil,
	}
}

// BounceRate 计算跳出率
func (g *SQLFragmentGenerator) BounceRate() SQLFragment {
	return SQLFragment{
		SQL:  "toFloat64(ifNotFinite(round(sum(is_bounce * sign) / sum(sign) * 100, 2), 0))",
		Args: nil,
	}
}

// ViewsPerVisit 页面浏览量除以访问量。
func (g *SQLFragmentGenerator) ViewsPerVisit() SQLFragment {
	return SQLFragment{
		SQL:  "ifNotFinite(round(sum(pageviews * sign) / sum(sign), 2), 0)",
		Args: nil,
	}
}

// VisitDuration 计算平均访问时长
func (g *SQLFragmentGenerator) VisitDuration() SQLFragment {
	return SQLFragment{
		SQL:  "toUInt32(ifNotFinite(round(avg(duration * sign)), 0))",
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
