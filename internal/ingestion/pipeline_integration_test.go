package ingestion_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/config"
	"github.com/zenstats/zenstats/internal/api/external"
	"github.com/zenstats/zenstats/internal/api/router"
	"github.com/zenstats/zenstats/internal/auth"
	"github.com/zenstats/zenstats/internal/event"
	"github.com/zenstats/zenstats/internal/ingestion"
	"github.com/zenstats/zenstats/internal/model"
	"github.com/zenstats/zenstats/internal/service"
	"github.com/zenstats/zenstats/internal/session"
	cl "github.com/zenstats/zenstats/internal/store/clickhouse"
	"github.com/zenstats/zenstats/internal/store/clickhouse/models"
	"github.com/zenstats/zenstats/internal/store/postgresql"
	"github.com/zenstats/zenstats/pkg/generic"
	"github.com/zenstats/zenstats/pkg/globals"
)

// This test uses the isolated PostgreSQL/ClickHouse containers. No repositories,
// authorization checks or writers are mocked.
func TestCollectionQualityPipelineIntegration(t *testing.T) {
	if testing.Short() || os.Getenv("APP_ENV") != "test" {
		t.Skip("requires APP_ENV=test and isolated test databases")
	}
	geoFixture, err := os.ReadFile("../../data/GeoLite2-City.mmdb.test")
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	if err := config.Load("test"); err != nil {
		t.Fatal(err)
	}
	// Use the repository's real MaxMind fixture; never download during a test.
	if err := os.MkdirAll(config.Conf.DataPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(config.Conf.DataPath, "GeoLite2-City.mmdb"), geoFixture, 0600); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	db := postgresql.NewClient()
	defer db.Client.Close()
	if err := db.Client.Schema.Create(ctx); err != nil {
		t.Fatal(err)
	}
	globals.SetDB(db)
	defer globals.SetDB(nil)
	if err := cl.NewMigration().Run(); err != nil {
		t.Fatal(err)
	}
	conn := cl.GetConnection()
	defer conn.Close()
	name := fmt.Sprintf("quality-%d", time.Now().UnixNano())
	owner := db.Client.User.Create().SetEmail(name + "@example.test").SetIsAdmin(true).SaveX(ctx)
	defer db.Client.User.DeleteOne(owner).Exec(ctx)
	group := db.Client.UserGroup.Create().SetName(name).SetMaxMonthlyEvents(100).SaveX(ctx)
	defer db.Client.UserGroup.DeleteOne(group).Exec(ctx)
	userConfig := db.Client.UserConfig.Create().SetUserID(owner.ID).SetGroupID(group.ID).SaveX(ctx)
	defer db.Client.UserConfig.DeleteOne(userConfig).Exec(ctx)
	site := db.Client.Site.Create().SetDomain(name + ".example.test").SetTimezone("UTC").SetIsVerified(true).SaveX(ctx)
	defer db.Client.Site.DeleteOne(site).Exec(ctx)
	db.Client.SiteMembership.Create().SetSiteID(site.ID).SetUserID(owner.ID).SaveX(ctx)
	defer conn.Exec(ctx, "ALTER TABLE events DELETE WHERE site_id = ? SETTINGS mutations_sync = 1", site.ID)
	defer conn.Exec(ctx, "ALTER TABLE sessions DELETE WHERE site_id = ? SETTINGS mutations_sync = 1", site.ID)
	queue := generic.NewQueue[*model.EventRequest](1, 2)
	globals.SetQueue(queue)
	defer globals.SetQueue(nil)
	m := ingestion.Default
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/event", external.Event(service.GetSiteService()))
	router.RegisterAdminRouter(r.Group("/api"))
	post := func(body, ua, ipType string) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/api/event", strings.NewReader(body))
		req.Header.Set("User-Agent", ua)
		req.Header.Set("X-Zenstats-Ip-Type", ipType)
		r.ServeHTTP(w, req)
		return w
	}
	valid := fmt.Sprintf(`{"n":"pageview","d":%q,"u":%q}`, site.Domain, "https://"+site.Domain+"/test")
	browser := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
	quota := event.GetMonthlyQuota()
	defer quota.Stop()
	t.Run("full batch rejects atomically and rolls back quota", func(t *testing.T) {
		if err := queue.Enqueue(&model.EventRequest{}); err != nil {
			t.Fatal(err)
		}
		w := post(`{"n":"batch","e":[`+valid+`,`+valid+`]}`, browser, "")
		if w.Code != 503 || w.Header().Get("Retry-After") != "5" || w.Header().Get("X-Zenstats-Accepted") != "0" || queue.Size() != 1 || quota.Get(owner.ID) != 0 || m.Snapshot().Pending.Events != 0 {
			t.Fatalf("partial admission or quota charged: status=%d headers=%v size=%d quota=%d", w.Code, w.Header(), queue.Size(), quota.Get(owner.ID))
		}
		queue.Dequeue()
	})
	t.Run("mixed batch and filtered requests", func(t *testing.T) {
		w := post(`{"n":"batch","e":[`+valid+`,{}]}`, browser, "")
		if w.Code != 202 || w.Header().Get("X-Zenstats-Accepted") != "1" || w.Header().Get("X-Zenstats-Rejected") != "1" || quota.Get(owner.ID) != 1 {
			t.Fatalf("mixed response: %d %v", w.Code, w.Header())
		}
		w = post(valid, "Googlebot/2.1 (+http://www.google.com/bot.html)", "")
		if w.Code != 202 {
			t.Fatal(w.Code, w.Body.String())
		}
		w = post(`{"n":"batch","e":[`+valid+`,`+valid+`]}`, browser, "dc_ip")
		if w.Code != 202 || w.Header().Get("X-Zenstats-Filtered") != "2" {
			t.Fatal(w.Code, w.Header())
		}
		s := m.Snapshot()
		if s.Counters["accepted"] != 2 || s.Counters["persisted"] != 0 || s.Pending.Events != 2 || s.Counters["filtered"] != 2 {
			t.Fatalf("202 counted as persisted: %+v", s.Counters)
		}
	})
	t.Run("unknown and unverified sites are client rejections", func(t *testing.T) {
		w := post(strings.ReplaceAll(valid, site.Domain, "unknown-"+site.Domain), browser, "")
		if w.Code != 400 {
			t.Fatalf("unknown site response=%d", w.Code)
		}
		unverified := db.Client.Site.Create().SetDomain("unverified-" + site.Domain).SetTimezone("UTC").SaveX(ctx)
		defer db.Client.Site.DeleteOne(unverified).Exec(ctx)
		w = post(strings.ReplaceAll(valid, site.Domain, unverified.Domain), browser, "")
		if w.Code != 400 {
			t.Fatalf("unverified site response=%d", w.Code)
		}
	})
	worker, err := event.NewEventWork(queue, 100, 0)
	if err != nil {
		t.Fatal(err)
	}
	worker.Run()
	worker.Shutdown()
	t.Run("confirmed event count matches ClickHouse", func(t *testing.T) {
		s := m.Snapshot()
		var count uint64
		if err := conn.QueryRow(ctx, "SELECT count() FROM events WHERE site_id = ?", site.ID).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 || s.Counters["persisted"] != count || s.Counters["filtered"] != 3 || s.Counters["failed"] != 0 || s.Pending.Events != 0 {
			t.Fatalf("database=%d counters=%+v pending=%+v reasons=%+v", count, s.Counters, s.Pending, s.Reasons)
		}
		if s.Writers["events"].WrittenRows != 1 || s.Writers["sessions"].WrittenRows != 1 {
			t.Fatalf("writer accounting: %+v", s.Writers)
		}
	})
	t.Run("bad event row fails the whole batch and counts retries once", func(t *testing.T) {
		wb := event.NewWriteBuffer(ctx, 100, time.Hour)
		wb.Start()
		var records []*ingestion.Record
		_ = m.Accept(2, func(r []*ingestion.Record) error { records = r; return nil })
		for i, country := range []string{"US", "invalid-country"} {
			wb.Add(&models.Events{Timestamp: time.Now(), SiteId: uint64(site.ID), Name: "pageview", CountryCode: country, Collection: records[i]})
		}
		wb.Shutdown()
		s := m.Snapshot()
		w := s.Writers["events"]
		var count uint64
		if err := conn.QueryRow(ctx, "SELECT count() FROM events WHERE site_id = ?", site.ID).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 || s.Counters["persisted"] != 1 || s.Counters["failed"] != 2 || s.Pending.Events != 0 || w.FailedRows != 2 || w.FailedAttempts != 5 || w.RetryAttempts != 4 || w.FailedBatches != 1 || w.QueuedRows != 0 || w.InflightRows != 0 {
			t.Fatalf("partial success or retry overcount: database=%d counters=%+v writer=%+v", count, s.Counters, w)
		}
	})
	t.Run("bad session state cannot leave a lone cancellation", func(t *testing.T) {
		wb := session.NewWriteBuffer(ctx, 100, time.Hour)
		wb.Start()
		now := time.Now()
		if err := wb.AddMany(
			&models.Sessions{Start: now, Timestamp: now, SiteId: uint64(site.ID), SessionId: 999, Version: 99, Sign: -1, CountryCode: "US"},
			&models.Sessions{Start: now, Timestamp: now, SiteId: uint64(site.ID), SessionId: 999, Version: 100, Sign: 1, CountryCode: "invalid-country"},
		); err != nil {
			t.Fatal(err)
		}
		wb.Shutdown()
		w := m.Snapshot().Writers["sessions"]
		var count uint64
		if err := conn.QueryRow(ctx, "SELECT count() FROM sessions WHERE site_id = ? AND session_id = 999", site.ID).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 || w.FailedRows != 2 || w.FailedAttempts != 3 || w.RetryAttempts != 2 || w.FailedBatches != 1 || w.QueuedRows != 0 || w.InflightRows != 0 {
			t.Fatalf("partial session pair or bad metrics: count=%d writer=%+v", count, w)
		}
	})
	t.Run("administrator access only", func(t *testing.T) {
		regular := db.Client.User.Create().SetEmail("regular-" + owner.Email).SaveX(ctx)
		defer db.Client.User.DeleteOne(regular).Exec(ctx)
		adminToken, _ := auth.GenerateToken(owner.ID)
		regularToken, _ := auth.GenerateToken(regular.ID)
		refreshToken, _ := auth.GenerateRefreshToken(owner.ID)
		subToken, _ := auth.GenerateSubAccountToken(123, owner.ID, "admin", nil)
		for _, tc := range []struct {
			token string
			code  int
		}{{"", 401}, {regularToken, 403}, {refreshToken, 401}, {subToken, 403}, {adminToken, 200}} {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/api/admin/ingestion-quality", nil)
			if tc.token != "" {
				req.Header.Set("Authorization", "Bearer "+tc.token)
			}
			r.ServeHTTP(w, req)
			var body struct {
				Code int                `json:"code"`
				Data ingestion.Snapshot `json:"data"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Code != tc.code {
				t.Fatalf("expected %d, got %d", tc.code, body.Code)
			}
			if tc.code == 200 && (body.Data.InstanceID != m.Snapshot().InstanceID || len(body.Data.History) != 60 || w.Header().Get("Cache-Control") != "no-store") {
				t.Fatal("invalid monitor response")
			}
		}
	})
}
