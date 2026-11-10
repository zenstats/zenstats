package clickhouse

import (
	"context"
	"crypto/tls"
	"log/slog"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/zenstats/zenstats/config"
)

var (
	globalConn driver.Conn
	once       sync.Once
)

func initialize() {
	ctx := context.Background()
	clickhouseCfg := &clickhouse.Options{
		Addr: config.Conf.Clickhouse.Addr,
		Auth: clickhouse.Auth{
			Database: config.Conf.Clickhouse.Database,
			Username: config.Conf.Clickhouse.Username,
			Password: config.Conf.Clickhouse.Password,
		},
		DialTimeout:  time.Second * 60,
		ReadTimeout:  time.Second * 60,
		MaxIdleConns: 5,
		MaxOpenConns: 10,
	}
	// 开启ssl
	if config.Conf.Clickhouse.Ssl {
		clickhouseCfg.TLS = &tls.Config{}
	}

	conn, err := clickhouse.Open(clickhouseCfg)

	if err != nil {
		panic(err)
	}

	if err := conn.Ping(ctx); err != nil {
		if exception, ok := err.(*clickhouse.Exception); ok {
			slog.Error("failed to ping clickhouse", "exception", exception)
		}
		panic(err)
	}
	globalConn = conn
}

// GetConnection returns the ClickHouse connection.
func GetConnection() driver.Conn {
	once.Do(initialize)

	return globalConn
}

// NewHeatmapConnection reserves a small pool for optional heatmap work so its
// queries cannot exhaust connections used by events and session restoration.
func NewHeatmapConnection() (driver.Conn, error) {
	opts := &clickhouse.Options{
		Addr:        config.Conf.Clickhouse.Addr,
		Auth:        clickhouse.Auth{Database: config.Conf.Clickhouse.Database, Username: config.Conf.Clickhouse.Username, Password: config.Conf.Clickhouse.Password},
		DialTimeout: 2 * time.Second, ReadTimeout: 6 * time.Second,
		MaxOpenConns: 2, MaxIdleConns: 1,
	}
	if config.Conf.Clickhouse.Ssl {
		opts.TLS = &tls.Config{}
	}
	return clickhouse.Open(opts)
}
