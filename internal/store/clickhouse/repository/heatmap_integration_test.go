package repository

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/google/uuid"
	"github.com/zenstats/zenstats/config"
	"github.com/zenstats/zenstats/internal/heatmap"
	cl "github.com/zenstats/zenstats/internal/store/clickhouse"
)

func TestHeatmapAggregationIntegration(t *testing.T) {
	if testing.Short() || os.Getenv("APP_ENV") != "test" {
		t.Skip("requires isolated APP_ENV=test ClickHouse")
	}
	t.Chdir(t.TempDir())
	if err := config.Load("test"); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	admin := cl.GetConnection()
	db := fmt.Sprintf("heatmap_test_%d", time.Now().UnixNano())
	if err := admin.Exec(ctx, "CREATE DATABASE "+db); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(ctx, "DROP DATABASE "+db+" SYNC")
	conn, err := clickhouse.Open(&clickhouse.Options{Addr: config.Conf.Clickhouse.Addr, Auth: clickhouse.Auth{Database: db, Username: config.Conf.Clickhouse.Username, Password: config.Conf.Clickhouse.Password}})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := cl.EnsureHeatmapTables(ctx, conn); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"heatmap_clicks", "heatmap_clicks_hourly"} {
		if err := conn.Exec(ctx, "SYSTEM STOP MERGES "+table); err != nil {
			t.Fatal(err)
		}
	}
	repo := NewHeatmap(conn)
	now := time.Now().UTC().Truncate(time.Second)
	page := heatmap.Page{Hostname: "example.test", Path: "/pricing", Version: "v1", Variant: "default", ViewportWidth: 1280, ViewportHeight: 720, DocumentWidth: 1280, DocumentHeight: 2000}
	click := heatmap.Click{ID: uuid.New(), Timestamp: now, Page: page, X: 400, Y: 600, Element: "signup", RelativeX: .5, RelativeY: .5, Position: "document"}
	row := heatmap.NewRow(41, uuid.New(), click, heatmap.DefaultConfig())
	insert := func(r heatmap.Row) {
		t.Helper()
		if err := repo.Insert(ctx, []heatmap.Row{r}); err != nil {
			t.Fatal(err)
		}
	}
	check := func(want uint64) {
		t.Helper()
		d, err := repo.Data(ctx, 41, page, now.Add(-time.Hour).Truncate(time.Hour), now.Add(time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		if d.Clicks != want {
			t.Fatalf("clicks=%d want=%d %+v", d.Clicks, want, d)
		}
	}
	insert(row)
	insert(row)
	check(1)
	var raw uint64
	if err := conn.QueryRow(ctx, "SELECT count() FROM heatmap_clicks").Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if raw != 2 {
		t.Fatalf("source retries must reach dependent view; rows=%d", raw)
	}
	// Fail a real dependent-view write; remove the fault and replay the same
	// browser click. Whether the source committed or rolled back, count stays exact.
	if err := conn.Exec(ctx, "ALTER TABLE heatmap_clicks_hourly ADD CONSTRAINT reject_test CHECK viewport_width=0"); err != nil {
		t.Fatal(err)
	}
	fault := row
	fault.ID = uuid.New()
	if err := repo.Insert(ctx, []heatmap.Row{fault}); err == nil {
		t.Fatal("expected dependent view failure")
	}
	if err := conn.Exec(ctx, "ALTER TABLE heatmap_clicks_hourly DROP CONSTRAINT reject_test"); err != nil {
		t.Fatal(err)
	}
	insert(fault)
	insert(fault)
	check(2)
	// Simulate an outage with source data present but no view processing, then
	// repair from retained raw data twice while existing aggregate states remain.
	if err := conn.Exec(ctx, "DROP VIEW heatmap_clicks_mv"); err != nil {
		t.Fatal(err)
	}
	missing := row
	missing.ID = uuid.New()
	missing.X = 800
	insert(missing)
	check(2)
	if err := cl.EnsureHeatmapTables(ctx, conn); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := repo.Replay(ctx, 41, now.Add(-time.Hour), now.Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	check(3)
	unmapped := row
	unmapped.ID = uuid.New()
	unmapped.Position = "fixed"
	insert(unmapped)
	other := row
	other.ID = uuid.New()
	other.SiteID = 42
	insert(other)
	d, err := repo.Data(ctx, 41, page, now.Add(-time.Hour).Truncate(time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if d.Clicks != 4 || d.Unmapped != 1 || len(d.Points) != 2 || len(d.Elements) != 1 || d.Elements[0].Count != 4 {
		t.Fatalf("bad mapped/element/site aggregation: %+v", d)
	}
	pages, err := repo.Pages(ctx, 41, now.Add(-time.Hour).Truncate(time.Hour), now.Add(time.Hour))
	if err != nil || len(pages) != 1 || pages[0].Clicks != 4 {
		t.Fatalf("bad pages: %+v %v", pages, err)
	}
	for _, table := range []string{"heatmap_clicks", "heatmap_clicks_hourly"} {
		if err := conn.Exec(ctx, "SYSTEM START MERGES "+table); err != nil {
			t.Fatal(err)
		}
		if err := conn.Exec(ctx, "OPTIMIZE TABLE "+table+" FINAL"); err != nil {
			t.Fatal(err)
		}
	}
	check(4)
	expired := row
	expired.ID = uuid.New()
	expired.RawExpires = now.Add(-time.Hour)
	expired.AggregateExpires = now.Add(-time.Hour)
	insert(expired)
	check(4) // Queries hide expired aggregates before asynchronous TTL cleanup.
	for _, table := range []string{"heatmap_clicks", "heatmap_clicks_hourly"} {
		if err := conn.Exec(ctx, "ALTER TABLE "+table+" MATERIALIZE TTL SETTINGS mutations_sync=1"); err != nil {
			t.Fatal(err)
		}
	}
	var expiredRaw uint64
	if err := conn.QueryRow(ctx, "SELECT count() FROM heatmap_clicks WHERE click_id=?", expired.ID).Scan(&expiredRaw); err != nil || expiredRaw != 0 {
		t.Fatal(expiredRaw, err)
	}
	var expiredStates uint64
	if err := conn.QueryRow(ctx, "SELECT count() FROM heatmap_clicks_hourly WHERE expires<=now()").Scan(&expiredStates); err != nil || expiredStates != 0 {
		t.Fatal(expiredStates, err)
	}
	check(4)

	// With a real view write failure, the optional collector remains bounded,
	// rejects overload, retries, drains on shutdown and reports every failed row.
	if err := conn.Exec(ctx, "ALTER TABLE heatmap_clicks_hourly ADD CONSTRAINT reject_collector CHECK viewport_width=0"); err != nil {
		t.Fatal(err)
	}
	collector := heatmap.NewCollector(repo)
	defer collector.Shutdown()
	batch := make([]heatmap.Row, heatmap.MaxBatchSize)
	for i := range batch {
		batch[i] = row
		batch[i].SiteID = 99
		batch[i].ID = uuid.New()
	}
	rejected := false
	for range 1000 {
		collector.Received(len(batch))
		if err := collector.Enqueue(batch); err != nil {
			rejected = true
			break
		}
	}
	collector.Shutdown()
	quality := collector.Snapshot()
	if !rejected || quality.Pending != 0 || quality.Accepted == 0 || quality.Persisted != 0 || quality.Failed != quality.Accepted || quality.Retries < 2 || quality.Received != quality.Accepted+quality.Rejected {
		t.Fatalf("bounded failure accounting: %+v", quality)
	}
	if err := conn.Exec(ctx, "ALTER TABLE heatmap_clicks_hourly DROP CONSTRAINT reject_collector"); err != nil {
		t.Fatal(err)
	}
}
