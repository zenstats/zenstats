package repository

import (
	"context"
	"fmt"
	"math"
	"os"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/google/uuid"
	"github.com/zenstats/zenstats/config"
	"github.com/zenstats/zenstats/internal/heatmap"
	cl "github.com/zenstats/zenstats/internal/store/clickhouse"
)

func TestScrollAggregationIntegration(t *testing.T) {
	if testing.Short() || os.Getenv("APP_ENV") != "test" {
		t.Skip("requires isolated APP_ENV=test ClickHouse")
	}
	t.Chdir(t.TempDir())
	if err := config.Load("test"); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	admin := cl.GetConnection()
	db := fmt.Sprintf("scroll_test_%d", time.Now().UnixNano())
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
	if err := conn.Exec(ctx, "SYSTEM STOP MERGES heatmap_scroll_reports"); err != nil {
		t.Fatal(err)
	}
	repo := NewHeatmap(conn)
	writer := NewScrollWriter(repo)
	now := time.Now().UTC().Truncate(time.Millisecond)
	from, to := now.Add(-time.Hour), now.Add(time.Hour)
	page := heatmap.Page{Hostname: "example.test", Path: "/soft/a.html", AggregationKey: "software", Version: "v1", Variant: "default", ViewportWidth: 1280, ViewportHeight: 720, DocumentWidth: 1280, DocumentHeight: 2000}
	r := heatmap.NewScrollRow(41, heatmap.ScrollReport{ID: uuid.New(), PageViewID: uuid.New(), StartedAt: now, Timestamp: now, Revision: 1, Page: page, MaxBottom: 500, MaxHeight: 2000}, heatmap.DefaultConfig(), .1)
	insert := func(rows ...heatmap.ScrollRow) {
		t.Helper()
		if err := writer.Insert(ctx, rows); err != nil {
			t.Fatal(err)
		}
	}
	get := func() heatmap.ScrollData {
		t.Helper()
		d, err := repo.ScrollData(ctx, 41, page, from, to, false)
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	insert(r, r)
	d := get()
	if d.SampledViews != 1 || d.ValidViews != 1 || len(d.Regions) != 80 || d.Regions[19].Views != 1 || d.Regions[20].Views != 0 {
		t.Fatalf("no-click baseline or retry: %+v", d)
	}
	later := r
	later.ID = uuid.New()
	later.Revision = 2
	later.MaxBottom = 1500
	insert(later, r, later)
	d = get()
	if d.ValidViews != 1 || d.AverageMaxDepth != 75 || d.Regions[59].Views != 1 || d.Regions[60].Views != 0 {
		t.Fatalf("newer revision: %+v", d)
	}
	// A larger document reduces normalized depth. Latest revision, not max(%), wins.
	grown := later
	grown.ID = uuid.New()
	grown.Revision = 3
	grown.MaxHeight = 2400
	insert(grown, later)
	d = get()
	if math.Abs(d.AverageMaxDepth-62.5) > .001 || d.Regions[49].Views != 1 || d.Regions[50].Views != 0 {
		t.Fatalf("growing page: %+v", d)
	}
	short := r
	short.ID = uuid.New()
	short.PageViewID = uuid.New()
	short.Page.DocumentHeight = 720
	short.MaxBottom = 720
	short.MaxHeight = 720
	insert(short, short)
	for i := range 3 {
		excluded := r
		excluded.ID = uuid.New()
		excluded.PageViewID = uuid.New()
		switch i {
		case 0:
			excluded.MaxHeight = 2600
		case 1:
			excluded.Clipped = true
		case 2:
			excluded.Unsupported = true
		}
		insert(excluded)
	}
	expired := r
	expired.ID = uuid.New()
	expired.PageViewID = uuid.New()
	expired.Expires = now.Add(-time.Hour)
	insert(expired)
	otherSite := r
	otherSite.SiteID = 42
	insert(otherSite)
	d = get()
	if d.SampledViews != 5 || d.ValidViews != 2 || d.DynamicViews != 1 || d.ClippedViews != 1 || d.Unsupported != 1 || d.Regions[79].Views != 1 || d.Regions[79].Rate != 50 {
		t.Fatalf("denominator, exclusions and site isolation: %+v", d)
	}
	pages, err := repo.ScrollPages(ctx, 41, from, to, HeatmapPageFilter{})
	if err != nil || len(pages) != 1 || pages[0].SampledViews != 5 {
		t.Fatalf("scroll-only layouts: %+v %v", pages, err)
	}
	otherPath := r
	otherPath.ID = uuid.New()
	otherPath.PageViewID = uuid.New()
	otherPath.Page.Path = "/soft/b.html"
	otherPath.SampleRate = .2
	insert(otherPath)
	group, err := repo.ScrollData(ctx, 41, page, from, to, true)
	if err != nil || group.ValidViews != 3 || !group.MixedSampling {
		t.Fatalf("template aggregation: %+v %v", group, err)
	}
	if get().ValidViews != 2 {
		t.Fatal("specific path mixed with template")
	}
	if err := conn.Exec(ctx, "SYSTEM START MERGES heatmap_scroll_reports"); err != nil {
		t.Fatal(err)
	}
	if err := conn.Exec(ctx, "OPTIMIZE TABLE heatmap_scroll_reports FINAL"); err != nil {
		t.Fatal(err)
	}
	if get().ValidViews != 2 {
		t.Fatal("background merges changed counts")
	}
	emptyPage := page
	emptyPage.Version = "missing"
	empty, err := repo.ScrollData(ctx, 41, emptyPage, from, to, false)
	if err != nil || empty.ValidViews != 0 || len(empty.Regions) != 80 || math.IsNaN(empty.AverageMaxDepth) {
		t.Fatalf("empty data: %+v %v", empty, err)
	}
}
