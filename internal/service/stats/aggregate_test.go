package stats

import (
	"testing"
	"time"

	"github.com/zenstats/zenstats/internal/service/stats/types"
)

func TestTimeSeriesDefaultIntervalUsesDailyDimension(t *testing.T) {
	interval, err := ParseInterval("")
	if err != nil {
		t.Fatalf("ParseInterval empty returned error: %v", err)
	}

	if got := intervalToDimension(string(interval)); got != "time:day" {
		t.Fatalf("default interval dimension = %q, want time:day", got)
	}
}

func TestFillMissingDailyPointsUsesReturnedDayKeys(t *testing.T) {
	service := NewAggregateService(nil)
	params := &types.Params{
		Timezone: "Asia/Shanghai",
		Metrics:  []string{"visitors"},
		UTCTimeRange: types.TimeRange{
			Start: mustParseTime(t, "2026-05-25T16:00:00Z"),
			End:   mustParseTime(t, "2026-06-01T15:59:59Z"),
		},
	}

	points := []TimeSeriesPoint{
		{Timestamp: "2026-05-26", Metrics: map[string]any{"visitors": uint64(3)}},
	}

	filled := service.fillMissingTimePoints(points, IntervalDaily, params)
	if len(filled) == 0 {
		t.Fatal("expected filled points")
	}

	found := false
	for _, point := range filled {
		if point.Timestamp == "2026-05-26" {
			found = true
			if point.Metrics["visitors"] != uint64(3) {
				t.Fatalf("filled point visitors = %#v, want 3", point.Metrics["visitors"])
			}
		}
	}
	if !found {
		t.Fatalf("expected returned day timestamp to be preserved in filled points: %#v", filled)
	}
}

func mustParseTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("parse time %q: %v", value, err)
	}
	return parsed
}
