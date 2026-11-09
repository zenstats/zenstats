package clickhouse

import (
	"context"
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2"
)

// EnsureHeatmapTables also supports an isolated database for integration tests.
// UUID set states make retries idempotent even when the source INSERT committed
// before a dependent view failed. Source deduplication is deliberately disabled:
// retrying must run the view again. The raw table eventually replaces duplicates.
func EnsureHeatmapTables(ctx context.Context, conn clickhouse.Conn) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS heatmap_clicks (
			site_id UInt64, click_id UUID, batch_id UUID,
			timestamp DateTime64(3, 'UTC'), bucket DateTime('UTC'),
			hostname LowCardinality(String), path String, version LowCardinality(String), variant LowCardinality(String),
			viewport_width UInt16, viewport_height UInt16, document_width UInt32, document_height UInt32,
			x Float32, y Float32, element LowCardinality(String), relative_x Float32, relative_y Float32,
			position LowCardinality(String), sample_rate Float64,
			raw_expires DateTime('UTC'), aggregate_expires DateTime('UTC')
		) ENGINE = ReplacingMergeTree
		PARTITION BY toYYYYMMDD(timestamp)
		ORDER BY (site_id, hostname, path, version, variant, viewport_width, viewport_height, document_width, document_height, bucket, click_id)
		TTL raw_expires DELETE
		SETTINGS index_granularity = 8192, non_replicated_deduplication_window = 0`,
		`CREATE TABLE IF NOT EXISTS heatmap_clicks_hourly (
			site_id UInt64, bucket DateTime('UTC'),
			hostname LowCardinality(String), path String, version LowCardinality(String), variant LowCardinality(String),
			viewport_width UInt16, viewport_height UInt16, document_width UInt32, document_height UInt32,
			grid_x Int32, grid_y Int32, element LowCardinality(String), sample_rate Float64,
			clicks AggregateFunction(uniqExact, UUID), last_click SimpleAggregateFunction(max, DateTime64(3, 'UTC')),
			expires DateTime('UTC')
		) ENGINE = AggregatingMergeTree
		PARTITION BY toYYYYMM(bucket)
		ORDER BY (site_id, hostname, path, version, variant, viewport_width, viewport_height, document_width, document_height, bucket, grid_x, grid_y, element, sample_rate, expires)
		TTL expires DELETE
		SETTINGS index_granularity = 8192`,
		`CREATE MATERIALIZED VIEW IF NOT EXISTS heatmap_clicks_mv TO heatmap_clicks_hourly AS
		SELECT site_id, bucket, hostname, path, version, variant,
			viewport_width, viewport_height, document_width, document_height,
			if(position = 'document', intDiv(toInt32(x), 16) * 16, -1) AS grid_x,
			if(position = 'document', intDiv(toInt32(y), 16) * 16, -1) AS grid_y,
			element, sample_rate, uniqExactState(click_id) AS clicks, max(timestamp) AS last_click, aggregate_expires AS expires
		FROM heatmap_clicks
		GROUP BY site_id, bucket, hostname, path, version, variant,
			viewport_width, viewport_height, document_width, document_height, grid_x, grid_y, element, sample_rate, expires`,
	}
	for _, query := range queries {
		if err := conn.Exec(ctx, query); err != nil {
			return fmt.Errorf("heatmap migration: %w", err)
		}
	}
	return nil
}
