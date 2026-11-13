package sql

import (
	"strings"
	"testing"
	"time"

	"github.com/zenstats/zenstats/internal/service/stats/types"
)

func TestEventNameCountsOnlyComputeRequestedVisitors(t *testing.T) {
	for _, tc := range []struct {
		name    string
		metrics []string
		wantUV  bool
	}{
		{name: "event counts", metrics: []string{"events"}},
		{name: "explicit visitors", metrics: []string{"events", "visitors"}, wantUV: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Date(2026, 10, 14, 16, 0, 0, 0, time.UTC)
			query := &types.Query{
				SiteID:       "1",
				UTCTimeRange: types.TimeRange{Start: start, End: start.AddDate(0, 0, 30)},
				Metrics:      tc.metrics,
				Dimensions:   []string{"event:name"},
				OrderBy:      []*types.OrderBy{{Dimension: "events", Direction: "desc"}},
				Pagination:   &types.Pagination{Limit: 50},
			}
			statement, args, err := NewQueryBuilder().Build(query, &types.Site{ID: "1", Timezone: "UTC"})
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(statement, "uniqExact(user_id)") != tc.wantUV {
				t.Fatalf("unexpected visitor aggregation (want %v): %s", tc.wantUV, statement)
			}
			for _, fragment := range []string{"countIf(name != 'engagement') as events", "GROUP BY name", "ORDER BY events desc", "LIMIT 50", "site_id = ?", "timestamp >=", "timestamp <="} {
				if !strings.Contains(statement, fragment) {
					t.Errorf("missing %q: %s", fragment, statement)
				}
			}
			if len(args) < 3 || args[0] != "1" {
				t.Errorf("expected site and time bounds, got %v", args)
			}
		})
	}
}
