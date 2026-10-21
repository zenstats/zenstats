package sql

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/zenstats/zenstats/internal/service/stats/types"
)

func TestFilterOperatorRegressions(t *testing.T) {
	for _, op := range []string{"is_not", "contains", "contains_not", "matches", "matches_not", "matches_wildcard", "matches_wildcard_not"} {
		t.Run("event_name_"+op, func(t *testing.T) {
			wb := NewWhereBuilder("1")
			if err := wb.AddFilter("events", &types.Filter{Operator: op, Dimension: "event:name", Values: []any{"signup"}}); err != nil {
				t.Fatal(err)
			}
			condition, _ := wb.Build()
			if condition == "name = ?" {
				t.Fatalf("%s was reduced to equality", op)
			}
			if strings.Contains(op, "not") && !strings.Contains(condition, "NOT") {
				t.Fatalf("exclusion lost: %s", condition)
			}
		})
	}
	for _, tc := range []struct {
		pattern, value string
		match          bool
	}{
		{"/docs/*", "/docs/intro", true}, {"*/docs", "/a/docs", true}, {"*docs*", "/a/docs/b", true},
		{"*", "/anything", true}, {"/file?.txt", "/file1.txt", true}, {"/docs/*", "/other", false},
		{"/a.b", "/axb", false},
	} {
		if got := regexp.MustCompile(wildcardToRegex(tc.pattern)).MatchString(tc.value); got != tc.match {
			t.Errorf("pattern=%q value=%q match=%v", tc.pattern, tc.value, got)
		}
	}
	wb := NewWhereBuilder("1")
	if err := wb.AddFilter("events", &types.Filter{Operator: "matches_wildcard", Dimension: "event:page", Values: []any{"/DOCS/*"}, Modifiers: map[string]string{"case_sensitive": "false"}}); err != nil {
		t.Fatal(err)
	}
	_, args := wb.Build()
	if !regexp.MustCompile(args[0].([]string)[0]).MatchString("/docs/intro") {
		t.Fatal("case-insensitive wildcard did not normalize pattern")
	}
}

func TestCrossTableFiltersKeepScopeAndBooleanTree(t *testing.T) {
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct{ metric, dimension, value string }{
		{"bounce_rate", "event:name", "signup"}, {"pageviews", "visit:entry_page", "/landing"},
		{"pageviews", "visit:exit_page_hostname", "example.com"},
	} {
		q := &types.Query{SiteID: "42", Metrics: []string{tc.metric}, Timezone: "UTC", Now: now, UTCTimeRange: types.TimeRange{Start: now.Add(-time.Hour), End: now}, Filters: []*types.Filter{{Operator: "is", Dimension: tc.dimension, Values: []any{tc.value}}}}
		statement, args, err := NewQueryBuilder().Build(q, &types.Site{ID: "42", Timezone: "UTC"})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(statement, "session_id IN (SELECT session_id FROM") {
			t.Fatalf("missing membership filter: %s", statement)
		}
		found := false
		sites := 0
		for _, arg := range args {
			if arg == tc.value {
				found = true
			}
			if arg == "42" {
				sites++
			}
		}
		if !found || sites < 2 {
			t.Fatalf("foreign predicate or tenant scope lost: %v", args)
		}
	}
	wb := NewWhereBuilder("42")
	wb.FilterSiteTimeRange("events", now.Add(-time.Hour), now)
	err := wb.AddFilter("events", &types.Filter{Operator: "or", SubFilters: []*types.Filter{
		{Operator: "is", Dimension: "event:name", Values: []any{"signup"}},
		{Operator: "is", Dimension: "visit:entry_page", Values: []any{"/landing"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	condition, args := wb.Build()
	if !strings.Contains(condition, " OR ") || !strings.Contains(condition, "sessions FINAL") || len(args) < 7 {
		t.Fatalf("mixed OR lost scope or branch: %s %v", condition, args)
	}
}
