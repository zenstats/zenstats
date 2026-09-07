package funnel

import (
	"strings"
	"testing"
	"time"
)

// 回归测试：漏斗 CTE 必须用 minOrNullIf。
// minIf 对非 Nullable DateTime 在无匹配时返回 1970-01-01 而非 NULL，
// 会让 ts_N IS NOT NULL 恒为真，第一步统计成"匹配任意步骤的所有用户"。
func TestBuildFunnelQueryUsesMinOrNullIf(t *testing.T) {
	svc := &AnalysisService{}
	p1 := int64(1)
	req := &AnalysisRequest{
		SiteID: "1",
		Steps: []*FunnelStep{
			{StepID: 1, GoalID: 1, GoalType: "page", GoalValue: "/soft/34460.html", GoalName: "visit", StepOrder: 1},
			{StepID: 2, ParentStepID: &p1, GoalID: 2, GoalType: "event", GoalValue: "download", GoalName: "dl", StepOrder: 2},
		},
		StartTime: time.Now().Add(-24 * time.Hour),
		EndTime:   time.Now(),
	}
	query, args := svc.buildFunnelQuery(req)

	if strings.Contains(query, "minIf(") {
		t.Fatalf("query must not use minIf (returns epoch instead of NULL for non-matching users):\n%s", query)
	}
	if !strings.Contains(query, "minOrNullIf(timestamp") {
		t.Fatalf("query must use minOrNullIf:\n%s", query)
	}
	// 占位符数量与参数数量一致：2 个 goal（minIf 段）+ site/start/end + 2 个 goal（OR 段）
	if got, want := strings.Count(query, "?"), len(args); got != want {
		t.Fatalf("placeholder count %d != args count %d\n%s", got, want, query)
	}
	if len(args) != 7 {
		t.Fatalf("expected 7 args (2 goals + 3 range + 2 goals), got %d: %v", len(args), args)
	}
	// 页面目标应限定 pageview + pathname 精确匹配
	if !strings.Contains(query, "(name = 'pageview' AND pathname = ?)") {
		t.Fatalf("page goal condition missing:\n%s", query)
	}
}
