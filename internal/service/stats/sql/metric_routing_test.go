package sql

import (
	"strings"
	"testing"

	"github.com/zenstats/zenstats/internal/service/stats/types"
)

// ============================================================
// metricCategory 路由测试 — 确保每个指标去正确的表
// ============================================================

func TestMetricCategoryRoutesAllMetricsCorrectly(t *testing.T) {
	qo := &QueryOptimizer{}

	// 无 time 维度的默认 query
	query := &types.Query{
		Dimensions: []string{},
	}

	tests := []struct {
		metric   string
		want     string // "event", "session", "either"
	}{
		// event 指标
		{"pageviews", "event"},
		{"events", "event"},
		{"time_on_page", "event"},
		{"scroll_depth", "event"},
		// session 指标
		{"bounce_rate", "session"},
		{"visit_duration", "session"},
		{"views_per_visit", "session"},
		// either 指标
		{"visitors", "either"},
		{"visits", "either"},
		// either/computed
		{"conversion_rate", "either"},
		{"group_conversion_rate", "either"},
		{"percentage", "either"},
	}

	for _, tt := range tests {
		t.Run(tt.metric, func(t *testing.T) {
			got := qo.metricCategory(tt.metric, query)
			if got != tt.want {
				t.Errorf("metricCategory(%q) = %q, want %q", tt.metric, got, tt.want)
			}
		})
	}
}

func TestMetricCategoryVisitorsVisitsWithMinuteDimension(t *testing.T) {
	qo := &QueryOptimizer{}

	query := &types.Query{
		Dimensions: []string{"time:minute"},
	}

	if got := qo.metricCategory("visitors", query); got != "session" {
		t.Errorf("visitors + time:minute should route to session, got %q", got)
	}
	if got := qo.metricCategory("visits", query); got != "session" {
		t.Errorf("visits + time:minute should route to session, got %q", got)
	}
}

// ============================================================
// GetMetricSQL 测试 — 确保每个指标的 SQL 正确
// ============================================================

func TestGetMetricSQLEventTable(t *testing.T) {
	tests := []struct {
		metric   string
		contains string // SQL 必须包含的关键词
		absent   string // SQL 不能包含的关键词
	}{
		{"visitors", "uniq(user_id)", "sign"},
		{"visits", "countIf(name = 'pageview')", ""},  // event 表 visits 在 selectEventMetrics 特殊处理
		{"pageviews", "countIf(name = 'pageview')", "sign"},
		{"events", "countIf(name != 'engagement')", "sign"},
		{"scroll_depth", "avgIf(scroll_depth", ""},
	}

	for _, tt := range tests {
		t.Run(tt.metric+"_event", func(t *testing.T) {
			sql, err := AvailableMetrics.GetMetricSQL(tt.metric, false)
			// visits 在 GetMetricSQL 中返回 session 版本（event 版本在 selectEventMetrics 特殊处理）
			if tt.metric == "visits" {
				if !strings.Contains(sql, "sum(sign)") {
					t.Errorf("GetMetricSQL(visits) session default should contain sum(sign): %q", sql)
				}
				return
			}
			if err != nil {
				// scroll_depth/time_on_page 等无默认 SQL 是预期的
				if tt.metric == "scroll_depth" || tt.metric == "time_on_page" {
					return
				}
				t.Errorf("GetMetricSQL(%q) returned error: %v", tt.metric, err)
				return
			}
			if tt.contains != "" && !strings.Contains(sql, tt.contains) {
				t.Errorf("GetMetricSQL(%q) = %q, missing %q", tt.metric, sql, tt.contains)
			}
			if tt.absent != "" && strings.Contains(sql, tt.absent) {
				t.Errorf("GetMetricSQL(%q) = %q, must not contain %q", tt.metric, sql, tt.absent)
			}
		})
	}
}

func TestGetMetricSQLSessionTable(t *testing.T) {
	qb := NewQueryBuilder()

	tests := []struct {
		metric   string
		contains string
		absent   string
	}{
		{"bounce_rate", "sum(is_bounce * sign)", ""},
		{"visit_duration", "avg(duration * sign)", ""},
		{"views_per_visit", "sum(pageviews * sign)", ""},
		{"visits", "sum(sign)", ""},
		{"visitors", "uniq(user_id)", "sign"},
	}

	for _, tt := range tests {
		t.Run(tt.metric+"_session", func(t *testing.T) {
			sql, err := qb.getMetricSQLForTable(tt.metric, "sessions", false)
			if err != nil {
				t.Errorf("getMetricSQLForTable(%q, sessions) returned error: %v", tt.metric, err)
				return
			}
			if tt.contains != "" && !strings.Contains(sql, tt.contains) {
				t.Errorf("getMetricSQLForTable(%q, sessions) = %q, missing %q", tt.metric, sql, tt.contains)
			}
			if tt.absent != "" && strings.Contains(sql, tt.absent) {
				t.Errorf("getMetricSQLForTable(%q, sessions) = %q, must not contain %q", tt.metric, sql, tt.absent)
			}
		})
	}
}

