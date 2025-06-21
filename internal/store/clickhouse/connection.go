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
		DialTimeout: time.Second * 60,
		ReadTimeout: time.Second * 60,
	}
	// 开启ssl
	if config.Conf.Clickhouse.Ssl {
		clickhouseCfg.TLS = &tls.Config{
			InsecureSkipVerify: true,
		}
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
