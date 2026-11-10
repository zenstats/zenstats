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
	legacyID := seedLegacyHeatmap(t, ctx, conn)
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
	if err := conn.QueryRow(ctx, "SELECT count() FROM heatmap_clicks WHERE site_id=41").Scan(&raw); err != nil {
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

	// Multiple URLs and document heights combine within a device class. Versions
	// and keys remain isolated even after background aggregate merges and migration reruns.
	groupPage := page
	groupPage.AggregationKey = "software-detail"
	for _, path := range []string{"/soft/a.html", "/soft/b.html"} {
		r := row
		r.SiteID = 75
		r.ID = uuid.New()
		r.Page = groupPage
		r.Page.Path = path
		insert(r)
		insert(r)
	}
	separate := row
	separate.SiteID = 75
	separate.Page = groupPage
	separate.Page.Path = "/soft/a.html"
	for i := 0; i < 3; i++ {
		r := separate
		r.ID = uuid.New()
		switch i {
		case 0:
			r.Page.AggregationKey = "other"
		case 1:
			r.Page.DocumentHeight++
			r.Y = 620
		case 2:
			r.Page.Version = "v2"
		}
		insert(r)
	}
	if err := cl.EnsureHeatmapTables(ctx, conn); err != nil {
		t.Fatal(err)
	}
	if err := conn.Exec(ctx, "OPTIMIZE TABLE heatmap_clicks_hourly FINAL"); err != nil {
		t.Fatal(err)
	}
	grouped, err := repo.Pages(ctx, 75, now.Add(-time.Hour).Truncate(time.Hour), now.Add(time.Hour), true)
	if err != nil || len(grouped) != 3 {
		t.Fatalf("groups=%+v err=%v", grouped, err)
	}

	filtered, err := repo.PagesMatching(ctx, 75, now.Add(-time.Hour).Truncate(time.Hour), now.Add(time.Hour), HeatmapPageFilter{ByKey: true, Key: "other"})
	if err != nil || len(filtered) != 1 || filtered[0].AggregationKey != "other" {
		t.Fatalf("key filter: %+v %v", filtered, err)
	}
	filtered, err = repo.PagesMatching(ctx, 75, now.Add(-time.Hour).Truncate(time.Hour), now.Add(time.Hour), HeatmapPageFilter{Path: "/soft/b.*"})
	if err != nil || len(filtered) != 1 || filtered[0].Path != "/soft/b.html" {
		t.Fatalf("path filter: %+v %v", filtered, err)
	}
	filtered, err = repo.PagesMatching(ctx, 75, now.Add(-time.Hour).Truncate(time.Hour), now.Add(time.Hour), HeatmapPageFilter{Path: "/soft/b"})
	if err != nil || len(filtered) != 0 {
		t.Fatalf("path must be anchored: %+v %v", filtered, err)
	}
	groupPage.Path = "/"
	d, err = repo.Data(ctx, 75, groupPage, now.Add(-time.Hour).Truncate(time.Hour), now.Add(time.Hour), true)
	if err != nil || d.Clicks != 3 || len(d.Points) != 1 {
		t.Fatalf("combined=%+v err=%v", d, err)
	}
	groupPage.Path = "/soft/a.html"
	d, err = repo.Data(ctx, 75, groupPage, now.Add(-time.Hour).Truncate(time.Hour), now.Add(time.Hour))
	if err != nil || d.Clicks != 2 {
		t.Fatalf("specific URL=%+v err=%v", d, err)
	}
	for range 2 {
		if err := repo.Replay(ctx, 75, now.Add(-time.Hour), now.Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	groupPage.Path = "/"
	d, err = repo.Data(ctx, 75, groupPage, now.Add(-time.Hour).Truncate(time.Hour), now.Add(time.Hour), true)
	if err != nil || d.Clicks != 3 {
		t.Fatalf("replay combined=%+v err=%v", d, err)
	}
	check(4) // Existing unlabelled historical data remains readable.

	// Different windows and page lengths land in the same percentage cell;
	// device breakpoints, edge clicks and positioning exclusions remain distinct.
	for _, width := range []uint16{390, 767, 768, 1023, 1024, 1280, 1440, 1920} {
		r := row
		r.SiteID, r.ID = 88, uuid.New()
		r.Page.ViewportWidth = width
		r.Page.ViewportHeight = 600 + width/10
		r.Page.DocumentWidth, r.Page.DocumentHeight = uint32(width), uint32(width)*3
		r.X, r.Y = float64(width)*.3375, float64(r.Page.DocumentHeight)*.34375
		insert(r)
		insert(r)
	}
	for range 2 {
		if err := repo.Replay(ctx, 88, now.Add(-time.Hour), now.Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	devices, err := repo.Pages(ctx, 88, now.Add(-time.Hour).Truncate(time.Hour), now.Add(time.Hour))
	if err != nil || len(devices) != 3 {
		t.Fatalf("device groups: %+v %v", devices, err)
	}
	for _, device := range devices {
		want := uint64(2)
		if device.Device() == "desktop" {
			want = 4
		}
		data, err := repo.Data(ctx, 88, device.Page, now.Add(-time.Hour).Truncate(time.Hour), now.Add(time.Hour))
		if err != nil || data.Clicks != want || len(data.Points) != 1 || data.Points[0].Count != want || data.Points[0].X != 13 || data.Points[0].Y != 27 || data.CoordinateSpace != "normalized" || data.GridColumns != 40 || data.GridRows != 80 {
			t.Fatalf("percentage projection: device=%s data=%+v err=%v", device.Device(), data, err)
		}
	}
	var layouts uint64
	if err := conn.QueryRow(ctx, `SELECT uniqExact(tuple(viewport_width,viewport_height,document_width,document_height)) FROM heatmap_clicks_hourly WHERE site_id=88`).Scan(&layouts); err != nil || layouts != 3 {
		t.Fatalf("stored dimension cardinality=%d err=%v", layouts, err)
	}
	edge := row
	edge.SiteID, edge.ID = 88, uuid.New()
	edge.X, edge.Y = float64(edge.Page.DocumentWidth)-.01, float64(edge.Page.DocumentHeight)-.01
	insert(edge)
	data, err := repo.Data(ctx, 88, page, now.Add(-time.Hour).Truncate(time.Hour), now.Add(time.Hour))
	if err != nil || len(data.Points) != 2 {
		t.Fatalf("edge bins: %+v %v", data, err)
	}
	foundEdge := false
	for _, point := range data.Points {
		if point.X == 39 && point.Y == 79 {
			foundEdge = true
		}
	}
	if !foundEdge {
		t.Fatalf("edge outside grid: %+v", data)
	}

	// Cover every fine cell, including the new upper half of each axis. This
	// catches a stale 800-point query limit as well as accidental coarse bins.
	finePage := page
	finePage.DocumentWidth, finePage.DocumentHeight = 1280, 2560
	fineRows := make([]heatmap.Row, 0, 40*80)
	for y := range 80 {
		for x := range 40 {
			r := row
			r.SiteID, r.ID, r.Page = 90, uuid.New(), finePage
			r.X, r.Y = float64(x*32+16), float64(y*32+16)
			fineRows = append(fineRows, r)
		}
	}
	for range 2 {
		if err := repo.Insert(ctx, fineRows); err != nil {
			t.Fatal(err)
		}
	}
	fullGrid, err := repo.Data(ctx, 90, finePage, now.Add(-time.Hour).Truncate(time.Hour), now.Add(time.Hour))
	if err != nil || fullGrid.Clicks != 3200 || len(fullGrid.Points) != 3200 || fullGrid.Truncated || fullGrid.GridColumns != 40 || fullGrid.GridRows != 80 {
		t.Fatalf("full fine grid: clicks=%d points=%d truncated=%v grid=%dx%d err=%v", fullGrid.Clicks, len(fullGrid.Points), fullGrid.Truncated, fullGrid.GridColumns, fullGrid.GridRows, err)
	}
	for _, p := range fullGrid.Points {
		if p.X < 0 || p.X >= 40 || p.Y < 0 || p.Y >= 80 || p.Count != 1 {
			t.Fatalf("fine cell bounds or retry count: %+v", p)
		}
	}

	// Pre-upgrade data has no grid_version column. Repeated migration and replay
	// preserve its original cell, then queries merge it with the new region grid.
	legacyPage := page
	legacyPage.Path = "/legacy"
	for range 2 {
		if err := cl.EnsureHeatmapTables(ctx, conn); err != nil {
			t.Fatal(err)
		}
		if err := repo.Replay(ctx, 89, now.Add(-time.Hour), now.Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	legacyNew := row
	legacyNew.SiteID, legacyNew.ID, legacyNew.Page = 89, uuid.New(), legacyPage
	legacyNew.X, legacyNew.Y = 416, 599
	insert(legacyNew)
	legacyRetry := legacyNew
	legacyRetry.ID = legacyID
	insert(legacyRetry)
	for range 2 {
		if err := repo.Replay(ctx, 89, now.Add(-time.Hour), now.Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	legacyData, err := repo.Data(ctx, 89, legacyPage, now.Add(-time.Hour).Truncate(time.Hour), now.Add(time.Hour))
	if err != nil || legacyData.Clicks != 2 || len(legacyData.Points) != 1 || legacyData.Points[0].Count != 2 || legacyData.Points[0].X != 13 || legacyData.Points[0].Y != 24 {
		t.Fatalf("legacy migration and replay: %+v %v", legacyData, err)
	}

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

// Construct the previous deployed schema, and insert a row before migration.
func seedLegacyHeatmap(t *testing.T, ctx context.Context, conn clickhouse.Conn) uuid.UUID {
	id := uuid.New()
	t.Helper()
	queries := []string{
		`CREATE TABLE IF NOT EXISTS heatmap_clicks (
			site_id UInt64, click_id UUID, batch_id UUID,
			timestamp DateTime64(3, 'UTC'), bucket DateTime('UTC'),
			hostname LowCardinality(String), path String, version LowCardinality(String), variant LowCardinality(String),
			viewport_width UInt16, viewport_height UInt16, document_width UInt32, document_height UInt32,
			x Float32, y Float32, element LowCardinality(String), relative_x Float32, relative_y Float32,
			position LowCardinality(String), sample_rate Float64,
			raw_expires DateTime('UTC'), aggregate_expires DateTime('UTC')
		) ENGINE = ReplacingMergeTree
		PARTITION BY toYYYYMMDD(timestamp)
		ORDER BY (site_id, hostname, path, version, variant, viewport_width, viewport_height, document_width, document_height, bucket, click_id)
		TTL raw_expires DELETE
		SETTINGS index_granularity = 8192, non_replicated_deduplication_window = 0`,
		`CREATE TABLE IF NOT EXISTS heatmap_clicks_hourly (
			site_id UInt64, bucket DateTime('UTC'),
			hostname LowCardinality(String), path String, version LowCardinality(String), variant LowCardinality(String),
			viewport_width UInt16, viewport_height UInt16, document_width UInt32, document_height UInt32,
			grid_x Int32, grid_y Int32, element LowCardinality(String), sample_rate Float64,
			clicks AggregateFunction(uniqExact, UUID), last_click SimpleAggregateFunction(max, DateTime64(3, 'UTC')),
			expires DateTime('UTC')
		) ENGINE = AggregatingMergeTree
		PARTITION BY toYYYYMM(bucket)
		ORDER BY (site_id, hostname, path, version, variant, viewport_width, viewport_height, document_width, document_height, bucket, grid_x, grid_y, element, sample_rate, expires)
		TTL expires DELETE
		SETTINGS index_granularity = 8192`,
		`ALTER TABLE heatmap_clicks ADD COLUMN IF NOT EXISTS aggregation_key LowCardinality(String),
            MODIFY ORDER BY (site_id,hostname,path,version,variant,viewport_width,viewport_height,document_width,document_height,bucket,click_id,aggregation_key)`,
		`ALTER TABLE heatmap_clicks_hourly ADD COLUMN IF NOT EXISTS aggregation_key LowCardinality(String),
            MODIFY ORDER BY (site_id,hostname,path,version,variant,viewport_width,viewport_height,document_width,document_height,bucket,grid_x,grid_y,element,sample_rate,expires,aggregation_key)`,
		`CREATE MATERIALIZED VIEW IF NOT EXISTS heatmap_clicks_mv TO heatmap_clicks_hourly AS SELECT site_id, bucket, hostname, path, version, variant, aggregation_key,
			viewport_width, viewport_height, document_width, document_height,
			if(position = 'document', intDiv(toInt32(x), 16) * 16, -1) AS grid_x,
			if(position = 'document', intDiv(toInt32(y), 16) * 16, -1) AS grid_y,
			element, sample_rate, uniqExactState(click_id) AS clicks, max(timestamp) AS last_click, aggregate_expires AS expires
		FROM heatmap_clicks
		GROUP BY site_id, bucket, hostname, path, version, variant, aggregation_key,
			viewport_width, viewport_height, document_width, document_height, grid_x, grid_y, element, sample_rate, expires`,
	}
	for _, query := range queries {
		if err := conn.Exec(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	if err := conn.Exec(ctx, `INSERT INTO heatmap_clicks
	(site_id,click_id,batch_id,timestamp,bucket,hostname,path,version,variant,viewport_width,viewport_height,document_width,document_height,x,y,element,relative_x,relative_y,position,sample_rate,raw_expires,aggregate_expires)
	VALUES (89,?,generateUUIDv4(),now(),toStartOfHour(now()),'example.test','/legacy','v1','default',1280,720,1280,2000,416,599,'signup',0.5,0.5,'document',0.1,now()+INTERVAL 7 DAY,now()+INTERVAL 90 DAY)`, id); err != nil {
		t.Fatal(err)
	}
	return id
}