func TestSelectEventMetricsVisitsUsesSessionId(t *testing.T) {
	qb := NewQueryBuilder()
	query := &types.Query{Metrics: []string{"visits"}}

	got := qb.selectEventMetrics(query)
	if !strings.Contains(got, "uniq(session_id) as visits") {
		t.Errorf("selectEventMetrics(visits) should use uniq(session_id): %q", got)
	}
}

func TestSelectEventMetricsAllSupportedMetrics(t *testing.T) {
	qb := NewQueryBuilder()

	// 所有 event 表支持的指标组合
	query := &types.Query{
		Metrics: []string{"visitors", "visits", "pageviews", "events"},
	}

	got := qb.selectEventMetrics(query)

	checks := []string{
		"uniq(user_id) as visitors",
		"uniq(session_id) as visits",
		"countIf(name = 'pageview') as pageviews",
		"countIf(name != 'engagement') as events",
	}

	for _, c := range checks {
		if !strings.Contains(got, c) {
			t.Errorf("selectEventMetrics missing %q in:\n%s", c, got)
		}
	}

	// 不能包含 sessions 表专属列
	forbidden := []string{"sign", "is_bounce", "duration", "pageviews * sign"}
	for _, f := range forbidden {
		if strings.Contains(got, f) {
			t.Errorf("selectEventMetrics must not contain session column %q: %q", f, got)
		}
	}
}

// ============================================================
// partitionMetrics 集成测试 — 确保指标正确分流
// ============================================================

func TestPartitionMetricsAggregateQuery(t *testing.T) {
	qo := &QueryOptimizer{}

	// 模拟 aggregate 默认指标: visitors, pageviews, bounce_rate, visit_duration, views_per_visit
	// replaceViewsPerVisit 会将 views_per_visit→visits, 并确保 pageviews 存在
	metrics := []string{"visitors", "visits", "pageviews", "bounce_rate", "visit_duration"}
	query := &types.Query{
		Metrics:    metrics,
		Dimensions: []string{},
	}

	splits := qo.Split(query)

	// 应该有 event 和 session 两个查询
	if len(splits) < 2 {
		t.Fatalf("expected 2 splits for aggregate, got %d", len(splits))
	}

	hasEvent := false
	hasSession := false
	for _, s := range splits {
		switch s.TableType {
		case TableEvents:
			hasEvent = true
			// event 查询应包含: visitors, visits, pageviews
			for _, want := range []string{"visitors", "visits", "pageviews"} {
				if !hasMetric(s.Query.Metrics, want) {
					t.Errorf("event query missing metric %q: %v", want, s.Query.Metrics)
				}
			}
			// 不应包含 session 指标
			for _, bad := range []string{"bounce_rate", "visit_duration"} {
				if hasMetric(s.Query.Metrics, bad) {
					t.Errorf("event query should not contain %q: %v", bad, s.Query.Metrics)
				}
			}
		case TableSessions, TableSessionsSmeared:
			hasSession = true
			for _, want := range []string{"bounce_rate", "visit_duration"} {
				if !hasMetric(s.Query.Metrics, want) {
					t.Errorf("session query missing metric %q: %v", want, s.Query.Metrics)
				}
			}
		}
	}

	if !hasEvent || !hasSession {
		t.Errorf("missing queries: event=%v session=%v", hasEvent, hasSession)
	}
}

func TestPartitionMetricsEventOnly(t *testing.T) {
	qo := &QueryOptimizer{}

	// 只有 event 指标 + event 维度
	metrics := []string{"visitors", "pageviews", "events"}
	query := &types.Query{
		Metrics:    metrics,
		Dimensions: []string{"event:page"},
	}

	splits := qo.Split(query)

	// event:page 会触发 SessionsJoinEvents，可能产生额外的 session 拆分
	// 这是预期行为——支持 session smearing 和 event↔session 维度关联
	hasEvent := false
	for _, s := range splits {
		if s.TableType == TableEvents {
			hasEvent = true
			for _, want := range []string{"visitors", "pageviews", "events"} {
				if !hasMetric(s.Query.Metrics, want) {
					t.Errorf("event query missing metric %q: %v", want, s.Query.Metrics)
				}
			}
		}
	}
	if !hasEvent {
		t.Error("event-only query must have an event split")
	}
}

