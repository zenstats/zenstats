package sql

import (
	"strings"
	"testing"

	"github.com/zenstats/zenstats/internal/service/stats/types"
)

// TestCustomPropNameEscapesSQLInjection 验证自定义属性名中的单引号被转义，
// 无法越出 ClickHouse 字符串字面量注入 SQL。
func TestCustomPropNameEscapesSQLInjection(t *testing.T) {
	wb := NewWhereBuilder("1")
	err := wb.AddFilter("events", &types.Filter{
		Operator:  "is",
		Dimension: "event:props:x')]=1 OR '1'='1",
		Values:    []any{"v"},
	})
	if err != nil {
		t.Fatalf("AddFilter returned error: %v", err)
	}
	where, _ := wb.Build()
	// 注入用的裸单引号必须被转义为 \'，不能出现未转义的 ')]=1 OR '1'='1 片段
	if strings.Contains(where, "')]=1 OR '1'='1") {
		t.Fatalf("prop name not escaped, injection possible: %q", where)
	}
	if !strings.Contains(where, `\'`) {
		t.Fatalf("expected escaped quote in output, got %q", where)
	}
}

// TestOrderByRejectsInjection 验证 order_by 只接受本次查询声明的字段与 asc/desc 方向。
func TestOrderByRejectsInjection(t *testing.T) {
	qb := NewQueryBuilder()
	query := &types.Query{
		Metrics:    []string{"visitors", "pageviews"},
		Dimensions: []string{"visit:browser"},
		OrderBy: []*types.OrderBy{
			{Dimension: "visitors", Direction: "asc)) UNION SELECT 1 --"}, // 恶意方向 → 应回退为 desc
			{Dimension: "evil_col", Direction: "desc"},                    // 未声明字段 → 应丢弃
			{Dimension: "pageviews", Direction: "asc"},                    // 合法 → 应保留 asc
		},
	}
	out := qb.buildOrderBy("SELECT 1", query)
	if strings.Contains(out, "UNION") || strings.Contains(out, "--") {
		t.Fatalf("order_by injection leaked into SQL: %q", out)
	}
	if strings.Contains(out, "evil_col") {
		t.Fatalf("undeclared order-by field accepted: %q", out)
	}
	// 恶意方向被规范化为 desc，合法字段 pageviews 的 asc 被保留
	if !strings.Contains(out, "visitors desc") {
		t.Fatalf("malicious direction not normalized to desc: %q", out)
	}
	if !strings.Contains(out, "pageviews asc") {
		t.Fatalf("valid asc direction not applied: %q", out)
	}
}

func TestDimensionToColumnUsesExistingDeviceAlias(t *testing.T) {
	qb := NewQueryBuilder()

	tests := []struct {
		name      string
		dimension string
		want      string
	}{
		{name: "event device", dimension: "event:device", want: "device"},
		{name: "visit device", dimension: "visit:device", want: "device"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := qb.DimensionToColumn(tt.dimension, "sessions", "select", 0); got != tt.want {
				t.Fatalf("DimensionToColumn(%q) = %q, want %q", tt.dimension, got, tt.want)
			}
			if got := qb.DimensionToColumn(tt.dimension, "sessions", "group", 0); got != tt.want {
				t.Fatalf("DimensionToColumn(%q, group) = %q, want %q", tt.dimension, got, tt.want)
			}
		})
	}
}

func TestDimensionToColumnUsesExistingRegionAlias(t *testing.T) {
	qb := NewQueryBuilder()

	tests := []struct {
		name      string
		dimension string
	}{
		{name: "event region", dimension: "event:region"},
		{name: "visit region", dimension: "visit:region"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := qb.DimensionToColumn(tt.dimension, "sessions", "select", 0); got != "continent_name as region" {
				t.Fatalf("DimensionToColumn(%q) = %q, want %q", tt.dimension, got, "continent_name as region")
			}
			if got := qb.DimensionToColumn(tt.dimension, "sessions", "group", 0); got != "continent_name" {
				t.Fatalf("DimensionToColumn(%q, group) = %q, want %q", tt.dimension, got, "continent_name")
			}
		})
	}
}

