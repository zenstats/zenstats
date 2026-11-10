package repository

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/zenstats/zenstats/config"
	cl "github.com/zenstats/zenstats/internal/store/clickhouse"
	"github.com/zenstats/zenstats/internal/store/clickhouse/models"
)

func TestLocationRejectsEmptyNames(t *testing.T) {
	// A nil connection/cache ensures invalid geolocation never reaches storage.
	r := &LocationRepository{}
	if _, err := r.GetOrCreate(context.Background(), "city", " "); err == nil {
		t.Fatal("empty city accepted")
	}
	for _, pair := range [][2]string{{"", "US"}, {"United States", ""}} {
		if _, err := r.GetOrCreateById(context.Background(), "country", pair[0], pair[1]); err == nil {
			t.Fatal("empty country name/id accepted")
		}
	}
}

func TestLocationIDIntegration(t *testing.T) {
	if testing.Short() || os.Getenv("APP_ENV") != "test" {
		t.Skip("requires isolated APP_ENV=test ClickHouse")
	}
	t.Chdir(t.TempDir())
	if err := config.Load("test"); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	admin := cl.GetConnection()
	database := fmt.Sprintf("location_test_%d", time.Now().UnixNano())
	if err := admin.Exec(ctx, "CREATE DATABASE "+database); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(ctx, "DROP DATABASE "+database+" SYNC")
	conn, err := clickhouse.Open(&clickhouse.Options{Addr: config.Conf.Clickhouse.Addr, Auth: clickhouse.Auth{Database: database, Username: config.Conf.Clickhouse.Username, Password: config.Conf.Clickhouse.Password}})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.Exec(ctx, "CREATE TABLE location_data(id String,name String,type String) ENGINE=MergeTree ORDER BY(type,id)"); err != nil {
		t.Fatal(err)
	}
	r := &LocationRepository{conn: conn, cache: expirable.NewLRU[string, *models.LocationData](10, nil, time.Minute)}
	for _, id := range []string{"AA", "BB", "AA", "BB"} {
		location, err := r.GetOrCreateById(ctx, "country", "Same localized name", id)
		if err != nil {
			t.Fatal(err)
		}
		if location.ID != id {
			t.Fatalf("country ID collapsed by name: got %s want %s", location.ID, id)
		}
	}
	var count uint64
	if err := conn.QueryRow(ctx, "SELECT count() FROM location_data").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("got %d dictionary records, want 2", count)
	}
}
