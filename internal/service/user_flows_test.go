package service

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/zenstats/zenstats/config"
	"github.com/zenstats/zenstats/internal/service/stats/types"
	cl "github.com/zenstats/zenstats/internal/store/clickhouse"
)

func assertFlowConservation(t *testing.T, data *UserFlowsResponse, steps int) {
	t.Helper()
	incoming, outgoing := make([]uint64, len(data.Nodes)), make([]uint64, len(data.Nodes))
	for _, edge := range data.Links {
		if data.Nodes[edge.Target].Depth != data.Nodes[edge.Source].Depth+1 {
			t.Fatal("nonadjacent flow edge")
		}
		incoming[edge.Target] += edge.Value
		outgoing[edge.Source] += edge.Value
	}
	var total uint64
	for i, node := range data.Nodes {
		if node.Depth == 0 {
			total += outgoing[i]
		}
		if node.Depth > 0 && node.Depth < steps-1 && !node.IsDropOff && incoming[i] != outgoing[i] {
			t.Fatalf("flow lost at %+v: in=%d out=%d", node, incoming[i], outgoing[i])
		}
		if node.IsDropOff && outgoing[i] != 0 {
			t.Fatal("exit has outgoing flow")
		}
		if node.Depth == steps-1 && outgoing[i] != 0 {
			t.Fatal("last step must not force an exit")
		}
	}
	if total != data.TotalSessions {
		t.Fatalf("first-step sum=%d total=%d", total, data.TotalSessions)
	}
}

func TestUserFlowsGraph(t *testing.T) {
	rows := []rawFlowEdge{
		{Depth: 0, Source: "/a", Target: "/b", Count: 2},
		{Depth: 0, Source: "/a", IsDropOff: 1, Count: 1},
		{Depth: 1, Source: "/b", Target: "/c", Count: 1},
		{Depth: 1, Source: "/b", IsDropOff: 1, Count: 1},
	}
	data := buildSankeyGraph(rows)
	if data.TotalSessions != 3 || data.DropOffSessions != 2 || data.DropOffRate < 66.6 || data.DropOffRate > 66.7 {
		t.Fatalf("bad summary: %+v", data)
	}
	assertFlowConservation(t, data, 3)
	for range 10 {
		if !reflect.DeepEqual(data, buildSankeyGraph(rows)) {
			t.Fatal("unstable graph ordering")
		}
	}
	empty := buildSankeyGraph(nil)
	if empty.Nodes == nil || empty.Links == nil {
		t.Fatal("empty arrays must serialize as []")
	}
}

