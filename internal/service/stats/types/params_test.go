package types

import (
	"testing"
	"time"
)

func TestPeriodRangesKeepIndependentBoundaries(t *testing.T) {
	for _, tc := range []struct{ period, start, end string }{
		{"day", "2026-11-09T16:00:00Z", "2026-11-10T15:59:59Z"},
		{"p7", "2026-11-03T16:00:00Z", "2026-11-10T15:59:59Z"},
		{"p14", "2026-10-27T16:00:00Z", "2026-11-10T15:59:59Z"},
		{"p30", "2026-10-11T16:00:00Z", "2026-11-10T15:59:59Z"},
		{"w", "2026-11-03T16:00:00Z", "2026-11-10T15:59:59Z"},
		{"m", "2026-10-31T16:00:00Z", "2026-11-30T15:59:59Z"},
		{"year", "2025-12-31T16:00:00Z", "2026-12-31T15:59:59Z"},
		{"30d", "2026-10-11T16:00:00Z", "2026-11-10T15:59:59Z"},
	} {
		t.Run(tc.period, func(t *testing.T) {
			p := &Params{Period: tc.period, Date: "2026-11-10"}
			if err := p.ParsePeriodToUTCTimeRange("Asia/Shanghai"); err != nil {
				t.Fatal(err)
			}
			if p.UTCTimeRange.Start.Format(time.RFC3339) != tc.start || p.UTCTimeRange.End.Format(time.RFC3339) != tc.end {
				t.Fatalf("range collapsed or shifted: %+v", p.UTCTimeRange)
			}
		})
	}
	p := &Params{Period: "realtime"}
	if err := p.ParsePeriodToUTCTimeRange("UTC"); err != nil {
		t.Fatal(err)
	}
	if duration := p.UTCTimeRange.End.Sub(p.UTCTimeRange.Start); duration != 30*time.Minute {
		t.Fatalf("realtime duration=%s", duration)
	}
}

func TestCustomPeriodIncludesFinalDayAndRespectsDST(t *testing.T) {
	p := &Params{Period: "custom", From: "2026-11-01", To: "2026-11-01"}
	if err := p.ParsePeriodToUTCTimeRange("America/New_York"); err != nil {
		t.Fatal(err)
	}
	// DST ends on this day: the selected local day contains 25 hours.
	if p.UTCTimeRange.Start.Format(time.RFC3339) != "2026-11-01T04:00:00Z" || p.UTCTimeRange.End.Format(time.RFC3339) != "2026-11-02T04:59:59Z" {
		t.Fatalf("DST day range: %+v", p.UTCTimeRange)
	}
	p = &Params{Period: "custom", From: "2026-11-10 09:30", To: "2026-11-10 10:45"}
	if err := p.ParsePeriodToUTCTimeRange("Asia/Shanghai"); err != nil {
		t.Fatal(err)
	}
	if duration := p.UTCTimeRange.End.Sub(p.UTCTimeRange.Start); duration != 75*time.Minute {
		t.Fatalf("exact times were expanded: %s", duration)
	}
}
