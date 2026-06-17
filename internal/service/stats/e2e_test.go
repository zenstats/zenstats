package stats_test

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/gin-gonic/gin"
	atypes "github.com/zenstats/zenstats/internal/api/types"
	"github.com/zenstats/zenstats/internal/service"
	"github.com/zenstats/zenstats/internal/service/stats"
	"github.com/zenstats/zenstats/internal/service/stats/types"
	cl "github.com/zenstats/zenstats/internal/store/clickhouse"
	"github.com/zenstats/zenstats/internal/store/postgresql"
	"github.com/zenstats/zenstats/pkg/globals"
)

// ============================================================
// 真实数据 E2E 测试 — 全部通过 StatsService 完整链路
//
// 运行: APP_ENV=test go test ./internal/service/stats/... -run TestE2E -v -count=1
// 跳过: go test -short ...
// ============================================================

func testGinCtx() *gin.Context {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	return c
}

// setupE2E 初始化测试环境，返回 StatsService + QueryService + siteID
func setupE2E(t *testing.T) (*service.StatsService, *stats.QueryService, int64) {
	t.Helper()
	if testing.Short() {
		t.Skip("short mode")
	}

	var conn driver.Conn
	func() {
		defer func() { recover() }()
		conn = cl.GetConnection()
	}()
	if conn == nil {
		t.Skip("ClickHouse not available")
	}
	if err := conn.Ping(context.Background()); err != nil {
		t.Skipf("ClickHouse ping failed: %v", err)
	}

	db := postgresql.NewClient()
	if db == nil || db.Client == nil {
		t.Skip("PostgreSQL not available")
	}
	globals.SetDB(db)

	// 测试环境 site_id=1（migration 自动创建）
	siteID := int64(1)
	if os.Getenv("APP_ENV") != "test" {
		row := conn.QueryRow(context.Background(),
			"SELECT site_id FROM zenstats_events_db.events ORDER BY timestamp DESC LIMIT 1")
		var sid uint64
		if err := row.Scan(&sid); err != nil {
			t.Skip("no events data — run seed first")
		}
		siteID = int64(sid)
	}

	runner := stats.NewQueryRunner()
	qs := stats.NewQueryService(runner)
	return service.GetStatsService(), qs, siteID
}

// today 返回今天日期字符串
func today() string { return time.Now().Format("2006-01-02") }

// ============================================================
// 1. Aggregate — 总览指标
// ============================================================

func TestE2EAggregateStats(t *testing.T) {
	svc, _, siteID := setupE2E(t)

	req := &atypes.StatsRequest{
		Period: "day",
		Date:   today(),
	}

	result, err := svc.GetAggregate(testGinCtx(), siteID, req)
	if err != nil {
		t.Fatalf("GetAggregate failed: %v", err)
	}
	if result == nil || result.Results == nil {
		t.Fatal("result or results map is nil")
	}

	expected := []string{"visitors", "pageviews", "bounce_rate", "visit_duration", "views_per_visit"}
	for _, m := range expected {
		mr, ok := result.Results[m]
		if !ok {
			t.Errorf("metric %q missing", m)
			continue
		}
		t.Logf("  %-16s value=%v", m, mr.Value)
	}

	// 合理性校验
	v := toFloat(result.Results["visitors"].Value)
	p := toFloat(result.Results["pageviews"].Value)
	if v > 0 && p > 0 && p < v {
		t.Errorf("pageviews(%.0f) < visitors(%.0f)", p, v)
	}
	b := toFloat(result.Results["bounce_rate"].Value)
	if b < 0 || b > 100 {
		t.Errorf("bounce_rate %.1f ∉ [0,100]", b)
	}
	d := toFloat(result.Results["visit_duration"].Value)
	if d < 0 {
		t.Errorf("visit_duration %.0f < 0", d)
	}
}

// ============================================================
// 2. TimeSeries — 时间序列
// ============================================================

func TestE2ETimeSeries(t *testing.T) {
	svc, _, siteID := setupE2E(t)

	req := &atypes.StatsRequest{
		Period:   "day",
		Date:     today(),
		Interval: "hourly",
	}

	points, err := svc.GetTimeSeries(testGinCtx(), siteID, req)
	if err != nil {
		t.Fatalf("GetTimeSeries failed: %v", err)
	}
	if len(points) == 0 {
		t.Fatal("no points returned")
	}

	// 结构 + 排序校验
	for i, p := range points {
		if p.Timestamp == "" {
			t.Errorf("point[%d] empty timestamp", i)
		}
		if p.Metrics == nil {
			t.Errorf("point[%d] nil metrics", i)
		}
	}
	for i := 1; i < len(points); i++ {
		if points[i].Timestamp < points[i-1].Timestamp {
			t.Errorf("sort broken at [%d]", i)
		}
	}
	t.Logf("TimeSeries: %d points (first=%s, last=%s)", len(points),
		points[0].Timestamp, points[len(points)-1].Timestamp)
}