func TestUserFlowsIntegration(t *testing.T) {
	if testing.Short() || os.Getenv("APP_ENV") != "test" {
		t.Skip("requires isolated APP_ENV=test ClickHouse")
	}
	t.Chdir(t.TempDir())
	if err := config.Load("test"); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	admin := cl.GetConnection()
	db := fmt.Sprintf("flows_test_%d", time.Now().UnixNano())
	if err := admin.Exec(ctx, "CREATE DATABASE "+db); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(ctx, "DROP DATABASE "+db+" SYNC")
	conn, err := clickhouse.Open(&clickhouse.Options{Addr: config.Conf.Clickhouse.Addr, Auth: clickhouse.Auth{Database: db, Username: config.Conf.Clickhouse.Username, Password: config.Conf.Clickhouse.Password}})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	for _, ddl := range []string{
		`CREATE TABLE events(site_id UInt64,user_id UInt64,session_id UInt64,timestamp DateTime,name String,pathname String,country_code String,browser String) ENGINE=MergeTree ORDER BY(site_id,timestamp)`,
		`CREATE TABLE sessions(site_id UInt64,user_id UInt64,session_id UInt64,start DateTime,timestamp DateTime,entry_page String,sign Int8,version UInt64) ENGINE=VersionedCollapsingMergeTree(sign,version) ORDER BY(site_id,user_id,session_id)`,
		`INSERT INTO events SELECT 1,1,1,toDateTime('2026-11-10 00:00:00')+number,'pageview','/a','CN','Chrome' FROM numbers(10000)`,
		`INSERT INTO events VALUES(1,1,1,'2026-11-10 03:00:00','pageview','/b','CN','Chrome'),(1,1,1,'2026-11-10 03:01:00','pageview','/c','CN','Chrome'),(1,1,1,'2026-11-10 03:02:00','pageview','/d','CN','Chrome'),(1,1,1,'2026-11-10 03:03:00','signup','/d','CN','Chrome'),(1,1,2,'2026-11-10 00:00:00','pageview','/a','US','Firefox'),(1,1,2,'2026-11-10 00:01:00','pageview','/b','US','Firefox'),(1,1,3,'2026-11-10 00:00:00','pageview','/a','US','Firefox'),(1,2,1,'2026-11-10 00:00:00','pageview','/x','US','Firefox'),(1,2,1,'2026-11-10 00:01:00','pageview','/y','US','Firefox'),(1,2,1,'2026-11-10 00:02:00','pageview','/z','US','Firefox'),(1,9,0,'2026-11-10 00:00:00','pageview','/invalid','CN','Chrome'),(2,1,1,'2026-11-10 00:00:00','pageview','/foreign','CN','Chrome'),(1,1,1,'2026-11-09 00:00:00','pageview','/before','CN','Chrome'),(1,1,1,'2026-11-11 00:00:00','pageview','/after','CN','Chrome')`,
		`INSERT INTO sessions VALUES(1,1,1,'2026-11-09 00:00:00','2026-11-10 03:03:00','/before',1,1),(1,1,2,'2026-11-10 00:00:00','2026-11-10 00:01:00','/a',1,1),(1,1,3,'2026-11-10 00:00:00','2026-11-10 00:00:00','/a',1,1),(1,2,1,'2026-11-10 00:00:00','2026-11-10 00:02:00','/x',1,1)`,
	} {
		if err := conn.Exec(ctx, ddl); err != nil {
			t.Fatal(err)
		}
	}
	start := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	query := &types.Query{SiteID: "1", UTCTimeRange: types.TimeRange{Start: start, End: start.Add(24*time.Hour - time.Second)}}
	service := &StatsService{cl: conn}
	run := func(steps int, entry string, filters []*types.Filter) *UserFlowsResponse {
		t.Helper()
		query.Filters = filters
		edges, err := service.fetchFlowEdges(ctx, query, UserFlowsParams{Steps: steps, Limit: 3, EntryPath: entry})
		if err != nil {
			t.Fatal(err)
		}
		result := buildSankeyGraph(edges)
		assertFlowConservation(t, result, steps)
		return result
	}
	for _, c := range []struct {
		steps int
		drop  uint64
	}{{2, 1}, {3, 2}, {4, 3}} {
		data := run(c.steps, "", nil)
		if data.TotalSessions != 4 || data.DropOffSessions != c.drop {
			t.Fatalf("steps=%d: %+v", c.steps, data)
		}
	}
	if data := run(3, "/a", nil); data.TotalSessions != 3 || data.DropOffSessions != 2 {
		t.Fatalf("entry filter: %+v", data)
	}
	if data := run(3, "/b", nil); data.TotalSessions != 0 {
		t.Fatal("later page became an entry")
	}
	for _, filter := range []*types.Filter{
		{Operator: "is", Dimension: "visit:country", Values: []any{"CN"}},
		{Operator: "is", Dimension: "event:name", Values: []any{"signup"}},
		{Operator: "is", Dimension: "event:page", Values: []any{"/c"}},
		{Operator: "is", Dimension: "visit:entry_page", Values: []any{"/before"}},
	} {
		data := run(3, "", []*types.Filter{filter})
		if data.TotalSessions != 1 || data.DropOffSessions != 0 {
			t.Fatalf("filter %s erased the full path: %+v", filter.Dimension, data)
		}
		found := false
		for _, n := range data.Nodes {
			if n.Depth == 0 && n.Name == "/a" {
				found = true
			}
		}
		if !found {
			t.Fatal("filtered path no longer begins at /a")
		}
	}
	// More than 1,000 distinct paths: no LIMIT may silently change the cohort.
	if err := conn.Exec(ctx, `INSERT INTO events SELECT 1,100+number,100+number,toDateTime('2026-11-10 04:00:00'),'pageview',concat('/tail/',toString(number)),'US','Firefox' FROM numbers(1105)`); err != nil {
		t.Fatal(err)
	}
	a := run(3, "", nil)
	b := run(3, "", nil)
	if a.TotalSessions != 1109 || a.DropOffSessions != 1107 {
		t.Fatalf("long-tail sessions dropped: %+v", a)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatal("Top N ties are unstable")
	}
	if len(a.Links) > 2*4*5 || len(a.Nodes) > 3*5 {
		t.Fatalf("unbounded graph: %d links %d nodes", len(a.Links), len(a.Nodes))
	}
	other := false
	for _, n := range a.Nodes {
		other = other || n.IsOther
	}
	if !other {
		t.Fatal("missing Other bucket")
	}
}
