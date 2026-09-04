package stats

import (
	"testing"
	"time"

	"github.com/zenstats/zenstats/internal/service/stats/types"
)

func TestEstimateBucketCount(t *testing.T) {
	start := time.Date(2026, 8, 6, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		days     int
		interval Interval
		min, max int
	}{
		{30, IntervalHourly, 720, 721},   // 30d 按小时：官方 UI 支持的最大 hourly 组合
		{365, IntervalDaily, 365, 366},   // 12mo 按天
		{730, IntervalHourly, 17520, 17545}, // 2 年按小时：应超过 maxTimeSeriesBuckets
	}
	for _, c := range cases {
		tr := types.TimeRange{Start: start, End: start.AddDate(0, 0, c.days)}
		got := estimateBucketCount(tr, c.interval)
		if got < c.min || got > c.max {
			t.Errorf("estimateBucketCount(%dd, %s) = %d, want [%d,%d]", c.days, c.interval, got, c.min, c.max)
		}
	}
	if n := estimateBucketCount(types.TimeRange{Start: start.AddDate(0, 0, 30), End: start}, IntervalHourly); n != 0 {
		t.Errorf("inverted range should be 0 buckets, got %d", n)
	}
	// 30d hourly 必须在上限之内，2 年 hourly 必须被拒绝
	tr30 := types.TimeRange{Start: start, End: start.AddDate(0, 0, 30)}
	if estimateBucketCount(tr30, IntervalHourly) > maxTimeSeriesBuckets {
		t.Error("30d hourly must be within maxTimeSeriesBuckets")
	}
	tr2y := types.TimeRange{Start: start, End: start.AddDate(2, 0, 0)}
	if estimateBucketCount(tr2y, IntervalHourly) <= maxTimeSeriesBuckets {
		t.Error("2y hourly must exceed maxTimeSeriesBuckets")
	}
}
