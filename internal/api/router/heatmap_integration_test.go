package router

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zenstats/zenstats/config"
	"github.com/zenstats/zenstats/internal/auth"
	"github.com/zenstats/zenstats/internal/heatmap"
	p "github.com/zenstats/zenstats/internal/performance"
	"github.com/zenstats/zenstats/internal/service"
	cl "github.com/zenstats/zenstats/internal/store/clickhouse"
	"github.com/zenstats/zenstats/internal/store/clickhouse/repository"
	"github.com/zenstats/zenstats/internal/store/postgresql"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent/sitemembership"
	"github.com/zenstats/zenstats/pkg/globals"
)

// Uses real, disposable PostgreSQL and ClickHouse databases, including actual
// middleware, file storage, queue draining and incremental aggregate queries.
func TestHeatmapAPIIntegration(t *testing.T) {
	if testing.Short() || os.Getenv("APP_ENV") != "test" {
		t.Skip("requires isolated APP_ENV=test databases")
	}
	t.Chdir(t.TempDir())
	if err := config.Load("test"); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	admin := postgresql.NewClient()
	defer admin.Client.Close()
	database := fmt.Sprintf("heatmap_api_%d", time.Now().UnixNano())
	if _, err := admin.DB.ExecContext(ctx, "CREATE DATABASE "+database); err != nil {
		t.Fatal(err)
	}
	defer admin.DB.ExecContext(ctx, "DROP DATABASE "+database+" WITH (FORCE)")
	config.Conf.Database.Database = database
	db := postgresql.NewClient()
	defer db.Client.Close()
	if err := db.Client.Schema.Create(ctx); err != nil {
		t.Fatal(err)
	}
	globals.SetDB(db)
	chAdmin := cl.GetConnection()
	if err := chAdmin.Exec(ctx, "CREATE DATABASE "+database); err != nil {
		t.Fatal(err)
	}
	defer chAdmin.Exec(ctx, "DROP DATABASE "+database+" SYNC")
	conn, err := clickhouse.Open(&clickhouse.Options{Addr: config.Conf.Clickhouse.Addr, Auth: clickhouse.Auth{Database: database, Username: config.Conf.Clickhouse.Username, Password: config.Conf.Clickhouse.Password}, MaxOpenConns: 2})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := cl.EnsureHeatmapTables(ctx, conn); err != nil {
		t.Fatal(err)
	}
	if err := cl.EnsurePerformanceTables(ctx, conn); err != nil {
		t.Fatal(err)
	}
	storage := t.TempDir()
	s := service.NewHeatmapService(db, repository.NewHeatmap(conn), storage)
	defer s.Shutdown()
	performanceService := service.NewPerformanceService(db, s)
	defer performanceService.Shutdown()
	owner, err := db.Client.User.Create().SetEmail("owner@example.test").SetIsAdmin(true).Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	stranger, err := db.Client.User.Create().SetEmail("stranger@example.test").Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	site, err := db.Client.Site.Create().SetDomain("example.test").SetTimezone("Asia/Kathmandu").SetIsVerified(true).Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate an existing installation with a non-empty legacy config table.
	if _, err := db.Client.HeatmapConfig.Create().SetSiteID(site.ID).SetTimezone("Asia/Kathmandu").Save(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.ExecContext(ctx, "ALTER TABLE heatmap_configs DROP COLUMN groups"); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := db.Client.Schema.Create(ctx); err != nil {
			t.Fatal(err)
		}
	}
	legacy, err := db.Client.HeatmapConfig.Query().Only(ctx)
	if err != nil || legacy.Groups == nil || len(legacy.Groups) != 0 {
		t.Fatalf("legacy config migration: %+v %v", legacy, err)
	}

	if _, err := db.Client.SiteMembership.Create().SetSiteID(site.ID).SetUserID(owner.ID).SetRole(sitemembership.RoleOwner).Save(ctx); err != nil {
		t.Fatal(err)
	}
	other, err := db.Client.Site.Create().SetDomain("other.test").SetIsVerified(true).Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Client.SiteMembership.Create().SetSiteID(other.ID).SetUserID(stranger.ID).Save(ctx); err != nil {
		t.Fatal(err)
	}
	viewer, err := db.Client.User.Create().SetEmail("viewer@example.test").Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Client.SiteMembership.Create().SetSiteID(site.ID).SetUserID(viewer.ID).SetRole(sitemembership.RoleViewer).Save(ctx); err != nil {
		t.Fatal(err)
	}
	viewerToken, _ := auth.GenerateToken(viewer.ID)
	token, _ := auth.GenerateToken(owner.ID)
	otherToken, _ := auth.GenerateToken(stranger.ID)
	subToken, _ := auth.GenerateSubAccountToken(1, owner.ID, "viewer", nil)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(gin.Recovery())
	registerHeatmapRouterWithService(r.Group("/api"), s)
	registerPerformanceRouterWithService(r.Group("/api"), performanceService)
	call := func(method, url, token, contentType string, body []byte) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, url, bytes.NewReader(body))
		req.RemoteAddr = "203.0.113.10:1234"
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if strings.HasPrefix(url, "/api/heatmap/clicks") || strings.HasPrefix(url, "/api/heatmap/scroll") {
			req.Header.Set("Origin", "https://example.test")
		}
		if strings.HasPrefix(url, "/api/performance/reports") {
			req.RemoteAddr = "127.0.0.1:1234"
			req.Header.Set("Origin", "https://example.test")
		}
		res := httptest.NewRecorder()
		r.ServeHTTP(res, req)
		return res
	}
	encode := func(value any) []byte {
		b, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	code := func(res *httptest.ResponseRecorder) int {
		t.Helper()
		var value struct {
			Code int `json:"code"`
		}
		if err := json.Unmarshal(res.Body.Bytes(), &value); err != nil {
			t.Fatal(res.Body.String(), err)
		}
		return value.Code
	}
	base := "/api/sites/example.test/heatmap"
	cfg := heatmap.DefaultConfig()
	page := heatmap.Page{Hostname: "example.test", Path: "/pricing", Version: "v1", Variant: "default", ViewportWidth: 800, ViewportHeight: 600, DocumentWidth: 800, DocumentHeight: 1600}
	click := heatmap.Click{ID: uuid.New(), Timestamp: time.Now().UTC(), Page: page, X: 200, Y: 1000, Element: "signup", RelativeX: .5, RelativeY: .5, Position: "document"}
	batch := heatmap.Batch{Version: 1, Domain: site.Domain, ID: uuid.New(), SampleRate: 1, Clicks: []heatmap.Click{click}}
	t.Run("default off and authorization", func(t *testing.T) {
		res := call("GET", "/api/heatmap/config?domain=example.test", "", "", nil)
		var value struct{ Enabled bool }
		if err := json.Unmarshal(res.Body.Bytes(), &value); err != nil || value.Enabled {
			t.Fatal(res.Body.String())
		}
		if res := call("POST", "/api/heatmap/clicks", "", "text/plain", encode(batch)); res.Code != 403 {
			t.Fatal(res.Code, res.Body.String())
		}
		if got := code(call("GET", base+"/config", "", "", nil)); got != 401 {
			t.Fatal(got)
		}
		if got := code(call("GET", base+"/config", otherToken, "", nil)); got != 404 {
			t.Fatal(got)
		}
		if got := code(call("GET", base+"/config", subToken, "", nil)); got != 200 {
			t.Fatal(got)
		}
		cfg.Enabled = true
		cfg.Paths = []string{"/pricing"}
		cfg.SampleRate = 1
		if got := code(call("PUT", base+"/config", subToken, "application/json", encode(cfg))); got != 403 {
			t.Fatal(got)
		}
		if got := code(call("PUT", base+"/config", viewerToken, "application/json", encode(cfg))); got != 403 {
			t.Fatal(got)
		}
		res = call("PUT", base+"/config", token, "application/json", encode(cfg))
		if code(res) != 200 {
			t.Fatal(res.Body.String())
		}
		saved, err := s.Config(ctx, site.ID)
		if err != nil || saved.Timezone != "Asia/Kathmandu" {
			t.Fatal(saved, err)
		}
		if _, err := db.Client.Site.UpdateOneID(site.ID).SetTimezone("UTC").Save(ctx); err != nil {
			t.Fatal(err)
		}
		saved, err = s.SaveConfig(ctx, site.ID, cfg)
		if err != nil || saved.Timezone != "Asia/Kathmandu" {
			t.Fatal(saved, err)
		}
		from, to, err := s.DateRange(ctx, site.ID, "2026-11-09", "2026-11-09")
		if err != nil || from.Format(time.RFC3339) != "2026-11-08T18:15:00Z" || to.Sub(from) != 24*time.Hour {
			t.Fatal(from, to, err)
		}
	})
	t.Run("scroll snapshots authorization retention and independent transport", func(t *testing.T) {
		now := time.Now().UTC()
		report := heatmap.ScrollReport{ID: uuid.New(), PageViewID: uuid.New(), StartedAt: now, Timestamp: now, Revision: 1, Page: page, MaxBottom: 600, MaxHeight: 1600}
		batch := heatmap.ScrollBatch{Version: 1, Domain: site.Domain, ID: uuid.New(), SampleRate: 1, Reports: []heatmap.ScrollReport{report}}
		post := func(b heatmap.ScrollBatch) *httptest.ResponseRecorder {
			return call("POST", "/api/heatmap/scroll", "", "text/plain", encode(b))
		}
		if got := post(batch).Code; got != 403 {
			t.Fatal(got)
		}
		cfg.ScrollEnabled = true
		if got := code(call("PUT", base+"/config", viewerToken, "application/json", encode(cfg))); got != 403 {
			t.Fatal(got)
		}
		if got := code(call("PUT", base+"/config", token, "application/json", encode(cfg))); got != 200 {
			t.Fatal(got)
		}
		saved, err := s.Config(ctx, site.ID)
		if err != nil || !saved.ScrollEnabled || !saved.ClicksEnabled {
			t.Fatal(saved, err)
		}
		var capability struct {
			ScrollEnabled bool `json:"scroll_enabled"`
			ScrollVersion int  `json:"scroll_protocol_version"`
		}
		res := call("GET", "/api/heatmap/config?domain=example.test", "", "", nil)
		if err := json.Unmarshal(res.Body.Bytes(), &capability); err != nil || !capability.ScrollEnabled || capability.ScrollVersion != 1 {
			t.Fatal(res.Body.String(), err)
		}
		invalid := batch
		invalid.Reports = append([]heatmap.ScrollReport{}, report, report)
		invalid.Reports[1].ID = uuid.New()
		invalid.Reports[1].Revision = 0
		if got := post(invalid).Code; got != 400 {
			t.Fatal(got)
		}
		invalid.Reports = append([]heatmap.ScrollReport{}, report)
		invalid.Reports[0].Page.Hostname = "unrelated.test"
		if got := post(invalid).Code; got != 403 {
			t.Fatal(got)
		}
		request := httptest.NewRequest("POST", "/api/heatmap/scroll", bytes.NewReader(encode(batch)))
		request.Header.Set("Origin", "https://unrelated.test")
		response := httptest.NewRecorder()
		r.ServeHTTP(response, request)
		if response.Code != 403 {
			t.Fatal(response.Code)
		}
		for range 2 {
			if got := post(batch).Code; got != 202 {
				t.Fatal(got)
			}
		}
		batch.Reports[0].ID = uuid.New()
		batch.Reports[0].Revision = 2
		batch.Reports[0].MaxBottom = 1200
		if got := post(batch).Code; got != 202 {
			t.Fatal(got)
		}
		s.ScrollCollector.Shutdown()
		quality := s.ScrollCollector.Snapshot()
		if quality.Accepted != 3 || quality.Persisted != 3 || quality.Pending != 0 || quality.Failed != 0 {
			t.Fatalf("%+v", quality)
		}
		loc, _ := time.LoadLocation(saved.Timezone)
		today := now.In(loc).Format("2006-01-02")
		query := "?start=" + today + "&end=" + today
		for _, suffix := range []string{"/scroll/pages" + query, "/scroll/data" + query + "&page=" + url.QueryEscape(string(encode(page)))} {
			if got := code(call("GET", base+suffix, otherToken, "", nil)); got != 404 {
				t.Fatal(got)
			}
			if got := code(call("GET", base+suffix, "", "", nil)); got != 401 {
				t.Fatal(got)
			}
			if got := code(call("GET", base+suffix, subToken, "", nil)); got != 200 {
				t.Fatal(got)
			}
		}
		res = call("GET", base+"/scroll/data"+query+"&page="+url.QueryEscape(string(encode(page))), viewerToken, "", nil)
		var result struct {
			Data heatmap.ScrollData `json:"data"`
		}
		if err := json.Unmarshal(res.Body.Bytes(), &result); err != nil || result.Data.SampledViews != 1 || result.Data.ValidViews != 1 || result.Data.AverageMaxDepth != 75 || len(result.Data.Regions) != 80 {
			t.Fatal(res.Body.String(), err)
		}
		old := now.In(loc).AddDate(0, 0, -8).Format("2006-01-02")
		if got := code(call("GET", base+"/scroll/pages?start="+old+"&end="+today, token, "", nil)); got != 400 {
			t.Fatal(got)
		}
		if got := code(call("GET", "/api/admin/scroll-quality", viewerToken, "", nil)); got != 403 {
			t.Fatal(got)
		}
		if got := code(call("GET", "/api/admin/scroll-quality", token, "", nil)); got != 200 {
			t.Fatal(got)
		}
		if res := post(batch); res.Code != 503 || res.Header().Get("Retry-After") == "" {
			t.Fatal(res.Code, res.Header())
		}
	})
	t.Run("performance collection, missing metrics and permissions", func(t *testing.T) {
		base := "/api/sites/example.test/performance"
		now := time.Now().UTC()
		value := 9000.
		contextReport := p.Report{ID: uuid.New(), LifecycleID: uuid.New(), Kind: "context", Revision: 1, NavigationStartedAt: now, ReportedAt: now, Hostname: "example.test", Path: "/pricing", Release: "v1", NavigationType: "navigate", Supported: p.Names}
		metric := contextReport
		metric.Kind = "metric"
		metric.ID = uuid.New()
		metric.MetricName = "LCP"
		metric.MetricID = "v6-test"
		metric.Value = &value
		metric.Supported = nil
		batch := p.Batch{Version: 1, Domain: site.Domain, ID: uuid.New(), SampleRate: 1, ConfigVersion: 1, Reports: []p.Report{contextReport, metric}}
		post := func(b p.Batch) *httptest.ResponseRecorder {
			return call("POST", "/api/performance/reports", "", "text/plain", encode(b))
		}
		if got := post(batch).Code; got != 403 {
			t.Fatal(got)
		}
		if got := code(call("GET", base+"/config", "", "", nil)); got != 401 {
			t.Fatal(got)
		}
		if got := code(call("GET", base+"/config", otherToken, "", nil)); got != 404 {
			t.Fatal(got)
		}
		cfg := p.DefaultConfig()
		cfg.Enabled = true
		cfg.Paths = []string{"/pricing"}
		cfg.SampleRate = 1
		for _, tok := range []string{viewerToken, subToken} {
			if got := code(call("PUT", base+"/config", tok, "application/json", encode(cfg))); got != 403 {
				t.Fatal(got)
			}
		}
		res := call("PUT", base+"/config", token, "application/json", encode(cfg))
		if code(res) != 200 {
			t.Fatal(res.Body.String())
		}
		saved, err := performanceService.Config(ctx, site.ID)
		if err != nil || saved.Version != 1 || !saved.Enabled {
			t.Fatal(saved, err)
		}
		var capability struct {
			Enabled bool   `json:"enabled"`
			Version uint64 `json:"config_version"`
		}
		res = call("GET", "/api/performance/config?domain=example.test", "", "", nil)
		if err := json.Unmarshal(res.Body.Bytes(), &capability); err != nil || !capability.Enabled || capability.Version != 1 {
			t.Fatal(res.Body.String(), err)
		}
		invalid := batch
		invalid.Reports = append([]p.Report{}, contextReport, metric)
		invalid.Reports[1].MetricName = "FID"
		if got := post(invalid).Code; got != 400 {
			t.Fatal(got)
		}
		invalid.Reports[1].MetricName = "LCP"
		invalid.Reports[1].Hostname = "unrelated.test"
		if got := post(invalid).Code; got != 403 {
			t.Fatal(got)
		}
		req := httptest.NewRequest("POST", "/api/performance/reports", bytes.NewReader(encode(batch)))
		req.Header.Set("Origin", "https://unrelated.test")
		recorder := httptest.NewRecorder()
		r.ServeHTTP(recorder, req)
		if recorder.Code != 403 {
			t.Fatal(recorder.Code)
		}
		if res := call("POST", "/api/performance/reports", "", "text/plain", bytes.Repeat([]byte(" "), 17<<10)); res.Code != 413 {
			t.Fatal(res.Code)
		}
		for range 2 {
			if got := post(batch).Code; got != 202 {
				t.Fatal(got)
			}
		}
		value = 1500.
		metric.Value = &value
		metric.ID = uuid.New()
		metric.Revision = 2
		batch.Reports = []p.Report{metric}
		if got := post(batch).Code; got != 202 {
			t.Fatal(got)
		}
		performanceService.Collector.Shutdown()
		quality := performanceService.Collector.Snapshot()
		if quality.Accepted != 5 || quality.Persisted != 5 || quality.Pending != 0 {
			t.Fatal(quality)
		}
		loc, _ := time.LoadLocation(saved.Timezone)
		today := now.In(loc).Format("2006-01-02")
		query := "/data?start=" + today + "&end=" + today
		if got := code(call("GET", base+query, otherToken, "", nil)); got != 404 {
			t.Fatal(got)
		}
		res = call("GET", base+query, subToken, "", nil)
		var result struct {
			Data p.Data `json:"data"`
		}
		if code(res) != 200 {
			t.Fatal(res.Body.String())
		}
		if err := json.Unmarshal(res.Body.Bytes(), &result); err != nil || result.Data.Navigations != 1 || result.Data.Metrics[0].Samples != 1 || result.Data.Metrics[0].P75 == nil || *result.Data.Metrics[0].P75 != 1500 || result.Data.Metrics[1].P75 != nil || result.Data.Metrics[1].Good != 0 {
			t.Fatal(res.Body.String(), err)
		}
		old := now.In(loc).AddDate(0, 0, -8).Format("2006-01-02")
		if got := code(call("GET", base+"/data?start="+old+"&end="+today, token, "", nil)); got != 400 {
			t.Fatal(got)
		}
		if got := code(call("GET", "/api/admin/performance-quality", viewerToken, "", nil)); got != 403 {
			t.Fatal(got)
		}
		if got := code(call("GET", "/api/admin/performance-quality", token, "", nil)); got != 200 {
			t.Fatal(got)
		}
		if res := post(batch); res.Code != 503 || res.Header().Get("Retry-After") == "" {
			t.Fatal(res.Code, res.Header())
		}
	})
	t.Run("atomic validation and no event session pipeline", func(t *testing.T) {
		invalid := batch
		invalid.Clicks = append([]heatmap.Click{}, click, click)
		invalid.Clicks[1].ID = uuid.New()
		invalid.Clicks[1].Page.Path = "/pricing?secret=value"
		if res := call("POST", "/api/heatmap/clicks", "", "text/plain", encode(invalid)); res.Code != 400 {
			t.Fatal(res.Code, res.Body.String())
		}
		invalid = batch
		invalid.Clicks = append([]heatmap.Click{}, click)
		invalid.Clicks[0].Page.Hostname = "unrelated.test"
		if res := call("POST", "/api/heatmap/clicks", "", "text/plain", encode(invalid)); res.Code != 403 {
			t.Fatal(res.Code, res.Body.String())
		}
		if res := call("POST", "/api/heatmap/clicks", "", "text/plain", []byte(`{"unknown":true}`)); res.Code != 400 {
			t.Fatal(res.Code)
		}
		if res := call("POST", "/api/heatmap/clicks", "", "text/plain", bytes.Repeat([]byte(" "), 17<<10)); res.Code != 413 {
			t.Fatal(res.Code)
		}
		for i := 0; i < 2; i++ {
			if res := call("POST", "/api/heatmap/clicks", "", "text/plain", encode(batch)); res.Code != 202 || res.Header().Get("X-Zenstats-Accepted") != "1" {
				t.Fatal(res.Code, res.Body.String())
			}
		}
		// No ordinary work queue is initialized: heatmap must be self-contained.
		s.Collector.Shutdown()
		q := s.Collector.Snapshot()
		if q.Accepted != 2 || q.Persisted != 2 || q.Pending != 0 || q.Failed != 0 {
			t.Fatalf("%+v", q)
		}
		from := click.Timestamp.Add(-24 * time.Hour)
		to := click.Timestamp.Add(24 * time.Hour)
		result, err := s.Repo.Data(ctx, uint64(site.ID), page, from, to)
		if err != nil || result.Clicks != 1 || len(result.Points) != 1 || len(result.Elements) != 1 {
			t.Fatal(result, err)
		}
		otherResult, err := s.Repo.Data(ctx, uint64(other.ID), page, from, to)
		if err != nil || otherResult.Clicks != 0 {
			t.Fatal(otherResult, err)
		}
		if got := code(call("GET", "/api/admin/heatmap-quality", otherToken, "", nil)); got != 403 {
			t.Fatal(got)
		}
		if got := code(call("GET", "/api/admin/heatmap-quality", token, "", nil)); got != 200 {
			t.Fatal(got)
		}
		if res := call("POST", "/api/heatmap/clicks", "", "text/plain", encode(batch)); res.Code != 503 || res.Header().Get("Retry-After") == "" {
			t.Fatal(res.Code, res.Body.String())
		}
	})
	t.Run("automatic capture ownership permission and layout checks", func(t *testing.T) {
		cfg.Groups = []heatmap.GroupRule{{Path: "/soft/*.html", Key: "software", ExampleURL: "https://example.test/soft/a.html"}}
		if _, err := s.SaveConfig(ctx, site.ID, cfg); err != nil {
			t.Fatal(err)
		}
		badCfg := cfg
		badCfg.Groups = []heatmap.GroupRule{{Path: "/soft/*", Key: "software", ExampleURL: "https://other.test/soft/a"}}
		if res := call("PUT", base+"/config", token, "application/json", encode(badCfg)); res.Code != 400 {
			t.Fatal(res.Code, res.Body.String())
		}
		var picture bytes.Buffer
		png.Encode(&picture, image.NewRGBA(image.Rect(0, 0, 400, 800)))
		actual := page
		actual.Path = "/soft/a.html"
		workerCalls := 0
		wrongDevice, wrongVersion, wrongVariant, wrongRedirect := false, false, false, false
		worker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			workerCalls++
			if r.Header.Get("Authorization") != "Bearer internal-test-token" {
				t.Error("missing internal authentication")
			}
			var request struct {
				ViewportWidth  uint16 `json:"viewport_width"`
				ViewportHeight uint16 `json:"viewport_height"`
			}
			json.NewDecoder(r.Body).Decode(&request)
			if request.ViewportWidth != 768 || request.ViewportHeight != 1024 {
				t.Errorf("nonstandard viewport: %+v", request)
			}
			current := actual
			if wrongDevice {
				current.ViewportWidth = 390
			}
			if wrongVersion {
				current.Version = "old"
			}
			if wrongVariant {
				current.Variant = "experiment"
			}
			final := "https://example.test/soft/a.html"
			if wrongRedirect {
				final = "https://other.test/soft/a.html"
			}
			json.NewEncoder(w).Encode(map[string]any{"url": final, "page": current, "image": base64.StdEncoding.EncodeToString(picture.Bytes())})
		}))
		defer worker.Close()
		config.Conf.Heatmap.ScreenshotURL = worker.URL
		config.Conf.Heatmap.ScreenshotToken = "internal-test-token"
		defer func() { config.Conf.Heatmap.ScreenshotURL = ""; config.Conf.Heatmap.ScreenshotToken = "" }()
		selected := page
		selected.Path = "/"
		selected.AggregationKey = "software"
		selected = selected.Representative()
		payload := map[string]any{"url": "https://example.test/soft/a.html", "scope": "key", "page": selected}
		for _, tok := range []string{subToken, viewerToken} {
			if res := call("POST", base+"/capture", tok, "application/json", encode(payload)); code(res) != 403 {
				t.Fatal(res.Code, res.Body.String())
			}
		}
		for _, url := range []string{"https://other.test/soft/a.html", "file:///etc/passwd", "https://example.test/soft/a.html?token=private", "https://example.test/#/soft/a.html?token=private"} {
			payload["url"] = url
			if res := call("POST", base+"/capture", token, "application/json", encode(payload)); res.Code != 400 {
				t.Fatal(res.Code, res.Body.String())
			}
		}
		if workerCalls != 0 {
			t.Fatal("invalid requests reached browser")
		}
		payload["url"] = "https://example.test/soft/a.html"
		for _, fault := range []*bool{&wrongDevice, &wrongVersion, &wrongVariant, &wrongRedirect} {
			*fault = true
			if res := call("POST", base+"/capture", token, "application/json", encode(payload)); res.Code != 400 {
				t.Fatal(res.Code, res.Body.String())
			}
			*fault = false
		}

		urlPage := actual
		urlPage.AggregationKey = "old-key"
		payload["scope"] = "url"
		payload["page"] = urlPage
		if res := call("POST", base+"/capture", token, "application/json", encode(payload)); res.Code != 400 {
			t.Fatal(res.Code, res.Body.String())
		}
		payload["scope"] = "key"
		payload["page"] = selected
		res := call("POST", base+"/capture", token, "application/json", encode(payload))
		if res.Code != 200 {
			t.Fatal(res.Code, res.Body.String())
		}
		var saved struct{ Data service.HeatmapSnapshot }
		json.Unmarshal(res.Body.Bytes(), &saved)
		if saved.Data.AggregationKey != "software" || saved.Data.Path != "/soft/a.html" || saved.Data.SourceURL != "https://example.test/soft/a.html" {
			t.Fatal(saved.Data)
		}
		if err := s.DeleteSnapshot(ctx, site.ID, saved.Data.ID); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("snapshots matching permissions and cleanup", func(t *testing.T) {
		var body bytes.Buffer
		form := multipart.NewWriter(&body)
		if err := form.WriteField("page", string(encode(page.Representative()))); err != nil {
			t.Fatal(err)
		}
		imageField, err := form.CreateFormFile("image", "page.png")
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(imageField, image.NewRGBA(image.Rect(0, 0, 400, 800))); err != nil {
			t.Fatal(err)
		}
		if err := form.Close(); err != nil {
			t.Fatal(err)
		}
		res := call("POST", base+"/snapshots", subToken, form.FormDataContentType(), body.Bytes())
		if code(res) != 403 {
			t.Fatal(res.Body.String())
		}
		res = call("POST", base+"/snapshots", token, form.FormDataContentType(), body.Bytes())
		if code(res) != 200 {
			t.Fatal(res.Code, res.Body.String())
		}
		var saved struct{ Data service.HeatmapSnapshot }
		if err := json.Unmarshal(res.Body.Bytes(), &saved); err != nil {
			t.Fatal(err)
		}
		if saved.Data.ImageWidth != 400 || saved.Data.DocumentWidth != 768 || saved.Data.DocumentHeight != 1536 {
			t.Fatal(saved.Data)
		}
		imageURL := fmt.Sprintf("%s/snapshots/%d/image", base, saved.Data.ID)
		if res := call("GET", imageURL, token, "", nil); res.Code != 200 || res.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatal(res.Code)
		}
		if got := code(call("GET", imageURL, otherToken, "", nil)); got != 404 {
			t.Fatal(got)
		}
		if res := call("GET", fmt.Sprintf("/api/sites/other.test/heatmap/snapshots/%d/image", saved.Data.ID), otherToken, "", nil); res.Code != 404 {
			t.Fatal(res.Code, res.Body.String())
		}
		if res := call("GET", imageURL, subToken, "", nil); res.Code != 200 {
			t.Fatal(res.Code, res.Body.String())
		}
		badPage := page
		badPage.Hostname = "unrelated.test"
		if _, err := s.CreateSnapshot(ctx, site.ID, badPage, bytes.NewReader(nil)); err == nil {
			t.Fatal("unrelated snapshot accepted")
		}
		var wrong bytes.Buffer
		png.Encode(&wrong, image.NewRGBA(image.Rect(0, 0, 400, 400)))
		if _, err := s.CreateSnapshot(ctx, site.ID, page, &wrong, "https://example.test/pricing"); err == nil {
			t.Fatal("wrong aspect ratio accepted")
		}
		entries, err := os.ReadDir(filepath.Join(storage, "heatmaps"))
		if err != nil || len(entries) != 1 {
			t.Fatal(entries, err)
		}
		deleteContext, _ := gin.CreateTestContext(httptest.NewRecorder())
		if err := service.GetSiteService().DeleteSite(deleteContext, int(site.ID)); err != nil {
			t.Fatal(err)
		}
		old := time.Now().Add(-2 * time.Hour)
		for _, entry := range entries {
			p := filepath.Join(storage, "heatmaps", entry.Name())
			if err := os.Chtimes(p, old, old); err != nil {
				t.Fatal(err)
			}
		}
		if err := s.CleanupSnapshots(ctx); err != nil {
			t.Fatal(err)
		}
		entries, err = os.ReadDir(filepath.Join(storage, "heatmaps"))
		if err != nil || len(entries) != 0 {
			t.Fatal(entries, err)
		}
	})
}
