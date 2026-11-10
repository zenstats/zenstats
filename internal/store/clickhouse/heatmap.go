package clickhouse

import (
	"context"
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/zenstats/zenstats/internal/heatmap"
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
		`ALTER TABLE heatmap_clicks ADD COLUMN IF NOT EXISTS aggregation_key LowCardinality(String),
            MODIFY ORDER BY (site_id,hostname,path,version,variant,viewport_width,viewport_height,document_width,document_height,bucket,click_id,aggregation_key)`,
		`ALTER TABLE heatmap_clicks ADD COLUMN IF NOT EXISTS grid_version UInt8`,
		`ALTER TABLE heatmap_clicks_hourly ADD COLUMN IF NOT EXISTS aggregation_key LowCardinality(String),
            ADD COLUMN IF NOT EXISTS grid_version UInt8,
            MODIFY ORDER BY (site_id,hostname,path,version,variant,viewport_width,viewport_height,document_width,document_height,bucket,grid_x,grid_y,element,sample_rate,expires,aggregation_key,grid_version)`,
		`CREATE MATERIALIZED VIEW IF NOT EXISTS heatmap_clicks_mv TO heatmap_clicks_hourly AS ` + HeatmapAggregateQuery(""),
		`ALTER TABLE heatmap_clicks_mv MODIFY QUERY ` + HeatmapAggregateQuery(""),
	}
	for _, query := range queries {
		if err := conn.Exec(ctx, query); err != nil {
			return fmt.Errorf("heatmap migration: %w", err)
		}
	}
	return nil
}

// HeatmapAggregateQuery is shared by the view and Replay. Source rows explicitly
// record their grid generation so replay cannot move legacy UUIDs into a second
// cell. The common 16px-center rounding keeps old/new API retries in the same
// region even at bin boundaries. New API writes use region bins and only three
// dimension combinations;
// old API writes and retained rows keep their original 16px projection.
func HeatmapAggregateQuery(where string) string {
	return fmt.Sprintf(`SELECT * EXCEPT(vw,vh,dw,dh) FROM (SELECT src.site_id, src.bucket, src.hostname, src.path, src.version, src.variant, src.aggregation_key, src.grid_version,
		if(src.grid_version=0, src.viewport_width, toUInt16(multiIf(src.viewport_width<768,390,src.viewport_width<1024,768,1440))) AS vw,
		if(src.grid_version=0, src.viewport_height, toUInt16(multiIf(src.viewport_width<768,844,src.viewport_width<1024,1024,900))) AS vh,
		if(src.grid_version=0, src.document_width, toUInt32(vw)) AS dw,
		if(src.grid_version=0, src.document_height, toUInt32(vh)) AS dh,
		vw AS viewport_width, vh AS viewport_height, dw AS document_width, dh AS document_height,
		if(src.position!='document', -1, if(src.grid_version=0, intDiv(toInt32(src.x),16)*16,
			toInt32(least(%d, floor((intDiv(toInt32(src.x),16)*16+8)/greatest(src.document_width,1)*%d))))) AS grid_x,
		if(src.position!='document', -1, if(src.grid_version=0, intDiv(toInt32(src.y),16)*16,
			toInt32(least(%d, floor((intDiv(toInt32(src.y),16)*16+8)/greatest(src.document_height,1)*%d))))) AS grid_y,
		src.element, src.sample_rate, uniqExactState(src.click_id) AS clicks,
		max(src.timestamp) AS last_click, src.aggregate_expires AS expires
		FROM heatmap_clicks AS src %s
		GROUP BY src.site_id, src.bucket, src.hostname, src.path, src.version, src.variant, src.aggregation_key, src.grid_version,
		vw, vh, dw, dh, grid_x, grid_y, src.element, src.sample_rate, src.aggregate_expires)`,
		heatmap.GridColumns-1, heatmap.GridColumns, heatmap.GridRows-1, heatmap.GridRows, where)
}