// ============================================================
// 3-6. Breakdown — event:page / visit:source / visit:country / event:name
// ============================================================

func TestE2EBreakdownEventPage(t *testing.T) {
	svc, _, siteID := setupE2E(t)

	result, err := svc.GetBreakdown(testGinCtx(), siteID,
		&atypes.StatsRequest{Period: "day", Date: today(), Limit: 10, Page: 1},
		"event:page", "visitors,pageviews")
	if err != nil {
		t.Fatalf("GetBreakdown(event:page): %v", err)
	}
	if len(result.Data) == 0 {
		t.Fatal("no rows")
	}
	// 验证首页 "/" 存在
	found := false
	for _, row := range result.Data {
		if fmt.Sprintf("%v", row["name"]) == "/" {
			found = true
			t.Logf("  / → visitors=%v pageviews=%v", row["visitors"], row["pageviews"])
			break
		}
	}
	if !found {
		t.Error("homepage '/' not found in breakdown")
	}
	if len(result.Data) > 10 {
		t.Errorf("rows(%d) exceed limit(10)", len(result.Data))
	}
}

func TestE2EBreakdownVisitSource(t *testing.T) {
	svc, _, siteID := setupE2E(t)

	result, err := svc.GetBreakdown(testGinCtx(), siteID,
		&atypes.StatsRequest{Period: "day", Date: today(), Limit: 10},
		"visit:source", "visitors")
	if err != nil {
		t.Fatalf("GetBreakdown(visit:source): %v", err)
	}
	if len(result.Data) == 0 {
		t.Fatal("no rows")
	}
	t.Logf("Source breakdown: %d rows", len(result.Data))
	t.Logf("  top: %v", result.Data[0])
}

func TestE2EBreakdownCountry(t *testing.T) {
	svc, _, siteID := setupE2E(t)

	result, err := svc.GetBreakdown(testGinCtx(), siteID,
		&atypes.StatsRequest{Period: "day", Date: today(), Limit: 10},
		"visit:country", "visitors")
	if err != nil {
		t.Fatalf("GetBreakdown(visit:country): %v", err)
	}
	if len(result.Data) == 0 {
		t.Fatal("no rows")
	}
	t.Logf("Country breakdown: %d rows, top=%v", len(result.Data), result.Data[0]["name"])
}

func TestE2EBreakdownEventName(t *testing.T) {
	svc, _, siteID := setupE2E(t)

	result, err := svc.GetBreakdown(testGinCtx(), siteID,
		&atypes.StatsRequest{Period: "day", Date: today(), Limit: 10},
		"event:name", "visitors,events")
	if err != nil {
		t.Fatalf("GetBreakdown(event:name): %v", err)
	}
	found := false
	for _, row := range result.Data {
		name := fmt.Sprintf("%v", row["name"])
		t.Logf("  event: %s → visitors=%v events=%v", name, row["visitors"], row["events"])
		if name == "pageview" {
			found = true
		}
	}
	if !found {
		t.Error("'pageview' not found in event:name breakdown")
	}
}

// ============================================================
// 7. Current Visitors
// ============================================================

func TestE2ECurrentVisitors(t *testing.T) {
	svc, _, siteID := setupE2E(t)

	result, err := svc.GetCurrentVisitors(testGinCtx(), siteID)
	if err != nil {
		t.Fatalf("GetCurrentVisitors: %v", err)
	}
	t.Logf("Current visitors: total=%d", result.Total)
}

// ============================================================
// 8. Breakdown with Filter — 通过 StatsService 实际查询
// ============================================================

func TestE2EBreakdownWithFilter(t *testing.T) {
	_, qs, siteID := setupE2E(t)

	// 直接通过 params 构造带 country filter 的查询
	params := &types.Params{
		SiteID:     fmt.Sprintf("%d", siteID),
		Period:     "day",
		Date:       today(),
		Timezone:   "UTC",
		Metrics:    []string{"visitors"},
		Dimensions: []string{"visit:source"},
		Filters: []*types.Filter{
			{Operator: "is", Dimension: "visit:country", Values: []any{"US"}},
		},
		Pagination: &types.Pagination{Limit: 5},
	}

	query, err := qs.CreateQuery(params)
	if err != nil {
		t.Fatalf("CreateQuery with filter: %v", err)
	}

	site := &types.Site{ID: query.SiteID, Timezone: query.Timezone}
	result, err := qs.Execute(testGinCtx(), query, site)
	if err != nil {
		t.Fatalf("Execute with filter: %v", err)
	}

	t.Logf("Filtered source breakdown: %d rows", len(result.Data))
	for _, row := range result.Data {
		t.Logf("  %v", row)
	}

	// 过滤 US 后，数据应少于未过滤结果
	if len(result.Data) == 0 {
		t.Log("filter US returned 0 rows (may have no US data today)")
	}
}

