package sql

import (
	"strings"
	"testing"

	"github.com/zenstats/zenstats/internal/service/stats/types"
)

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
	if len(params) != 1 || params[0] != "^/download" {
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
	if !strings.Contains(where, "goal_id IN (SELECT id FROM goals WHERE site_id = ? AND name = ?)") {
		t.Fatalf("where = %q", where)
	}
	if len(params) != 2 || params[0] != "1" || params[1] != "Signup" {
		t.Fatalf("params = %#v", params)
	}
}