func TestSelectSessionMetricsUsesSessionEventsColumn(t *testing.T) {
	qb := NewQueryBuilder()
	query := &types.Query{Metrics: []string{"events"}}

	got := qb.selectSessionMetrics(query)
	if got != "sum(sign * events) as events" {
		t.Fatalf("selectSessionMetrics(events) = %q", got)
	}
	if strings.Contains(got, "name") {
		t.Fatalf("session events metric must not reference events.name: %q", got)
	}
}

func TestSelectEventMetricsAliasesPageviewsAsRequestedMetric(t *testing.T) {
	qb := NewQueryBuilder()
	query := &types.Query{Metrics: []string{"pageviews"}}

	got := qb.selectEventMetrics(query)
	if !strings.Contains(got, "countIf(name = 'pageview') as pageviews") {
		t.Fatalf("event pageviews metric must use countIf: %q", got)
	}
	if strings.Contains(got, "cur_pageviews") {
		t.Fatalf("event pageviews metric must not expose stale cur_pageviews alias: %q", got)
	}
}

func TestEventPageBreakdownMixedMetricsSelectsExistingSessionAliases(t *testing.T) {
	qb := NewQueryBuilder()
	query := &types.Query{
		Metrics:    []string{"visitors", "events", "pageviews", "bounce_rate"},
		Dimensions: []string{"event:page"},
	}
	qo := &QueryOptimizer{}
	optimized := qo.Optimize(query)
	splits := qo.Split(optimized)
	var eventQuery, sessionQuery *types.Query
	for _, split := range splits {
		switch split.TableType {
		case TableEvents:
			eventQuery = split.Query
		case TableSessions, TableSessionsSmeared:
			sessionQuery = split.Query
		}
	}

	selectClause := qb.buildJoinSelectClause((&TableDecider{}).EventFields(eventQuery), (&TableDecider{}).SessionFields(sessionQuery))
	eventSelect := qb.selectEventMetrics(eventQuery)

	// pageviews 和 events 现在是 event 指标，应在 event 子查询中
	if !strings.Contains(eventSelect, "countIf(name = 'pageview') as pageviews") {
		t.Fatalf("event subquery should expose pageviews alias with countIf: %q", eventSelect)
	}
	if !strings.Contains(eventSelect, "countIf(name != 'engagement') as events") {
		t.Fatalf("event subquery should expose events alias with countIf: %q", eventSelect)
	}
	if !strings.Contains(selectClause, "e.pageviews") {
		t.Fatalf("joined select should reference e.pageviews: %q", selectClause)
	}
	if strings.Contains(eventSelect, "cur_pageviews") || strings.Contains(selectClause, "cur_pageviews") {
		t.Fatalf("joined breakdown query must not use cur_pageviews alias; select=%q event=%q", selectClause, eventSelect)
	}
}

func TestAggregateSessionMetricsDoNotNestAggregateFunctions(t *testing.T) {
	qb := NewQueryBuilder()
	query := &types.Query{Metrics: []string{"events", "visits"}}

	got := qb.selectSessionMetrics(query)
	if strings.Contains(got, "sum(toUInt64") || strings.Contains(got, "sum(round(sum(") {
		t.Fatalf("session aggregate metrics must not nest aggregate functions: %q", got)
	}
	if !strings.Contains(got, "sum(sign * events)") {
		t.Fatalf("expected events to use raw session aggregation: %q", got)
	}
	if !strings.Contains(got, "sum(sign)") {
		t.Fatalf("expected visits to use sum(sign): %q", got)
	}
}

func TestTimeDimensionsUseStableAliasesForSelectAndGroupBy(t *testing.T) {
	qb := NewQueryBuilder()

	tests := []struct {
		dimension  string
		alias      string
		selectPart string
	}{
		{dimension: "time:hour", alias: "hour", selectPart: "toStartOfHour(timestamp)"},
		{dimension: "time:day", alias: "day", selectPart: "toStartOfDay(timestamp)"},
		{dimension: "time:week", alias: "week", selectPart: "toStartOfWeek(timestamp)"},
		{dimension: "time:month", alias: "month", selectPart: "toStartOfMonth(timestamp)"},
	}

	for _, tt := range tests {
		t.Run(tt.dimension, func(t *testing.T) {
			selectColumn := qb.DimensionToColumn(tt.dimension, "events", "select", 0)
			if !strings.Contains(selectColumn, tt.selectPart) || !strings.Contains(selectColumn, " as "+tt.alias) {
				t.Fatalf("select column for %s = %q", tt.dimension, selectColumn)
			}
			if groupColumn := qb.DimensionToColumn(tt.dimension, "events", "group", 0); groupColumn != tt.alias {
				t.Fatalf("group column for %s = %q, want %q", tt.dimension, groupColumn, tt.alias)
			}
		})
	}
}

