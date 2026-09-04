package sql

import (
	"strings"
	"testing"

	"github.com/zenstats/zenstats/internal/service/stats/types"
)

// 回归测试：时序查询（纯时间维度、无分页）不得附加默认 LIMIT 200。
// 30d×hourly 有 720 个桶，历史上默认 LIMIT 200 且无 ORDER BY 会任意丢弃 520 个桶，
// 补零后主图曲线完全错乱。
func TestPaginateSkipsDefaultLimitForTimeDimensions(t *testing.T) {
	qb := &QueryBuilder{}
	base := "SELECT x"

	timeQ := &types.Query{Dimensions: []string{"time:hour"}}
	if got := qb.paginate(base, timeQ); strings.Contains(got, "LIMIT") {
		t.Fatalf("time-dimension query must not get a default LIMIT, got: %s", got)
	}

	dimQ := &types.Query{Dimensions: []string{"event:page"}}
	if got := qb.paginate(base, dimQ); !strings.Contains(got, "LIMIT 200") {
		t.Fatalf("non-time dimension query should keep default LIMIT 200, got: %s", got)
	}

	mixedQ := &types.Query{Dimensions: []string{"time:hour", "event:page"}}
	if got := qb.paginate(base, mixedQ); !strings.Contains(got, "LIMIT 200") {
		t.Fatalf("mixed dimensions should keep default LIMIT 200, got: %s", got)
	}

	explicit := &types.Query{Dimensions: []string{"time:hour"}, Pagination: &types.Pagination{Limit: 50}}
	if got := qb.paginate(base, explicit); !strings.Contains(got, "LIMIT 50") {
		t.Fatalf("explicit pagination must be honored, got: %s", got)
	}
}
