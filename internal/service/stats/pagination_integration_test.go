package stats

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/zenstats/zenstats/config"
	"github.com/zenstats/zenstats/internal/service/stats/types"
	cl "github.com/zenstats/zenstats/internal/store/clickhouse"
)

// Exercise the actual paginated country/city query, including FINAL, exact UVs,
// joins, offsets and metadata. Each test uses disposable tables, not live data.
func TestRegionPaginationIntegration(t *testing.T) {
	if testing.Short() || os.Getenv("APP_ENV") != "test" {
		t.Skip("requires isolated ClickHouse")
	}
	t.Chdir(t.TempDir())
	if err := config.Load("test"); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	admin := cl.GetConnection()
	database := fmt.Sprintf("region_pagination_%d", time.Now().UnixNano())
	if err := admin.Exec(ctx, "CREATE DATABASE "+database); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(ctx, "DROP DATABASE "+database+" SYNC")
	conn, err := clickhouse.Open(&clickhouse.Options{Addr: config.Conf.Clickhouse.Addr, Auth: clickhouse.Auth{Database: database, Username: config.Conf.Clickhouse.Username, Password: config.Conf.Clickhouse.Password}, MaxOpenConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	for _, ddl := range []string{
		`CREATE TABLE sessions(site_id UInt64,user_id UInt64,session_id UInt64,sign Int8,version UInt64,start DateTime,timestamp DateTime,country_code String,city_geoname_id String,is_bounce UInt8,duration UInt32,page_views UInt32,country_name String ALIAS country_code,city_name String ALIAS city_geoname_id) ENGINE=VersionedCollapsingMergeTree(sign,version) ORDER BY(site_id,session_id)`,
		`CREATE TABLE events(site_id UInt64,user_id UInt64,session_id UInt64,timestamp DateTime,name String,path_name String,country_code String,city_geoname_id String,country_name String ALIAS country_code,city_name String ALIAS city_geoname_id) ENGINE=MergeTree ORDER BY(site_id,timestamp)`,
		`INSERT INTO sessions(site_id,user_id,session_id,sign,version,start,timestamp,country_code,city_geoname_id,is_bounce,duration,page_views) VALUES(1,1,1,1,1,now(),now(),'US','New York',1,0,1),(1,2,2,1,1,now(),now(),'US','New York',1,0,1),(1,3,3,1,1,now(),now(),'GB','London',1,0,1),(1,4,4,1,1,now(),now(),'UN','',1,0,1)`,
		`INSERT INTO events(site_id,user_id,session_id,timestamp,name,path_name,country_code,city_geoname_id) VALUES(1,1,1,now(),'pageview','/a','US','New York'),(1,2,2,now(),'pageview','/b','US','New York'),(1,3,3,now(),'pageview','/c','GB','London'),(1,4,4,now(),'pageview','/d','UN','')`,
	} {
		if err := conn.Exec(ctx, ddl); err != nil {
			t.Fatal(err)
		}
	}
	runner := &QueryRunner{conn: conn}
	now := time.Now()
	site := &types.Site{ID: "1", Timezone: "UTC"}
	for _, dimension := range []string{"visit:country", "visit:city"} {
		for _, metrics := range [][]string{{"visitors"}, {"visitors", "pageviews", "bounce_rate"}} {
			for _, offset := range []int{0, 2, 99} {
				query := &types.Query{SiteID: "1", Timezone: "UTC", Dimensions: []string{dimension}, Metrics: metrics, Now: now, UTCTimeRange: types.TimeRange{Start: now.Add(-30 * 24 * time.Hour), End: now.Add(time.Hour)}, Pagination: &types.Pagination{Limit: 2, Offset: offset}, OrderBy: []*types.OrderBy{{Dimension: "visitors", Direction: "desc"}}}
				result, err := runner.RunQuery(ctx, query, site)
				if err != nil {
					t.Fatalf("%s %v offset=%d: %v", dimension, metrics, offset, err)
				}
				want := 2
				if offset == 2 {
					want = 1
				}
				if offset == 99 {
					want = 0
				}
				if result.TotalRows != 3 || len(result.Data) != want || len(result.Columns) != len(metrics)+1 {
					t.Fatalf("pagination result: %+v", result)
				}
				if offset == 0 && result.Data[0]["visitors"] != uint64(2) {
					t.Fatalf("UV/order changed: %+v", result.Data)
				}
				for _, row := range result.Data {
					if _, exists := row["__zenstats_total_rows"]; exists {
						t.Fatal("internal metadata exposed")
					}
				}
			}
		}
	}
}
