package service

import (
	"strings"
	"testing"
	"time"
)

func mustLoc(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("load location %s: %v", name, err)
	}
	return loc
}

func TestRenderReportHTML_EscapesUserAndVisitorContent(t *testing.T) {
	change := 12.5
	html := renderReportHTML(reportViewData{
		DisplayName: `<script>alert('site')</script>`,
		ReportName:  "Weekly",
		DateLabel:   "Aug 23, 2026",
		HasStats:    true,
		Stats: &reportStats{
			Visitors:  metricWithChange{Value: 100, Change: &change},
			Pageviews: metricWithChange{Value: 200},
			TopPages: []rankedItem{
				{Name: `/<script>alert('page')</script>`, Visitors: 10},
				{Name: `/"><img src=x onerror=alert(1)>`, Visitors: 5},
			},
			TopSources: []rankedItem{
				{Name: `<b onmouseover="alert('src')">evil</b>`, Visitors: 7},
			},
		},
		UnsubscribeNote: "To unsubscribe, disable email reports in site settings.",
	})

	for _, forbidden := range []string{
		"<script>", "</script>", "<img src=x", `onmouseover="alert`, `<b onmouseover`,
	} {
		if strings.Contains(html, forbidden) {
			t.Errorf("rendered HTML contains unescaped payload %q", forbidden)
		}
	}
	for _, expected := range []string{
		"&lt;script&gt;alert(&#39;site&#39;)&lt;/script&gt;",
		"/&lt;script&gt;alert(&#39;page&#39;)&lt;/script&gt;",
		"&lt;img src=x onerror=alert(1)&gt;",
		"&lt;b onmouseover=&#34;alert(&#39;src&#39;)&#34;&gt;evil&lt;/b&gt;",
	} {
		if !strings.Contains(html, expected) {
			t.Errorf("rendered HTML missing escaped form %q", expected)
		}
	}
}

func TestRenderReportHTML_EscapesStatsError(t *testing.T) {
	html := renderReportHTML(reportViewData{
		DisplayName: "example.com",
		StatsError:  `<script>alert('err')</script>`,
	})
	if strings.Contains(html, "<script>") {
		t.Fatal("stats error text was not escaped")
	}
}

// Weekly cron fires Monday 00:00 UTC. Whatever the site timezone, the report must cover the
// most recently completed Mon..Sun (2026-08-17 .. 2026-08-23 for a 2026-08-24 trigger).
func TestCalcDateRangeAt_WeeklyAtCronTrigger(t *testing.T) {
	trigger := time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC)
	if trigger.Weekday() != time.Monday {
		t.Fatalf("test fixture must be a Monday, got %s", trigger.Weekday())
	}

	for _, tz := range []string{"UTC", "America/Los_Angeles", "Asia/Tokyo", "Pacific/Kiritimati", "Etc/GMT+12"} {
		t.Run(tz, func(t *testing.T) {
			loc := mustLoc(t, tz)
			from, to := calcDateRangeAt(trigger, tz, "weekly")

			wantFrom := time.Date(2026, 8, 17, 0, 0, 0, 0, loc)
			wantTo := time.Date(2026, 8, 23, 23, 59, 59, 0, loc)
			if !from.Equal(wantFrom) {
				t.Errorf("from = %v, want %v", from, wantFrom)
			}
			if !to.Equal(wantTo) {
				t.Errorf("to = %v, want %v", to, wantTo)
			}
			if from.Weekday() != time.Monday || to.Weekday() != time.Sunday {
				t.Errorf("range must be Mon..Sun, got %s..%s", from.Weekday(), to.Weekday())
			}
			if got := reportName("weekly", to); got != "Weekly" {
				t.Errorf("reportName = %q, want Weekly", got)
			}
		})
	}
}

// Monthly cron fires on the 1st at 01:00 UTC. For every timezone the report must cover the
// previous calendar month, and the subject month label must match the body's range.
func TestCalcDateRangeAt_MonthlyAtCronTrigger(t *testing.T) {
	trigger := time.Date(2026, 8, 1, 1, 0, 0, 0, time.UTC)

	for _, tz := range []string{"UTC", "America/Los_Angeles", "Asia/Tokyo", "Pacific/Kiritimati", "Etc/GMT+12"} {
		t.Run(tz, func(t *testing.T) {
			loc := mustLoc(t, tz)
			from, to := calcDateRangeAt(trigger, tz, "monthly")

			wantFrom := time.Date(2026, 7, 1, 0, 0, 0, 0, loc)
			wantTo := time.Date(2026, 7, 31, 23, 59, 59, 0, loc)
			if !from.Equal(wantFrom) {
				t.Errorf("from = %v, want %v", from, wantFrom)
			}
			if !to.Equal(wantTo) {
				t.Errorf("to = %v, want %v", to, wantTo)
			}
			if got := reportName("monthly", to); got != "July" {
				t.Errorf("reportName = %q, want July", got)
			}
		})
	}
}

// Regression: before the anchor fix, a Los Angeles site at the Monday 00:00 UTC trigger was
// still on Sunday local time and got the week *before* the intended one. The January monthly
// trigger also crosses a year boundary, which must resolve to December of the previous year.
func TestCalcDateRangeAt_YearBoundaryMonthly(t *testing.T) {
	trigger := time.Date(2027, 1, 1, 1, 0, 0, 0, time.UTC)
	loc := mustLoc(t, "America/Los_Angeles")
	from, to := calcDateRangeAt(trigger, "America/Los_Angeles", "monthly")
	if !from.Equal(time.Date(2026, 12, 1, 0, 0, 0, 0, loc)) || !to.Equal(time.Date(2026, 12, 31, 23, 59, 59, 0, loc)) {
		t.Fatalf("got %v .. %v, want Dec 2026", from, to)
	}
	if got := reportName("monthly", to); got != "December" {
		t.Fatalf("reportName = %q, want December", got)
	}
}

func TestCalcDateRangeAt_InvalidTimezoneFallsBackToUTC(t *testing.T) {
	trigger := time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC)
	from, _ := calcDateRangeAt(trigger, "Not/AZone", "weekly")
	if from.Location() != time.UTC {
		t.Fatalf("expected UTC fallback, got %v", from.Location())
	}
}