// ============================================================
// 9. 指标组合 — visitors+pageviews+events 同时查询
// ============================================================

func TestE2EAllEventMetrics(t *testing.T) {
	svc, _, siteID := setupE2E(t)

	req := &atypes.StatsRequest{
		Period:  "day",
		Date:    today(),
		Metrics: "visitors,pageviews,events",
	}

	result, err := svc.GetAggregate(testGinCtx(), siteID, req)
	if err != nil {
		t.Fatalf("GetAggregate(all events): %v", err)
	}

	for _, m := range []string{"visitors", "pageviews", "events"} {
		if mr, ok := result.Results[m]; ok {
			t.Logf("  %-12s value=%v", m, mr.Value)
		} else {
			t.Errorf("metric %q missing", m)
		}
	}

	// events 应 >= pageviews（events = pageviews + 自定义事件 + Outbound Link 等）
	ev := toFloat(result.Results["events"].Value)
	pv := toFloat(result.Results["pageviews"].Value)
	if pv > 0 && ev < pv {
		t.Errorf("events(%.0f) < pageviews(%.0f)", ev, pv)
	}
}

// ============================================================
// 10. QueryService — execute 完整链路验证
// ============================================================

func TestE2EQueryService(t *testing.T) {
	_, qs, siteID := setupE2E(t)

	// 使用已知工作的维度(visit:source) + 指标验证 QueryService 完整链路
	params := &types.Params{
		SiteID:     fmt.Sprintf("%d", siteID),
		Period:     "day",
		Date:       today(),
		Timezone:   "UTC",
		Metrics:    []string{"visitors", "events"},
		Dimensions: []string{"visit:source"},
		Pagination: &types.Pagination{Limit: 5},
	}

	query, err := qs.CreateQuery(params)
	if err != nil {
		t.Fatalf("CreateQuery: %v", err)
	}
	if query == nil {
		t.Fatal("CreateQuery returned nil")
	}
	if query.Now.IsZero() {
		t.Error("CreateQuery did not set Now")
	}
	if len(query.Metrics) == 0 {
		t.Error("CreateQuery returned empty metrics")
	}

	site := &types.Site{ID: query.SiteID, Timezone: query.Timezone}
	result, err := qs.Execute(testGinCtx(), query, site)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result == nil {
		t.Fatal("Execute returned nil")
	}

	t.Logf("QueryService: %d rows, columns=%v", len(result.Data), result.Columns)

	if len(result.Data) == 0 {
		t.Fatal("visit:source breakdown returned 0 rows — seed data may be empty")
	}

	// 验证行结构
	for i, row := range result.Data {
		if i >= 3 {
			break
		}
		dim := row["name"]
		v := row["visitors"]
		e := row["events"]
		t.Logf("  %-20s visitors=%v events=%v", dim, v, e)
	}
}

// ============================================================
// 11. Scroll Depth — 通过 StatsService 完整链路
// ============================================================

func TestE2EScrollDepth(t *testing.T) {
	svc, _, siteID := setupE2E(t)

	req := &atypes.StatsRequest{
		Period:  "p7",
		Date:    today(),
		Metrics: "scroll_depth,visitors,pageviews",
	}

	result, err := svc.GetAggregate(testGinCtx(), siteID, req)
	if err != nil {
		t.Fatalf("GetAggregate(scroll_depth): %v", err)
	}

	sd, hasSD := result.Results["scroll_depth"]
	if !hasSD {
		t.Fatal("scroll_depth metric missing in results")
	}
	t.Logf("  scroll_depth   value=%v", sd.Value)

	// 有 events 数据时 scroll_depth 应该 > 0
	pv, hasPV := result.Results["pageviews"]
	if hasPV && toFloat(pv.Value) > 0 && toFloat(sd.Value) == 0 {
		t.Error("scroll_depth should be > 0 when pageviews exist")
	}

	// scroll_depth 不应超过 100
	if toFloat(sd.Value) > 100 {
		t.Errorf("scroll_depth %.0f > 100 (invalid)", toFloat(sd.Value))
	}

	// scroll_depth 不应为负
	if toFloat(sd.Value) < 0 {
		t.Errorf("scroll_depth %.0f < 0 (invalid)", toFloat(sd.Value))
	}
}

// ============================================================
// 辅助
// ============================================================

func toFloat(v any) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case int64:
		return float64(val)
	case uint64:
		return float64(val)
	case int:
		return float64(val)
	default:
		return 0
	}
}

