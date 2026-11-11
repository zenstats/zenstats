package repository

import (
	"context"
	"fmt"
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/google/uuid"
	"github.com/zenstats/zenstats/config"
	p "github.com/zenstats/zenstats/internal/performance"
	cl "github.com/zenstats/zenstats/internal/store/clickhouse"
	"os"
	"testing"
	"time"
)

func TestPerformanceAggregationIntegration(t *testing.T) {
	if testing.Short() || os.Getenv("APP_ENV") != "test" {
		t.Skip("requires isolated APP_ENV=test ClickHouse")
	}
	t.Chdir(t.TempDir())
	if err := config.Load("test"); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	admin := cl.GetConnection()
	db := fmt.Sprintf("performance_test_%d", time.Now().UnixNano())
	if err := admin.Exec(ctx, "CREATE DATABASE "+db); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(ctx, "DROP DATABASE "+db+" SYNC")
	conn, err := clickhouse.Open(&clickhouse.Options{Addr: config.Conf.Clickhouse.Addr, Auth: clickhouse.Auth{Database: db, Username: config.Conf.Clickhouse.Username, Password: config.Conf.Clickhouse.Password}})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := cl.EnsurePerformanceTables(ctx, conn); err != nil {
		t.Fatal(err)
	}
	if err := conn.Exec(ctx, "SYSTEM STOP MERGES web_vitals_reports"); err != nil {
		t.Fatal(err)
	}
	repo := NewPerformance(NewHeatmap(conn))
	now := time.Now().UTC().Truncate(time.Millisecond)
	from, to := now.Add(-48*time.Hour), now.Add(time.Hour)
	makeContext := func(index int) p.Row {
		started := now.Add(-25 * time.Hour)
		if index >= 90 {
			started = now.Add(-time.Hour)
		}
		return p.Row{Report: p.Report{ID: uuid.New(), LifecycleID: uuid.New(), Kind: "context", Revision: 1, NavigationStartedAt: started, ReportedAt: now, Hostname: "example.test", Path: "/pricing", Release: "v1", NavigationType: "navigate", Supported: p.Names}, SiteID: 41, Device: "desktop", Browser: "Chrome", Country: "CN", SampleRate: .1, ConfigVersion: 1, Expires: now.Add(time.Hour)}
	}
	makeMetric := func(c p.Row, name string, value float64) p.Row {
		c.Kind = "metric"
		c.MetricName = name
		c.MetricID = uuid.NewString()
		c.ID = uuid.New()
		c.Supported = nil
		c.Value = &value
		return c
	}
	insert := func(rows ...p.Row) {
		t.Helper()
		if err := repo.Insert(ctx, rows); err != nil {
			t.Fatal(err)
		}
	}
	get := func(f p.Filter) p.Data {
		t.Helper()
		d, err := repo.Data(ctx, 41, from, to, f, "Asia/Kathmandu")
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	filter := p.Filter{Metric: "LCP"}
	c := makeContext(0)
	lcp := makeMetric(c, "LCP", 9000)
	insert(c, lcp, c, lcp)
	lcp.ID = uuid.New()
	lcp.Revision = 2
	v := 1000.
	lcp.Value = &v
	insert(lcp)
	d := get(filter)
	if d.Navigations != 1 || d.Metrics[0].Samples != 1 || *d.Metrics[0].P75 != 1000 || d.Metrics[1].P75 != nil || d.Metrics[1].Good != 0 {
		t.Fatalf("retries, revisions and missing INP: %+v", d)
	}
	rows := []p.Row{makeMetric(c, "CLS", 0), makeMetric(c, "INP", 100), makeMetric(c, "FCP", 1000), makeMetric(c, "TTFB", 100)}
	for i := 1; i < 100; i++ {
		c := makeContext(i)
		value := 1000.
		if i >= 90 {
			value = 5000
			c.SampleRate = .2
			c.ConfigVersion = 2
		}
		if i == 1 {
			c.InitDelayMS = 3000
		}
		rows = append(rows, c, makeMetric(c, "LCP", value), makeMetric(c, "CLS", 0), makeMetric(c, "FCP", 1000), makeMetric(c, "TTFB", 100))
		if i < 90 {
			rows = append(rows, makeMetric(c, "INP", 100))
		}
	}
	for offset := 0; offset < len(rows); offset += 100 {
		end := min(offset+100, len(rows))
		insert(rows[offset:end]...)
	}
	background := makeContext(100)
	background.BackgroundInit = true
	insert(background, makeMetric(background, "LCP", 99999))
	orphan := makeContext(100)
	insert(makeMetric(orphan, "LCP", 99999))
	expired := makeContext(100)
	expired.Expires = now.Add(-time.Hour)
	insert(expired, makeMetric(expired, "LCP", 99999))
	other := makeContext(100)
	other.SiteID = 42
	insert(other, makeMetric(other, "LCP", 99999))
	d = get(filter)
	if d.Navigations != 101 || d.Eligible != 100 || d.BackgroundInit != 1 || d.LateInit != 1 || !d.MixedSampling || !d.MixedConfig || d.Overall != "insufficient" {
		t.Fatalf("context denominator and quality: %+v", d)
	}
	if d.Metrics[0].Samples != 100 || *d.Metrics[0].P75 != 1000 || d.Metrics[0].Good != 90 || d.Metrics[0].Poor != 10 || d.Metrics[1].Samples != 90 || d.Metrics[1].Missing != 10 || *d.Metrics[2].P75 != 0 {
		t.Fatalf("weighted P75, missing INP and zero CLS: %+v", d.Metrics)
	}
	if len(d.Trend) != 2 || len(d.Pages) != 1 || d.Pages[0].Samples != 100 {
		t.Fatal(d.Trend, d.Pages)
	}
	filter.Device = "mobile"
	if get(filter).Navigations != 0 {
		t.Fatal("device filter failed")
	}
	filter.Device = ""
	filter.Path = "/other/*"
	if get(filter).Navigations != 0 {
		t.Fatal("path filter failed")
	}
	filter.Path = ""
	if err := conn.Exec(ctx, "SYSTEM START MERGES web_vitals_reports"); err != nil {
		t.Fatal(err)
	}
	if err := conn.Exec(ctx, "OPTIMIZE TABLE web_vitals_reports FINAL"); err != nil {
		t.Fatal(err)
	}
	merged := get(filter)
	if merged.Navigations != d.Navigations || merged.Metrics[0].Samples != 100 || *merged.Metrics[0].P75 != 1000 {
		t.Fatal("merge changed samples")
	}
}
