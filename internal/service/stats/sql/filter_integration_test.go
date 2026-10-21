package sql

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"testing"
	"time"

	"github.com/zenstats/zenstats/config"
	"github.com/zenstats/zenstats/internal/service/stats/types"
	"github.com/zenstats/zenstats/internal/session"
	cl "github.com/zenstats/zenstats/internal/store/clickhouse"
	"github.com/zenstats/zenstats/internal/store/clickhouse/models"
	"github.com/zenstats/zenstats/internal/store/clickhouse/repository"
	"github.com/zenstats/zenstats/internal/store/postgresql"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent/searchengines"
	"github.com/zenstats/zenstats/pkg/globals"
)

// Run against the isolated Compose databases: APP_ENV=test go test ./internal/service/stats/sql -run Integration -count=1.
func TestStatsFiltersIntegration(t *testing.T) {
	if testing.Short() || os.Getenv("APP_ENV") != "test" {
		t.Skip("requires APP_ENV=test and test databases")
	}
	t.Chdir(t.TempDir()) // Keep resolved config artifacts outside the repository.
	if err := config.Load("test"); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	db := postgresql.NewClient()
	defer db.Client.Close()
	if err := db.Client.Schema.Create(ctx); err != nil {
		t.Fatal(err)
	}
	previous := globals.GetDB()
	globals.SetDB(db)
	defer globals.SetDB(previous)
	if err := cl.NewMigration().Run(); err != nil {
		t.Fatal(err)
	}
	conn := cl.GetConnection()
	site, err := db.Client.Site.Create().SetDomain(fmt.Sprintf("filters-%d.example.test", time.Now().UnixNano())).SetTimezone("UTC").Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Client.Site.DeleteOne(site).Exec(ctx)
	sid := uint64(site.ID)
	if !db.Client.SearchEngines.Query().Where(searchengines.Domain("google.com")).ExistX(ctx) {
		engine, err := db.Client.SearchEngines.Create().SetDomain("google.com").SetName("Google").Save(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer db.Client.SearchEngines.DeleteOne(engine).Exec(ctx)
	}
	InvalidateSearchEngineCache()
	defer InvalidateSearchEngineCache()
	for _, name := range []string{"signup", "purchase"} {
		if _, err := db.Client.Goal.Create().SetSiteID(site.ID).SetDisplayName(name).SetEventName(name).Save(ctx); err != nil {
			t.Fatal(err)
		}
	}
	base := time.Now().UTC().Truncate(time.Second).Add(-10 * time.Minute)
	a := &models.Sessions{SiteId: sid, UserId: 101, SessionId: 101, Version: 1, Sign: 1, Start: base, Timestamp: base, EntryPage: "/landing", ExitPage: "/landing", PageViews: 1, Events: 1, IsBounce: 1, CountryCode: "US", ReferrerSource: "google.com", HostName: "example.test"}
	if err := conn.Exec(ctx, "SYSTEM STOP MERGES sessions"); err != nil {
		t.Fatal(err)
	}
	defer conn.Exec(ctx, "SYSTEM START MERGES sessions")
	if err := repository.GetSessionsRepository().BatchInsert(ctx, []*models.Sessions{a}); err != nil {
		t.Fatal(err)
	}
	tomb := *a
	tomb.Sign = -1
	latest := *a
	latest.Version = 2
	latest.Timestamp = base.Add(time.Minute)
	latest.ExitPage = "/checkout"
	latest.IsBounce = 0
	latest.Events = 2
	latest.Duration = 60
	b := &models.Sessions{SiteId: sid, UserId: 102, SessionId: 102, Version: 3, Sign: 1, Start: base, Timestamp: base.Add(time.Minute), EntryPage: "/other", ExitPage: "/other", PageViews: 1, Events: 2, IsBounce: 1, CountryCode: "US", HostName: "example.test"}
	c := &models.Sessions{SiteId: sid, UserId: 103, SessionId: 103, Version: 4, Sign: 1, Start: base, Timestamp: base.Add(time.Minute), EntryPage: "/landing", ExitPage: "/landing", PageViews: 1, Events: 2, IsBounce: 1, CountryCode: "US", HostName: "example.test"}
	if err := repository.GetSessionsRepository().BatchInsert(ctx, []*models.Sessions{&tomb, &latest, b, c}); err != nil {
		t.Fatal(err)
	}
	events := []*models.Events{
		{UserId: 101, SessionId: 101, Name: "pageview", PathName: "/landing", MetaKey: []string{"plan"}, MetaValue: []string{"pro"}, ReferrerSource: "google.com"},
		{UserId: 101, SessionId: 101, Name: "signup", PathName: "/docs/intro", MetaKey: []string{"plan"}, MetaValue: []string{"pro"}, ReferrerSource: "google.com"},
		{UserId: 102, SessionId: 102, Name: "pageview", PathName: "/other"},
		{UserId: 102, SessionId: 102, Name: "purchase", PathName: "/docs/next", MetaKey: []string{"plan"}, MetaValue: []string{"free"}},
		{UserId: 103, SessionId: 103, Name: "pageview", PathName: "/landing", MetaKey: []string{"tier"}, MetaValue: []string{"gold"}},
		{UserId: 103, SessionId: 103, Name: "signup", PathName: "/docs/third", MetaKey: []string{"plan"}, MetaValue: []string{"free"}},
	}
	for i, event := range events {
		event.SiteId = sid
		event.Timestamp = base.Add(time.Duration(i%2) * time.Minute)
		event.CountryCode = "US"
		event.HostName = "example.test"
	}
	if err := repository.GetEventsRepository().BatchInsert(ctx, events); err != nil {
		t.Fatal(err)
	}
	if err := repository.GetEventsRepository().BatchInsert(ctx, []*models.Events{{SiteId: sid, UserId: 101, SessionId: 101, Name: "historical", Timestamp: base.Add(-2 * time.Hour)}}); err != nil {
		t.Fatal(err)
	}
	filter := func(op, dim string, values ...any) *types.Filter {
		return &types.Filter{Operator: op, Dimension: dim, Values: values}
	}
	mixed := &types.Filter{Operator: "or", SubFilters: []*types.Filter{filter("is", "event:name", "signup"), filter("is", "visit:entry_page", "/other")}}
	cases := []struct {
		name, metric string
		filters      []*types.Filter
		want         float64
	}{
		{"event exclusion", "events", []*types.Filter{filter("is_not", "event:name", "signup")}, 4},
		{"wildcard", "events", []*types.Filter{filter("matches_wildcard", "event:page", "/docs/*")}, 3},
		{"property exists", "events", []*types.Filter{filter("is_not", "event:props:plan", "(none)")}, 4},
		{"property absent", "events", []*types.Filter{filter("is", "event:props:plan", "(none)")}, 2},
		{"property absent or pro", "events", []*types.Filter{filter("is", "event:props:plan", "(none)", "pro")}, 4},
		{"property excludes pro including absent", "events", []*types.Filter{filter("is_not", "event:props:plan", "pro")}, 4},
		{"direct source", "events", []*types.Filter{filter("is", "visit:source", "Direct / None")}, 4},
		{"classified source", "events", []*types.Filter{filter("is", "visit:source", "Google")}, 2},
		{"event-filtered bounce", "bounce_rate", []*types.Filter{filter("is", "event:name", "signup")}, 50},
		{"page alias session filter", "bounce_rate", []*types.Filter{filter("is", "visit:page", "/landing")}, 50},
		{"has done", "events", []*types.Filter{{Operator: "has_done", SubFilters: []*types.Filter{filter("is", "event:name", "signup")}}}, 4},
		{"has not done", "events", []*types.Filter{{Operator: "has_not_done", SubFilters: []*types.Filter{filter("is", "event:name", "signup")}}}, 2},
		{"has done respects time bounds", "events", []*types.Filter{{Operator: "has_done", SubFilters: []*types.Filter{filter("is", "event:name", "historical")}}}, 0},
		{"entry-filtered pageviews", "pageviews", []*types.Filter{filter("is", "visit:entry_page", "/landing")}, 2},
		{"mixed OR events", "events", []*types.Filter{mixed}, 4},
		{"mixed OR sessions", "bounce_rate", []*types.Filter{mixed}, 66.67},
		{"same matching event", "visitors", []*types.Filter{filter("is", "event:name", "pageview"), filter("is", "event:props:plan", "free")}, 0},
		{"same event for session metrics", "bounce_rate", []*types.Filter{filter("is", "event:name", "pageview"), filter("is", "event:props:plan", "free")}, 0},
		{"latest exit state", "visitors", []*types.Filter{filter("is", "visit:exit_page", "/landing")}, 1},
		{"multiple goals", "events", []*types.Filter{filter("is", "event:goal", "signup", "purchase")}, 3},
		{"excluded goal", "events", []*types.Filter{filter("is_not", "event:goal", "signup")}, 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q := &types.Query{SiteID: fmt.Sprint(site.ID), Timezone: "UTC", Metrics: []string{tc.metric}, Filters: tc.filters, Now: time.Now(), UTCTimeRange: types.TimeRange{Start: base.Add(-time.Minute), End: base.Add(5 * time.Minute)}}
			statement, args, err := NewQueryBuilder().Build(q, &types.Site{ID: q.SiteID, Timezone: "UTC"})
			if err != nil {
				t.Fatal(err)
			}
			var got float64
			if err := conn.QueryRow(ctx, "SELECT toFloat64("+tc.metric+") FROM ("+statement+")", args...).Scan(&got); err != nil {
				t.Fatalf("%v\nSQL=%s\nargs=%v", err, statement, args)
			}
			if math.Abs(got-tc.want) > .005 {
				t.Fatalf("got %v want %v\nSQL=%s\nargs=%v", got, tc.want, statement, args)
			}
		})
	}
	for _, tc := range []struct {
		name, metric, dimension, source string
		want                            float64
	}{
		{"grouped Google", "events", "visit:source", "Google", 2},
		{"grouped direct", "events", "visit:source", "Direct / None", 4},
		{"grouped session source", "bounce_rate", "visit:source", "Google", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q := &types.Query{SiteID: fmt.Sprint(site.ID), Timezone: "UTC", Metrics: []string{tc.metric}, Dimensions: []string{tc.dimension}, Filters: []*types.Filter{filter("is", "visit:source", tc.source)}, Now: time.Now(), UTCTimeRange: types.TimeRange{Start: base.Add(-time.Minute), End: base.Add(5 * time.Minute)}}
			statement, args, err := NewQueryBuilder().Build(q, &types.Site{ID: q.SiteID, Timezone: "UTC"})
			if err != nil {
				t.Fatal(err)
			}
			var got float64
			if err := conn.QueryRow(ctx, "SELECT sum(toFloat64("+tc.metric+")) FROM ("+statement+")", args...).Scan(&got); err != nil {
				t.Fatalf("%v\nSQL=%s", err, statement)
			}
			if math.Abs(got-tc.want) > .005 {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
	t.Run("session restore and inactivity", func(t *testing.T) {
		repo := repository.GetSessionsRepository()
		restored, err := repo.GetActiveSessionAt(ctx, 101, sid, base.Add(2*time.Minute), 30*time.Minute)
		if err != nil || restored.Version != 2 || restored.ExitPage != "/checkout" {
			t.Fatalf("restore=%+v err=%v", restored, err)
		}
		if _, err := repo.GetActiveSessionAt(ctx, 101, sid, base.Add(45*time.Minute), 30*time.Minute); !errors.Is(err, repository.ErrSessionNotFound) {
			t.Fatalf("expired session restored: %v", err)
		}
		sm := session.NewSessionManager(ctx, 1000)
		defer sm.Shutdown()
		start := time.Now().UTC().Truncate(time.Second).Add(-time.Hour)
		first, err := sm.OnEvent(&models.Events{UserId: 200, SiteId: sid, Name: "pageview", PathName: "/first", Timestamp: start})
		if err != nil {
			t.Fatal(err)
		}
		next, err := sm.OnEvent(&models.Events{UserId: 200, SiteId: sid, Name: "pageview", PathName: "/second", Timestamp: start.Add(31 * time.Minute)})
		if err != nil || first.SessionId == next.SessionId {
			t.Fatalf("31 minute gap did not create session: %v", err)
		}
		late, err := sm.OnEvent(&models.Events{UserId: 200, SiteId: sid, Name: "engagement", Timestamp: start.Add(time.Minute)})
		if err != nil || late.SessionId != first.SessionId || late.Duration != 60 {
			t.Fatalf("late engagement=%+v err=%v", late, err)
		}
	})

	// Re-reading after a real merge must preserve the selected latest state.
	if err := conn.Exec(ctx, "SYSTEM START MERGES sessions"); err != nil {
		t.Fatal(err)
	}
	if err := conn.Exec(ctx, "OPTIMIZE TABLE sessions FINAL"); err != nil {
		t.Fatal(err)
	}
	var count uint64
	if err := conn.QueryRow(ctx, "SELECT count() FROM sessions FINAL WHERE site_id = ? AND exit_page = '/landing' AND sign = 1", sid).Scan(&count); err != nil || count != 1 {
		t.Fatalf("post-merge state count=%d err=%v", count, err)
	}
}
