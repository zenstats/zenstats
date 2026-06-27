package stats

import "testing"

// TestCalculateTotalReturnsNilForRateMetrics verifies the bug fix: _rate metrics
// must return nil (not a simple average) because a correct weighted average requires
// per-row visitor counts that are not available in this aggregation path.
func TestCalculateTotalReturnsNilForRateMetrics(t *testing.T) {
	values := []BreakdownValue{
		{Value: "Chrome", Stats: map[string]any{
			"bounce_rate": float64(0.50),
			"visitors":    float64(100),
		}},
		{Value: "Firefox", Stats: map[string]any{
			"bounce_rate": float64(0.40),
			"visitors":    float64(50),
		}},
	}
	total := calculateTotal(values, []string{"bounce_rate", "visitors"})

	if total["bounce_rate"] != nil {
		t.Fatalf("expected bounce_rate total == nil (not a simple-average-able metric), got %v", total["bounce_rate"])
	}
	if total["visitors"] == nil {
		t.Fatal("expected non-nil visitors total")
	}
	if got, ok := total["visitors"].(float64); !ok || got != 150 {
		t.Fatalf("expected visitors total == 150.0, got %v", total["visitors"])
	}
}

func TestCalculateTotalAllRateMetricsAreNil(t *testing.T) {
	rateMetrics := []string{
		"bounce_rate",
		"conversion_rate",
		"exit_rate",
		"scroll_rate",
	}
	values := []BreakdownValue{
		{Value: "x", Stats: map[string]any{
			"bounce_rate":     float64(0.3),
			"conversion_rate": float64(0.1),
			"exit_rate":       float64(0.4),
			"scroll_rate":     float64(0.8),
		}},
	}
	total := calculateTotal(values, rateMetrics)

	for _, m := range rateMetrics {
		if total[m] != nil {
			t.Errorf("expected %s total == nil, got %v", m, total[m])
		}
	}
}

// TestCalculateTotalSumsNonRateMetrics verifies that non-_rate metrics are summed
// correctly across all breakdown values.
func TestCalculateTotalSumsNonRateMetrics(t *testing.T) {
	values := []BreakdownValue{
		{Value: "a", Stats: map[string]any{"pageviews": float64(10), "events": float64(3)}},
		{Value: "b", Stats: map[string]any{"pageviews": float64(20), "events": float64(7)}},
		{Value: "c", Stats: map[string]any{"pageviews": float64(5), "events": float64(2)}},
	}
	total := calculateTotal(values, []string{"pageviews", "events"})

	if got, ok := total["pageviews"].(float64); !ok || got != 35 {
		t.Fatalf("expected pageviews total == 35, got %v", total["pageviews"])
	}
	if got, ok := total["events"].(float64); !ok || got != 12 {
		t.Fatalf("expected events total == 12, got %v", total["events"])
	}
}

// TestCalculateTotalEmptyValuesReturnsZeroForNonRate verifies that non-_rate
// metrics return 0 (not nil) when there are no breakdown values.
func TestCalculateTotalEmptyValuesReturnsZeroForNonRate(t *testing.T) {
	total := calculateTotal([]BreakdownValue{}, []string{"visitors", "conversion_rate"})

	if total["conversion_rate"] != nil {
		t.Fatalf("expected conversion_rate == nil for empty values, got %v", total["conversion_rate"])
	}
	if total["visitors"] != 0 {
		t.Fatalf("expected visitors == 0 for empty values, got %v", total["visitors"])
	}
}

// TestCalculateTotalMixedMetrics verifies a realistic mixed-metric scenario.
func TestCalculateTotalMixedMetrics(t *testing.T) {
	values := []BreakdownValue{
		{Value: "US", Stats: map[string]any{
			"visitors":    float64(1000),
			"pageviews":   float64(3000),
			"bounce_rate": float64(0.45),
		}},
		{Value: "GB", Stats: map[string]any{
			"visitors":    float64(500),
			"pageviews":   float64(1200),
			"bounce_rate": float64(0.38),
		}},
	}
	metrics := []string{"visitors", "pageviews", "bounce_rate"}
	total := calculateTotal(values, metrics)

	if v, ok := total["visitors"].(float64); !ok || v != 1500 {
		t.Errorf("visitors total: want 1500, got %v", total["visitors"])
	}
	if v, ok := total["pageviews"].(float64); !ok || v != 4200 {
		t.Errorf("pageviews total: want 4200, got %v", total["pageviews"])
	}
	if total["bounce_rate"] != nil {
		t.Errorf("bounce_rate total: want nil, got %v", total["bounce_rate"])
	}
}
