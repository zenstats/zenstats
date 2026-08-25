package service

import (
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestRenderAlertHTML_EscapesDomainAndHref(t *testing.T) {
	domain := `evil.com"><script>alert(1)</script>`
	alert := &alertData{
		CurrentVisitors: 10, PreviousVisitors: 100,
		CurrentPageviews: 20, PreviousPageviews: 200,
		VisitorsChange: -90, PageviewsChange: -90,
		IsSpike:   false,
		Hour:      `<script>alert('hour')</script>`,
		Threshold: 50,
	}

	html := renderAlertHTML("https://zenstats.example", domain, alert)

	if strings.Contains(html, "<script>") {
		t.Fatal("alert HTML contains unescaped <script>")
	}
	if !strings.Contains(html, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Error("domain was not HTML-escaped in body")
	}
	wantHref := `href="` + strings.ReplaceAll("https://zenstats.example/sites/"+url.PathEscape(domain)+"/stats", `"`, "&#34;") + `"`
	if !strings.Contains(html, wantHref) {
		t.Errorf("href not built with path escaping, want %s", wantHref)
	}
	if strings.Contains(html, `href="https://zenstats.example/sites/evil.com"><script>`) {
		t.Error("raw domain leaked into href attribute")
	}
	if strings.Contains(html, `href="https://zenstats.example/sites/evil.com"`) {
		t.Error("href attribute terminated early by unescaped quote")
	}
}

func TestComputeAlertWindow_DailyUsesYesterdayInSiteTimezone(t *testing.T) {
	// 2026-08-25 01:00 UTC: Tokyo is already Aug 25 10:00, Los Angeles is still Aug 24 18:00.
	now := time.Date(2026, 8, 25, 1, 0, 0, 0, time.UTC)

	cases := []struct {
		tz                string
		wantDay, wantPrev string
	}{
		{"UTC", "2026-08-24", "2026-08-17"},
		{"Asia/Tokyo", "2026-08-24", "2026-08-17"},
		{"America/Los_Angeles", "2026-08-23", "2026-08-16"},
	}
	for _, c := range cases {
		t.Run(c.tz, func(t *testing.T) {
			w := computeAlertWindow(now, c.tz, "daily")
			if w.From != c.wantDay || w.To != c.wantDay {
				t.Errorf("current = %s..%s, want %s (full day)", w.From, w.To, c.wantDay)
			}
			if w.PrevFrom != c.wantPrev || w.PrevTo != c.wantPrev {
				t.Errorf("previous = %s..%s, want %s", w.PrevFrom, w.PrevTo, c.wantPrev)
			}
			// Must never compare against the partial current day.
			today := now.In(mustLoc(t, c.tz)).Format("2006-01-02")
			if w.From == today {
				t.Errorf("daily window must not use today (%s)", today)
			}
			if w.Label != c.wantDay+" vs "+c.wantPrev {
				t.Errorf("label = %q", w.Label)
			}
		})
	}
}

func TestComputeAlertWindow_HourlyUsesSiteLocalHour(t *testing.T) {
	now := time.Date(2026, 8, 25, 1, 30, 0, 0, time.UTC) // Tokyo 10:30, LA 18:30 (Aug 24)

	w := computeAlertWindow(now, "Asia/Tokyo", "hourly")
	if w.From != "2026-08-25 09:00" || w.To != "2026-08-25 10:00" {
		t.Errorf("Tokyo current = %s..%s", w.From, w.To)
	}
	if w.PrevFrom != "2026-08-18 09:00" || w.PrevTo != "2026-08-18 10:00" {
		t.Errorf("Tokyo previous = %s..%s", w.PrevFrom, w.PrevTo)
	}

	w = computeAlertWindow(now, "America/Los_Angeles", "hourly")
	if w.From != "2026-08-24 17:00" || w.To != "2026-08-24 18:00" {
		t.Errorf("LA current = %s..%s", w.From, w.To)
	}

	// Half-hour offset zones must still yield whole local hours.
	w = computeAlertWindow(now, "Asia/Kolkata", "hourly") // 07:00 local
	if w.From != "2026-08-25 06:00" || w.To != "2026-08-25 07:00" {
		t.Errorf("Kolkata current = %s..%s", w.From, w.To)
	}
}

func TestAlertState_DailyEvaluatedOncePerLocalDay(t *testing.T) {
	st := newAlertState()
	const site = int64(42)

	if !st.shouldEvalDaily(site, "2026-08-25") {
		t.Fatal("first evaluation of the day must run")
	}
	st.markDailyEvaluated(site, "2026-08-25")
	for i := 0; i < 23; i++ {
		if st.shouldEvalDaily(site, "2026-08-25") {
			t.Fatalf("hourly tick %d re-evaluated the same local day", i)
		}
	}
	if !st.shouldEvalDaily(site, "2026-08-26") {
		t.Fatal("next local day must evaluate again")
	}
	if !st.shouldEvalDaily(int64(7), "2026-08-25") {
		t.Fatal("state must be per-site")
	}
}

func TestAlertState_HourlySuppressesOngoingAnomaly(t *testing.T) {
	st := newAlertState()
	const site = int64(1)
	drop := &alertData{IsSpike: false}
	spike := &alertData{IsSpike: true}

	if !st.shouldSendHourly(site, drop) {
		t.Fatal("first hour of an anomaly must alert")
	}
	if st.shouldSendHourly(site, drop) || st.shouldSendHourly(site, drop) {
		t.Fatal("persisting anomaly must not re-alert every hour")
	}
	if !st.shouldSendHourly(site, spike) {
		t.Fatal("direction flip (drop -> spike) is a new anomaly and must alert")
	}
	if st.shouldSendHourly(site, nil) {
		t.Fatal("a normal hour never sends")
	}
	if !st.shouldSendHourly(site, drop) {
		t.Fatal("after recovery a new anomaly must alert again")
	}
}