func TestEventPageBreakdownAliasesPageForJoin(t *testing.T) {
	qb := NewQueryBuilder()

	eventQuery := &types.Query{
		Metrics:    []string{"visitors"},
		Dimensions: []string{"event:page"},
	}
	sessionQuery := &types.Query{
		Metrics:    []string{"pageviews", "bounce_rate"},
		Dimensions: []string{"visit:entry_page"},
	}

	eventSelect := qb.selectEventMetrics(eventQuery)
	sessionSelect := qb.selectSessionMetrics(sessionQuery)
	joinCondition := qb.buildGroupByJoin(eventQuery)

	if !strings.Contains(eventSelect, "pathname as page") {
		t.Fatalf("event page select should alias pathname as page: %q", eventSelect)
	}
	if !strings.Contains(sessionSelect, "entry_page as page") {
		t.Fatalf("session entry_page select should alias entry_page as page: %q", sessionSelect)
	}
	if joinCondition != "e.page = s.page" {
		t.Fatalf("join condition = %q, want %q", joinCondition, "e.page = s.page")
	}
}

func TestWhereBuilderMatchesAcceptsJSONDecodedValues(t *testing.T) {
	wb := NewWhereBuilder("1")
	err := wb.AddFilter("events", &types.Filter{
		Operator:  "matches",
		Dimension: "event:page",
		Values:    []any{"^/download"},
	})
	if err != nil {
		t.Fatalf("AddFilter returned error: %v", err)
	}
	where, params := wb.Build()
	if !strings.Contains(where, "multiMatchAny(pathname, ?)") {
		t.Fatalf("where = %q", where)
	}
	// multiMatchAny 的第二个参数是模式数组，参数应以 []string 绑定
	patterns, ok := params[0].([]string)
	if len(params) != 1 || !ok || len(patterns) != 1 || patterns[0] != "^/download" {
		t.Fatalf("params = %#v", params)
	}
}

func TestWhereBuilderGoalFilterBuildsValidCondition(t *testing.T) {
	wb := NewWhereBuilder("1")
	err := wb.AddFilter("events", &types.Filter{
		Operator:  "is",
		Dimension: "event:goal",
		Values:    []any{"Signup"},
	})
	if err != nil {
		t.Fatalf("AddFilter returned error: %v", err)
	}
	where, params := wb.Build()
	// Without a real DB connection, goal resolution falls back to 1=0
	if !strings.Contains(where, "1 = 0") {
		t.Fatalf("where = %q (expected 1=0 fallback without DB)", where)
	}
	if len(params) != 0 {
		t.Fatalf("params = %#v", params)
	}
}

// TestBuildOrderByNotFooledByWindowFunction 验证窗口函数内的 ORDER BY 不会让外层排序被误用逗号拼接。
func TestBuildOrderByNotFooledByWindowFunction(t *testing.T) {
	qb := NewQueryBuilder()
	q := &types.Query{
		Metrics: []string{"visitors"},
		OrderBy: []*types.OrderBy{{Dimension: "visitors", Direction: "desc"}},
	}
	in := "SELECT a, LEAD(x) OVER (PARTITION BY s ORDER BY t) AS n FROM tbl"
	out := qb.buildOrderBy(in, q)
	if !strings.Contains(out, " ORDER BY visitors desc") {
		t.Fatalf("expected a proper ORDER BY clause, got %q", out)
	}
	if strings.Contains(out, ", visitors desc") {
		t.Fatalf("outer order-by was appended with a comma (invalid): %q", out)
	}
}