func TestPartitionMetricsSessionOnly(t *testing.T) {
	qo := &QueryOptimizer{}

	metrics := []string{"bounce_rate", "visit_duration"}
	query := &types.Query{
		Metrics:    metrics,
		Dimensions: []string{},
	}

	splits := qo.Split(query)

	hasEvent := false
	for _, s := range splits {
		if s.TableType == TableEvents {
			hasEvent = true
		}
	}
	if hasEvent {
		t.Error("session-only query should not create an event split")
	}
}

// ============================================================
// derivedNameFilter 测试 — 确保 engagement 过滤正确
// ============================================================

func TestDerivedNameFilterDefaultExcludesEngagement(t *testing.T) {
	qb := NewQueryBuilder()
	query := &types.Query{
		Metrics:    []string{"visitors", "pageviews"},
		Dimensions: []string{"event:page"},
	}

	got := qb.derivedNameFilter(query)
	if got != " AND name != 'engagement'" {
		t.Errorf("derivedNameFilter should exclude engagement by default: %q", got)
	}
}

func TestDerivedNameFilterAllowsEngagementForScrollDepth(t *testing.T) {
	qb := NewQueryBuilder()
	query := &types.Query{
		Metrics:    []string{"scroll_depth"},
		Dimensions: []string{"event:page"},
	}

	got := qb.derivedNameFilter(query)
	if got != "" {
		t.Errorf("derivedNameFilter should NOT filter when scroll_depth requested: %q", got)
	}
}

func TestDerivedNameFilterAllowsEngagementForTimeOnPage(t *testing.T) {
	qb := NewQueryBuilder()
	query := &types.Query{
		Metrics:    []string{"visitors", "time_on_page"},
		Dimensions: []string{"event:page"},
	}

	got := qb.derivedNameFilter(query)
	if got != "" {
		t.Errorf("derivedNameFilter should NOT filter when time_on_page requested: %q", got)
	}
}

func TestDerivedNameFilterSkipsWhenGoalFilterPresent(t *testing.T) {
	qb := NewQueryBuilder()
	query := &types.Query{
		Metrics: []string{"visitors", "pageviews"},
		Filters: []*types.Filter{
			{Operator: "is", Dimension: "event:goal", Values: []any{"Signup"}},
		},
	}

	got := qb.derivedNameFilter(query)
	if got != "" {
		t.Errorf("derivedNameFilter should skip when goal filter present: %q", got)
	}
}

// ============================================================
// selectEventMetrics + selectSessionMetrics 完整性测试
// ============================================================

func TestSelectSessionMetricsAllSupportedMetrics(t *testing.T) {
	qb := NewQueryBuilder()

	query := &types.Query{
		Metrics: []string{"visitors", "visits", "bounce_rate", "visit_duration", "views_per_visit"},
	}

	got := qb.selectSessionMetrics(query)

	checks := []string{
		"uniq(user_id) as visitors",
		"sum(sign) as visits",
		"sum(is_bounce * sign)",
		"avg(duration * sign)",
		"sum(pageviews * sign)",
	}

	for _, c := range checks {
		if !strings.Contains(got, c) {
			t.Errorf("selectSessionMetrics missing %q in:\n%s", c, got)
		}
	}
}

func TestSelectEventMetricsDoesNotLeakSessionColumns(t *testing.T) {
	qb := NewQueryBuilder()

	allEventMetrics := []string{"visitors", "visits", "pageviews", "events"}
	query := &types.Query{Metrics: allEventMetrics}

	got := qb.selectEventMetrics(query)

	// 确认不包含 sessions 表专有列
	sessionOnlyCols := []string{"sign", "is_bounce", "pageviews * sign"}
	for _, col := range sessionOnlyCols {
		if strings.Contains(got, col) {
			t.Errorf("selectEventMetrics leaked session column %q: %q", col, got)
		}
	}
}

// ============================================================
// ValidateConflicts 测试 — 确保不会误拦合法组合
// ============================================================

func TestValidateConflictsAllowsEventMetricsWithVisitCountry(t *testing.T) {
	td := &TableDecider{}
	query := &types.Query{
		Metrics:    []string{"visitors", "events", "pageviews"},
		Dimensions: []string{"visit:country"},
	}

	err := td.ValidateConflicts(query)
	if err != nil {
		t.Errorf("events+pageviews with visit:country should be allowed: %v", err)
	}
}

