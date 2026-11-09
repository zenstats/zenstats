package repository

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/zenstats/zenstats/config"
	cl "github.com/zenstats/zenstats/internal/store/clickhouse"
	"github.com/zenstats/zenstats/internal/store/clickhouse/models"
	"github.com/zenstats/zenstats/pkg/geoip"
)

func TestActiveSessionLookupIntegration(t *testing.T) {
	if testing.Short() || os.Getenv("APP_ENV") != "test" {
		t.Skip("requires APP_ENV=test and isolated test ClickHouse")
	}
	t.Chdir(t.TempDir())
	if err := config.Load("test"); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := cl.NewMigration().Run(); err != nil {
		t.Fatal(err)
	}
	admin := cl.GetConnection()
	database := fmt.Sprintf("session_lookup_test_%d", time.Now().UnixNano())
	if err := admin.Exec(ctx, "CREATE DATABASE "+database); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := admin.Exec(ctx, "DROP DATABASE "+database+" SYNC"); err != nil {
			t.Error(err)
		}
	})
	if err := admin.Exec(ctx, "CREATE TABLE "+database+".sessions AS zenstats_events_db.sessions"); err != nil {
		t.Fatal(err)
	}
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: config.Conf.Clickhouse.Addr,
		Auth: clickhouse.Auth{Database: database, Username: config.Conf.Clickhouse.Username, Password: config.Conf.Clickhouse.Password},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	repo := &Sessions{conn: conn}
	// Cancellation rows must be tested in separate, unmerged parts too.
	if err := conn.Exec(ctx, "SYSTEM STOP MERGES sessions"); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 2, 1, 0, 15, 0, 0, time.UTC)
	makeSession := func(user, id uint64, start, end time.Time) *models.Sessions {
		return &models.Sessions{
			SiteId: 41, UserId: user, SessionId: id, Version: 1, Sign: 1,
			Start: start, Timestamp: end, IsBounce: 1, PageViews: 1, Events: 1,
			EntryPage: "/entry", ExitPage: "/entry", URL: "https://example.test/entry",
			HostName: "example.test", PathName: "/entry", Referrer: "https://search.test/",
			ReferrerSource: "search", OperatingSystem: "Linux", ScreenSize: "Desktop",
			UtmMedium: "email", UtmSource: "newsletter", UtmContent: "cta", UtmTerm: "term", UtmCampaign: "launch",
			EntryMetaKey: []string{"plan"}, EntryMetaValue: []string{"paid"}, Browser: "Firefox", BrowserVersion: "130",
			UserAgent: "fixture", OperatingSystemVersion: "6", IP: net.ParseIP("192.0.2.1").To4(),
			IPv6: net.ParseIP("2001:db8::1"), CountryCode: "US", ContinentGeonameId: "6255149", CityGeonameId: "5128581",
			Coordinates: geoip.Coordinates{Latitude: 40.7, Longitude: -74}, Channel: "Email",
		}
	}
	insert := func(rows ...*models.Sessions) {
		t.Helper()
		if err := repo.BatchInsert(ctx, rows); err != nil {
			t.Fatal(err)
		}
	}
	active := makeSession(116385, 1, now.Add(-time.Hour), now.Add(-5*time.Minute))
	insert(active)
	cancel := *active
	cancel.Sign = -1
	latest := *active
	latest.Version, latest.PageViews, latest.Events, latest.IsBounce = 2, 4, 5, 0
	latest.Timestamp, latest.ExitPage, latest.Duration = now, "/exit", 3600
	insert(&cancel, &latest)
	cancelled := makeSession(2, 2, now.Add(-time.Hour), now)
	insert(cancelled)
	cancelled.Sign = -1
	insert(cancelled)
	earlier := makeSession(3, 3, now.Add(-3*time.Hour), now.Add(-2*time.Hour))
	later := makeSession(3, 4, now.Add(-time.Hour), now)
	insert(earlier, later)
	long := makeSession(4, 5, now.Add(-6*24*time.Hour), now)
	insert(long)
	expired := makeSession(5, 6, now.Add(-time.Hour), now.Add(-30*time.Minute-time.Second))
	future := makeSession(6, 7, now.Add(time.Second), now.Add(time.Second))
	tooOld := makeSession(7, 8, now.Add(-7*24*time.Hour-time.Second), now)
	boundary := makeSession(8, 9, now.Add(-7*24*time.Hour), now.Add(-30*time.Minute))
	insert(expired, future, tooOld, boundary)
	otherSite := makeSession(active.UserId, 99, now, now)
	otherSite.SiteId++
	insert(otherSite)
	// A newer, completely cancelled session must not hide a live earlier one.
	removed := makeSession(3, 10, now.Add(-time.Minute), now)
	insert(removed)
	removed.Sign = -1
	insert(removed)
	cases := []struct {
		name string
		user uint64
		at   time.Time
		want *models.Sessions
	}{
		{"latest state across month boundary and site isolation", active.UserId, now, &latest},
		{"cancelled state stays absent", 2, now, nil},
		{"late event restores earlier session", 3, earlier.Timestamp, earlier},
		{"latest live session", 3, now, later},
		{"long active session", 4, now, long},
		{"inactivity expires session", 5, now, nil},
		{"future start excluded", 6, now, nil},
		{"recovery horizon", 7, now, nil},
		{"inclusive boundaries", 8, now, boundary},
		{"unknown visitor", 999, now, nil},
	}
	verify := func(t *testing.T) {
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				got, err := repo.GetActiveSessionAt(ctx, tc.user, 41, tc.at, 30*time.Minute)
				if tc.want == nil {
					if !errors.Is(err, ErrSessionNotFound) {
						t.Fatalf("want not found, got %+v, %v", got, err)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				// Compare every restored field, normalizing driver time/IP representations.
				got.Start, got.Timestamp = got.Start.UTC(), got.Timestamp.UTC()
				got.IP = got.IP.To4()
				if !reflect.DeepEqual(got, tc.want) {
					t.Fatalf("restored state mismatch\ngot: %+v\nwant: %+v", got, tc.want)
				}
			})
		}
	}
	t.Run("unmerged parts", verify)
	if err := conn.Exec(ctx, "SYSTEM START MERGES sessions"); err != nil {
		t.Fatal(err)
	}
	if err := conn.Exec(ctx, "OPTIMIZE TABLE sessions FINAL"); err != nil {
		t.Fatal(err)
	}
	t.Run("merged parts", verify)
	t.Run("lookup does not read unrelated wide rows", func(t *testing.T) {
		if err := conn.Exec(ctx, "SYSTEM STOP MERGES sessions"); err != nil {
			t.Fatal(err)
		}
		// Overlapping parts with 262k unrelated visitors and wide metadata recreate
		// the FINAL read expansion that blocked production ingestion.
		for part := 0; part < 2; part++ {
			if err := conn.Exec(ctx, `INSERT INTO sessions
				(start, timestamp, site_id, user_id, session_id, version, sign, user_agent, url)
				SELECT toDateTime('2026-02-01 00:00:00') - toIntervalDay(number % 7),
				toDateTime('2026-02-01 00:15:00'), 41, 100000 + intDiv(number, 7) * 2,
				number + ?, 1, 1, repeat('agent', 100), repeat('url', 100)
				FROM numbers(131072)`, uint64(part*131072+1000)); err != nil {
				t.Fatal(err)
			}
		}
		var optimizedBytes atomic.Uint64
		lookupCtx := clickhouse.Context(ctx, clickhouse.WithProgress(func(p *clickhouse.Progress) {
			optimizedBytes.Add(p.Bytes)
		}))
		if _, err := repo.GetActiveSessionAt(lookupCtx, active.UserId, 41, now, 30*time.Minute); err != nil {
			t.Fatal(err)
		}
		var finalBytes atomic.Uint64
		baselineCtx := clickhouse.Context(ctx, clickhouse.WithProgress(func(p *clickhouse.Progress) {
			finalBytes.Add(p.Bytes)
		}))
		rows, err := conn.Query(baselineCtx, `SELECT `+sessionStateColumns+`
			FROM sessions FINAL
			WHERE site_id = ? AND user_id = ? AND start >= ? AND start <= ?
			AND timestamp >= ? AND sign = 1 ORDER BY start DESC, version DESC LIMIT 1
			SETTINGS max_threads = 1`, uint64(41), active.UserId, now.AddDate(0, 0, -7), now, now.Add(-30*time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		rows.Close()
		t.Logf("read bytes: FINAL=%d, visitor aggregation=%d", finalBytes.Load(), optimizedBytes.Load())
		if optimizedBytes.Load() == 0 || optimizedBytes.Load()*4 >= finalBytes.Load() {
			t.Fatal("lookup must reduce wide-row reads by at least 75% on overlapping parts")
		}
	})
}
