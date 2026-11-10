package clickhouse

import (
	"context"
	"fmt"
	"github.com/ClickHouse/clickhouse-go/v2"
)

func EnsurePerformanceTables(ctx context.Context, conn clickhouse.Conn) error {
	err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS web_vitals_reports (
 site_id UInt64,lifecycle_id UUID,record_kind LowCardinality(String),metric_name LowCardinality(String),metric_id String,
 navigation_started_at DateTime64(3,'UTC'),report_id UUID,revision UInt16,reported_at DateTime64(3,'UTC'),
 hostname LowCardinality(String),path String,release LowCardinality(String),navigation_type LowCardinality(String),
 device LowCardinality(String),browser LowCardinality(String),country LowCardinality(String),
 value Float64,supported_metrics Array(String),init_delay_ms UInt32,background_init Bool,clipped Bool,
 sample_rate Float64,config_version UInt64,expires DateTime('UTC')
 ) ENGINE=ReplacingMergeTree(revision)
 PARTITION BY toYYYYMMDD(navigation_started_at)
 ORDER BY (site_id,toDate(navigation_started_at),lifecycle_id,record_kind,metric_name,metric_id)
 TTL expires DELETE SETTINGS index_granularity=8192,non_replicated_deduplication_window=0`)
	if err != nil {
		return fmt.Errorf("performance migration: %w", err)
	}
	return nil
}
