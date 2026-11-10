package funnel

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/zenstats/zenstats/config"
)

// Use inline fixtures against real ClickHouse without modifying stored events.
func TestFunnelSingleScanIntegration(t *testing.T) {
	if testing.Short() || os.Getenv("APP_ENV") != "test" {
		t.Skip("requires APP_ENV=test and isolated test ClickHouse")
	}
	t.Chdir(t.TempDir())
	if err := config.Load("test"); err != nil {
		t.Fatal(err)
	}
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: config.Conf.Clickhouse.Addr,
		Auth: clickhouse.Auth{Database: config.Conf.Clickhouse.Database, Username: config.Conf.Clickhouse.Username, Password: config.Conf.Clickhouse.Password},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	start := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	svc := &AnalysisService{conn: conn}
	run := func(t *testing.T, req *AnalysisRequest, source string) ([]uint64, uint64) {
		t.Helper()
		query, args := svc.buildFunnelQuery(req)
		query = strings.Replace(query, "zenstats_events_db.events", source, 1)
		var readRows atomic.Uint64
		ctx := clickhouse.Context(context.Background(), clickhouse.WithProgress(func(p *clickhouse.Progress) {
			readRows.Add(p.Rows)
		}))
		rows, err := conn.Query(ctx, query, args...)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var counts []uint64
		for rows.Next() {
			var order uint8
			var visitors uint64
			if err := rows.Scan(&order, &visitors); err != nil {
				t.Fatal(err)
			}
			if int(order) != len(counts)+1 {
				t.Fatalf("unexpected step order %d", order)
			}
			counts = append(counts, visitors)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		return counts, readRows.Load()
	}
	t.Run("eight steps scan events once", func(t *testing.T) {
		req := &AnalysisRequest{SiteID: "1", StartTime: start, EndTime: start.Add(time.Hour)}
		for i := range 8 {
			step := &FunnelStep{StepID: int64(i + 1), StepOrder: i + 1, GoalType: "event", GoalValue: fmt.Sprintf("event%d", i)}
			if i > 0 {
				parent := int64(i)
				step.ParentStepID = &parent
			}
			req.Steps = append(req.Steps, step)
		}
		source := `(SELECT toUInt64(1) AS site_id, intDiv(number,8) AS user_id,
			toDateTime('2026-11-01 00:00:00','UTC') + number%8 AS timestamp,
			concat('event',toString(number%8)) AS name FROM numbers(80000))`
		counts, reads := run(t, req, source)
		if !reflect.DeepEqual(counts, []uint64{10000, 10000, 10000, 10000, 10000, 10000, 10000, 10000}) {
			t.Fatalf("unexpected counts: %v", counts)
		}
		if reads == 0 || reads > 80000 {
			t.Fatalf("expected one scan of 80000 events, read %d", reads)
		}
		t.Logf("eight steps read %d events", reads)
	})
	t.Run("branches strict timestamps properties and empty results", func(t *testing.T) {
		root, download := int64(1), int64(2)
		req := &AnalysisRequest{SiteID: "1", StartTime: start, EndTime: start.Add(time.Hour), Steps: []*FunnelStep{
			{StepID: 1, StepOrder: 1, GoalType: "page", GoalValue: "/soft/*.html"},
			{StepID: 2, StepOrder: 2, ParentStepID: &root, GoalType: "event", GoalValue: "download", CustomProps: map[string]string{"plan": "paid"}},
			{StepID: 3, StepOrder: 3, ParentStepID: &root, GoalType: "event", GoalValue: "subscribe"},
			{StepID: 4, StepOrder: 4, ParentStepID: &download, GoalType: "event", GoalValue: "done"},
		}}
		source := `values('site_id UInt64, user_id UInt64, timestamp DateTime, name String, pathname String, "meta.key" Array(String), "meta.value" Array(String)',
			(1,1,'2026-11-01 00:00:00','pageview','/soft/a.html',[],[]),
			(1,1,'2026-11-01 00:01:00','download','',['plan'],['paid']),
			(1,1,'2026-11-01 00:02:00','subscribe','',[],[]),
			(1,1,'2026-11-01 00:03:00','done','',[],[]),
			(1,2,'2026-11-01 00:01:00','pageview','/soft/a.html',[],[]),
			(1,2,'2026-11-01 00:00:00','download','',['plan'],['paid']),
			(1,2,'2026-11-01 00:01:00','subscribe','',[],[]),
			(1,3,'2026-11-01 00:02:00','download','',['plan'],['paid']),
			(1,3,'2026-11-01 00:03:00','done','',[],[]),
			(1,4,'2026-11-01 00:00:00','pageview','/soft/b.html',[],[]),
			(1,4,'2026-11-01 00:00:00','download','',['plan'],['paid']),
			(1,4,'2026-11-01 00:01:00','subscribe','',[],[]),
			(1,5,'2026-11-01 00:00:00','pageview','/soft/a.html',[],[]),
			(1,5,'2026-11-01 00:01:00','download','',['plan'],['free']),
			(1,5,'2026-11-01 00:02:00','done','',[],[]),
			(2,6,'2026-11-01 00:00:00','pageview','/soft/a.html',[],[]),
			(1,7,'2026-10-31 23:59:59','pageview','/soft/a.html',[],[]),
			(1,7,'2026-11-01 00:01:00','download','',['plan'],['paid']),
			(1,8,'2026-11-01 00:00:00','download','/soft/a.html',['plan'],['paid']))`
		counts, _ := run(t, req, source)
		if !reflect.DeepEqual(counts, []uint64{4, 1, 2, 1}) {
			t.Fatalf("unexpected branch counts: %v", counts)
		}
		req.SiteID = "999"
		counts, _ = run(t, req, source)
		if !reflect.DeepEqual(counts, []uint64{0, 0, 0, 0}) {
			t.Fatalf("empty site must return every step with zero visitors: %v", counts)
		}
	})
}
