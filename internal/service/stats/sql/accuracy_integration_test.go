package sql

import (
	"context"
	"fmt"
	"math"
	"os"
	"testing"
	"time"

	"github.com/zenstats/zenstats/config"
	"github.com/zenstats/zenstats/internal/service/stats/types"
	cl "github.com/zenstats/zenstats/internal/store/clickhouse"
	"github.com/zenstats/zenstats/internal/store/clickhouse/models"
	"github.com/zenstats/zenstats/internal/store/clickhouse/repository"
	"github.com/zenstats/zenstats/internal/store/postgresql"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent/searchengines"
	"github.com/zenstats/zenstats/pkg/globals"
)

func TestTrafficAccuracyIntegration(t *testing.T) {
	if testing.Short() || os.Getenv("APP_ENV") != "test" {
		t.Skip("requires APP_ENV=test and isolated test databases")
	}
	t.Chdir(t.TempDir())
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
	gen := NewFragmentGenerator()
	toSQL := func(fragment SQLFragment) string { return fragment.ToSql() }
	t.Run("exact visitors and sessions", func(t *testing.T) {
		statement := "SELECT " + toSQL(gen.Uniq("user_id", false)) + ", " + toSQL(gen.VisitsForEvent(false)) + " FROM (SELECT number AS user_id, intDiv(number, 2) AS session_id FROM numbers(100000))"
		var visitors, visits uint64
		if err := conn.QueryRow(ctx, statement).Scan(&visitors, &visits); err != nil {
			t.Fatal(err)
		}
		if visitors != 100000 || visits != 50000 {
			t.Fatalf("visitors=%d visits=%d", visitors, visits)
		}
	})
	t.Run("active time per pageview and unscaled percentage", func(t *testing.T) {
		for _, sampling := range []bool{false, true} {
			statement := "SELECT " + toSQL(gen.TimeOnPageForEvent(sampling)) + ", " + toSQL(gen.ScrollDepthForEvent(sampling)) + ` FROM values('name String, engagement_time Int64, scroll_depth UInt8, _sample_factor Float64', ('pageview',0,0,10), ('engagement',10000,60,10), ('engagement',10000,80,10), ('engagement',5000,100,10), ('pageview',0,0,10))`
			var duration, depth float64
			if err := conn.QueryRow(ctx, statement).Scan(&duration, &depth); err != nil {
				t.Fatal(err)
			}
			if duration != 12.5 || depth != 80 {
				t.Fatalf("sampling=%v duration=%v depth=%v", sampling, duration, depth)
			}
		}
		for _, rows := range []string{"('pageview',0)", "('engagement',10000)", "('pageview',0), ('engagement',-10000)"} {
			var duration float64
			statement := "SELECT " + toSQL(gen.TimeOnPageForEvent(false)) + " FROM values('name String, engagement_time Int64', " + rows + ")"
			if err := conn.QueryRow(ctx, statement).Scan(&duration); err != nil || duration != 0 {
				t.Fatalf("rows=%s duration=%v err=%v", rows, duration, err)
			}
		}
	})

	site, err := db.Client.Site.Create().SetDomain(fmt.Sprintf("accuracy-%d.example.test", time.Now().UnixNano())).SetTimezone("UTC").Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Client.Site.DeleteOne(site).Exec(ctx)
	if !db.Client.SearchEngines.Query().Where(searchengines.Domain("google.com")).ExistX(ctx) {
		engine, err := db.Client.SearchEngines.Create().SetDomain("google.com").SetName("Google").Save(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer db.Client.SearchEngines.DeleteOne(engine).Exec(ctx)
	}
	InvalidateSearchEngineCache()
	defer InvalidateSearchEngineCache()
	base := time.Now().UTC().Truncate(time.Second).Add(-time.Minute)
	for i, source := range []struct{ utm, ref string }{{"newsletter", ""}, {"promotion", ""}, {"", "google.com.evil.test"}, {"", "www.google.com"}, {"", ""}} {
		sid := uint64(i + 1)
		s := &models.Sessions{SiteId: uint64(site.ID), SessionId: sid, UserId: sid, Version: 1, Sign: 1, Start: base, Timestamp: base, PageViews: 1, Events: 1, IsBounce: 1, UtmSource: source.utm, ReferrerSource: source.ref}
		if err := repository.GetSessionsRepository().BatchInsert(ctx, []*models.Sessions{s}); err != nil {
			t.Fatal(err)
		}
		// The event source deliberately differs; visit:source must use acquisition.
		e := &models.Events{SiteId: uint64(site.ID), SessionId: sid, UserId: sid, Timestamp: base, Name: "pageview", PathName: "/landing", UtmSource: "later-campaign"}
		if err := repository.GetEventsRepository().BatchInsert(ctx, []*models.Events{e}); err != nil {
			t.Fatal(err)
		}
	}
	for _, metric := range []string{"pageviews", "visits"} {
		t.Run("source groups and filters/"+metric, func(t *testing.T) {
			for _, source := range []string{"newsletter", "promotion", "google.com.evil.test", "Google", "Direct / None", ""} {
				q := &types.Query{SiteID: fmt.Sprint(site.ID), Timezone: "UTC", Metrics: []string{metric}, Dimensions: []string{"visit:source"}, Now: time.Now(), UTCTimeRange: types.TimeRange{Start: base.Add(-time.Minute), End: base.Add(time.Minute)}}
				if source != "" {
					q.Filters = []*types.Filter{{Operator: "is", Dimension: "visit:source", Values: []any{source}}}
				}
				statement, args, err := NewQueryBuilder().Build(q, &types.Site{ID: q.SiteID, Timezone: "UTC"})
				if err != nil {
					t.Fatal(err)
				}
				rows, err := conn.Query(ctx, "SELECT referrer_source, toFloat64("+metric+") FROM ("+statement+")", args...)
				if err != nil {
					t.Fatalf("%v\n%s", err, statement)
				}
				got := map[string]float64{}
				for rows.Next() {
					var name string
					var value float64
					if err := rows.Scan(&name, &value); err != nil {
						t.Fatal(err)
					}
					got[name] = value
				}
				if err := rows.Err(); err != nil {
					t.Fatal(err)
				}
				rows.Close()
				if source == "" {
					if len(got) != 5 {
						t.Fatalf("campaigns incorrectly merged: %v", got)
					}
				} else if len(got) != 1 || math.Abs(got[source]-1) > .005 {
					t.Fatalf("source=%s result=%v\nSQL=%s\nargs=%v", source, got, statement, args)
				}
			}
		})
	}
}
