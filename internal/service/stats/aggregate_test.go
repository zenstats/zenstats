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

// TestConversionRate 验证转化率计算与除零保护。
func TestConversionRate(t *testing.T) {
	cases := []struct {
		conv, base, want float64
	}{
		{5, 96, 5.2},   // 5/96*100=5.208 → 5.2
		{11, 96, 11.5}, // 11/96*100=11.458 → 11.5
		{0, 100, 0},
		{5, 0, 0}, // 分母为 0 → 0，不 panic
		{96, 96, 100},
	}
	for _, c := range cases {
		if got := conversionRate(c.conv, c.base); got != c.want {
			t.Errorf("conversionRate(%v,%v)=%v want %v", c.conv, c.base, got, c.want)
		}
	}
}

// TestReplaceConversionRate 验证 conversion_rate 被替换为 conversions 且去重。
func TestReplaceConversionRate(t *testing.T) {
	got := replaceConversionRate([]string{"visitors", "conversion_rate"})
	if len(got) != 2 || got[1] != "conversions" {
		t.Fatalf("expected [visitors conversions], got %v", got)
	}
	// 已含 conversions 时不重复添加
	got = replaceConversionRate([]string{"conversions", "conversion_rate"})
	if len(got) != 1 || got[0] != "conversions" {
		t.Fatalf("expected [conversions], got %v", got)
	}
}

// TestStripGoalFilters 验证 goal 过滤被剔除、其余过滤保留。
func TestStripGoalFilters(t *testing.T) {
	filters := []*types.Filter{
		{Operator: "is", Dimension: "event:goal", Values: []any{"Signup"}},
		{Operator: "is", Dimension: "visit:browser", Values: []any{"Chrome"}},
	}
	out := stripGoalFilters(filters)
	if len(out) != 1 || out[0].Dimension != "visit:browser" {
		t.Fatalf("expected only browser filter retained, got %v", out)
	}
}