func TestValidateConflictsBlocksSessionMetricsWithEventName(t *testing.T) {
	td := &TableDecider{}
	query := &types.Query{
		Metrics:    []string{"bounce_rate", "visit_duration"},
		Dimensions: []string{"event:name"},
	}

	err := td.ValidateConflicts(query)
	if err == nil {
		t.Error("session metrics with event:name should be blocked")
	}
}

func TestValidateConflictsAllowsSessionMetricsWithVisitEntryPage(t *testing.T) {
	td := &TableDecider{}
	query := &types.Query{
		Metrics:    []string{"bounce_rate", "visit_duration"},
		Dimensions: []string{"visit:entry_page"},
	}

	err := td.ValidateConflicts(query)
	if err != nil {
		t.Errorf("session metrics with visit:entry_page should be allowed: %v", err)
	}
}

func TestValidateConflictsEventPageIsSpecialCase(t *testing.T) {
	td := &TableDecider{}
	query := &types.Query{
		Metrics:    []string{"bounce_rate", "visit_duration"},
		Dimensions: []string{"event:page"},
	}

	err := td.ValidateConflicts(query)
	if err != nil {
		t.Errorf("session metrics with event:page should be allowed (special case): %v", err)
	}
}

// ============================================================
// isEventDimension / isEventMetric 一致性测试
// ============================================================

func TestIsEventMetricAndMetricCategoryAlignment(t *testing.T) {
	td := &TableDecider{}
	qo := &QueryOptimizer{}
	query := &types.Query{Dimensions: []string{}}

	// 所有指标在两个分类系统中的一致性
	allMetrics := []string{
		"visitors", "visits", "pageviews", "events",
		"bounce_rate", "visit_duration", "views_per_visit",
		"time_on_page", "scroll_depth",
	}

	for _, metric := range allMetrics {
		t.Run(metric, func(t *testing.T) {
			isEvent := td.isEventMetric(query, metric)
			cat := qo.metricCategory(metric, query)

			// event 指标在 metricCategory 中应为 "event"
			// session 指标在 metricCategory 中应为 "session"
			// either 指标两者都可能
			if isEvent && cat == "session" {
				t.Errorf("MISALIGNMENT: isEventMetric(%q)=true but metricCategory=%q", metric, cat)
			}
		})
	}
}

func TestIsEventDimensionClassification(t *testing.T) {
	td := &TableDecider{}

	tests := []struct {
		dimension string
		isEvent   bool
	}{
		{"event:name", true},
		{"event:page", true},
		{"event:hostname", true},
		{"event:country", true},
		{"visit:source", true},
		{"visit:country", true},
		{"visit:browser", true},
		{"visit:os", true},
		{"visit:device", true},
		{"visit:entry_page", false},
		{"visit:entry_page_hostname", false},
		{"visit:exit_page", false},
		{"visit:exit_page_hostname", false},
	}

	for _, tt := range tests {
		t.Run(tt.dimension, func(t *testing.T) {
			got := td.isEventDimension(tt.dimension)
			if got != tt.isEvent {
				t.Errorf("isEventDimension(%q) = %v, want %v", tt.dimension, got, tt.isEvent)
			}
		})
	}
}

// ============================================================
// BounceRate / VisitDuration / ViewsPerVisit 防护测试
// ============================================================

func TestBounceRateHasGreatestProtection(t *testing.T) {
	frag := NewFragmentGenerator().BounceRate()
	got := frag.ToSql()
	if !strings.Contains(got, "greatest(") {
		t.Errorf("BounceRate missing greatest protection: %q", got)
	}
	if !strings.Contains(got, "nullIf(sum(sign)") {
		t.Errorf("BounceRate missing nullIf division protection: %q", got)
	}
	if !strings.Contains(got, "ifNotFinite") {
		t.Errorf("BounceRate missing ifNotFinite: %q", got)
	}
}

func TestVisitDurationHasGreatestProtection(t *testing.T) {
	frag := NewFragmentGenerator().VisitDuration()
	got := frag.ToSql()
	if !strings.Contains(got, "greatest(") {
		t.Errorf("VisitDuration missing greatest protection: %q", got)
	}
}

func TestViewsPerVisitHasGreatestProtection(t *testing.T) {
	frag := NewFragmentGenerator().ViewsPerVisit()
	got := frag.ToSql()
	if !strings.Contains(got, "greatest(") {
		t.Errorf("ViewsPerVisit missing greatest protection: %q", got)
	}
}

// ============================================================
// 辅助函数
// ============================================================

func hasMetric(metrics []string, target string) bool {
	for _, m := range metrics {
		if m == target {
			return true
		}
	}
	return false
}
