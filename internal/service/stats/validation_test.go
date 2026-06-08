package stats

import (
	"testing"

	"github.com/zenstats/zenstats/internal/service/stats/types"
)

func TestIsValidDimensionIncludesVisitPageDimensions(t *testing.T) {
	dimensions := []string{
		"visit:entry_page",
		"visit:entry_page_hostname",
		"visit:exit_page",
		"visit:exit_page_hostname",
	}

	for _, dimension := range dimensions {
		t.Run(dimension, func(t *testing.T) {
			if !isValidDimension(dimension) {
				t.Fatalf("expected %q to be a valid dimension", dimension)
			}
		})
	}
}

func TestValidateMetricsRejectsSessionMetricsForEventNameBreakdown(t *testing.T) {
	params := &types.Params{Dimensions: []string{"event:name"}}

	for _, metric := range []string{"pageviews", "bounce_rate", "visit_duration", "views_per_visit"} {
		t.Run(metric, func(t *testing.T) {
			if _, err := validateMetrics([]string{"visitors", metric}, params); err == nil {
				t.Fatalf("expected %q to be rejected for event:name breakdown", metric)
			}
		})
	}
}

func TestValidateMetricsAllowsEventMetricsForEventNameBreakdown(t *testing.T) {
	params := &types.Params{Dimensions: []string{"event:name"}}

	if _, err := validateMetrics([]string{"visitors", "events"}, params); err != nil {
		t.Fatalf("expected visitors/events to be valid for event:name breakdown: %v", err)
	}
}
