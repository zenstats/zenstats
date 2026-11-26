package clickhouse

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/zenstats/zenstats/config"
)

type ddlRecorder struct {
	driver.Conn
	queries []string
	failAt  int
	err     error
}

func (c *ddlRecorder) Exec(_ context.Context, query string, _ ...any) error {
	c.queries = append(c.queries, query)
	if c.failAt == len(c.queries) {
		return c.err
	}
	return nil
}

func migrationConfig(t *testing.T) {
	t.Helper()
	old := config.Conf
	copy := *old
	config.Conf = &copy
	t.Cleanup(func() { config.Conf = old })
	config.Conf.Clickhouse.Database = "custom_Stats42"
	config.Conf.Clickhouse.EventsTTLDays = 30
	config.Conf.Clickhouse.SessionsTTLDays = 60
}

func TestDatabaseName(t *testing.T) {
	migrationConfig(t)
	for _, name := range []string{"", "zenstats_events_db", "custom_Stats42", "_analytics"} {
		config.Conf.Clickhouse.Database = name
		got, err := DatabaseName()
		if err != nil || got == "" {
			t.Fatalf("%q: %q %v", name, got, err)
		}
		if name == "" && got != "zenstats_events_db" {
			t.Fatal(got)
		}
	}
	for _, name := range []string{"1db", "db-name", "db.table", "db;DROP TABLE events", "db'", "db\n", " db", "中文"} {
		config.Conf.Clickhouse.Database = name
		if _, err := DatabaseName(); err == nil {
			t.Fatalf("%q accepted", name)
		}
	}
}

func TestMigrationCustomDatabaseAndTTL(t *testing.T) {
	migrationConfig(t)
	for _, days := range []int{30, 0} {
		config.Conf.Clickhouse.EventsTTLDays = days
		config.Conf.Clickhouse.SessionsTTLDays = days
		conn := &ddlRecorder{}
		m := &Migration{conn: conn}
		if err := m.ensureBaseTables(context.Background()); err != nil {
			t.Fatal(err)
		}
		joined := strings.Join(conn.queries, "\n")
		if strings.Contains(joined, "zenstats_events_db") || strings.Contains(joined, "{{database}}") {
			t.Fatal("unresolved database")
		}
		for _, part := range []string{"CREATE TABLE IF NOT EXISTS custom_Stats42.events", "CREATE TABLE IF NOT EXISTS custom_Stats42.sessions", "DICTIONARY custom_Stats42.location_data_dict", "DB 'custom_Stats42'", "dictGet('custom_Stats42.location_data_dict'", "ALTER TABLE custom_Stats42.events ADD COLUMN"} {
			if !strings.Contains(joined, part) {
				t.Fatalf("missing %q", part)
			}
		}
		for _, pair := range [][2]string{{"events", "timestamp"}, {"sessions", "start"}} {
			clause := fmt.Sprintf("TTL %s + INTERVAL %d DAY DELETE", pair[1], days)
			if days > 0 {
				if strings.Count(joined, clause) != 2 {
					t.Fatalf("expected CREATE and ALTER TTL for %s", pair[0])
				}
				if !strings.Contains(joined, "ALTER TABLE custom_Stats42."+pair[0]+" MODIFY "+clause) {
					t.Fatal("missing ALTER TTL")
				}
			} else if strings.Contains(joined, "TTL ") {
				t.Fatal("TTL enabled with zero days")
			}
		}
		if err := m.dropTable(context.Background(), "events"); err != nil {
			t.Fatal(err)
		}
		if got := conn.queries[len(conn.queries)-1]; got != "DROP TABLE IF EXISTS custom_Stats42.events;" {
			t.Fatal(got)
		}
	}
}

func TestEnsureBaseTablesPropagatesEveryFailure(t *testing.T) {
	migrationConfig(t)
	good := &ddlRecorder{}
	if err := (&Migration{conn: good}).ensureBaseTables(context.Background()); err != nil {
		t.Fatal(err)
	}
	cause := errors.New("DDL failed")
	for i, query := range good.queries {
		t.Run(fmt.Sprintf("statement_%d", i+1), func(t *testing.T) {
			conn := &ddlRecorder{failAt: i + 1, err: cause}
			if err := (&Migration{conn: conn}).ensureBaseTables(context.Background()); !errors.Is(err, cause) {
				t.Fatalf("swallowed failure in %s: %v", query, err)
			}
			if len(conn.queries) != i+1 {
				t.Fatal("continued after failure")
			}
		})
	}
	config.Conf.Clickhouse.Database = "invalid;db"
	conn := &ddlRecorder{}
	if err := (&Migration{conn: conn}).ensureBaseTables(context.Background()); err == nil || len(conn.queries) != 0 {
		t.Fatal("invalid name reached database")
	}
}
