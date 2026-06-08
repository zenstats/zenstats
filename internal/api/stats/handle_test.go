package stats

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func newStatsValidateContext(rawQuery string) *gin.Context {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	request := httptest.NewRequest(http.MethodGet, "/api/stats/example.com/aggregate?"+rawQuery, nil)
	ctx.Request = request
	return ctx
}

func TestStatsHandleValidateSetsDefaultDateForNonRealtimePeriod(t *testing.T) {
	handle := &StatsHandle{}

	req, err := handle.validate(newStatsValidateContext("period=day"))
	if err != nil {
		t.Fatalf("validate returned unexpected error: %v", err)
	}

	want := time.Now().Format("2006-01-02")
	if req.Date != want {
		t.Fatalf("default date = %q, want %q", req.Date, want)
	}
}

func TestStatsHandleValidateRequiresCustomDateRange(t *testing.T) {
	handle := &StatsHandle{}

	if _, err := handle.validate(newStatsValidateContext("period=custom&from=2026-01-01")); err == nil {
		t.Fatal("expected custom period without to date to fail")
	}

	if _, err := handle.validate(newStatsValidateContext("period=custom&from=2026-01-01&to=2026-01-31")); err != nil {
		t.Fatalf("expected valid custom date range, got error: %v", err)
	}
}

func TestStatsHandleValidateBindsMetricsAndFiltersFromQuery(t *testing.T) {
	handle := &StatsHandle{}

	req, err := handle.validate(newStatsValidateContext(`period=day&metrics=visitors,pageviews&filters=%5B%5B%22is%22,%22visit:browser%22,%5B%22Chrome%22%5D%5D%5D`))
	if err != nil {
		t.Fatalf("validate returned unexpected error: %v", err)
	}
	if req.Metrics != "visitors,pageviews" {
		t.Fatalf("metrics = %q, want visitors,pageviews", req.Metrics)
	}
	if req.Filters != `[["is","visit:browser",["Chrome"]]]` {
		t.Fatalf("filters = %q", req.Filters)
	}
}

func TestStatsHandleValidateRejectsInvalidDates(t *testing.T) {
	handle := &StatsHandle{}

	tests := []string{
		"period=day&date=2026-02-31",
		"period=custom&from=2026-13-01&to=2026-01-31",
		"period=custom&from=2026-01-01&to=not-a-date",
	}

	for _, rawQuery := range tests {
		t.Run(rawQuery, func(t *testing.T) {
			if _, err := handle.validate(newStatsValidateContext(rawQuery)); err == nil {
				t.Fatal("expected invalid date query to fail")
			}
		})
	}
}

func TestStatsHandleValidateRealtimeDoesNotRequireDate(t *testing.T) {
	handle := &StatsHandle{}

	req, err := handle.validate(newStatsValidateContext("period=realtime"))
	if err != nil {
		t.Fatalf("validate returned unexpected error: %v", err)
	}
	if req.Date != "" {
		t.Fatalf("realtime date = %q, want empty", req.Date)
	}
}

func TestStatsHandleValidateYesterdayDefaultsToYesterdayDate(t *testing.T) {
	handle := &StatsHandle{}

	req, err := handle.validate(newStatsValidateContext("period=yesterday"))
	if err != nil {
		t.Fatalf("validate returned unexpected error: %v", err)
	}
	want := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	if req.Date != want {
		t.Fatalf("yesterday date = %q, want %q", req.Date, want)
	}
}
