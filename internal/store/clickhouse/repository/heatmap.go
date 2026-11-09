package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/zenstats/zenstats/internal/heatmap"
)

type Heatmap struct{ conn clickhouse.Conn }

func NewHeatmap(conn clickhouse.Conn) *Heatmap { return &Heatmap{conn: conn} }

// Insert never suppresses a repeated block: the idempotent view must run on every
// retry to repair a source-committed/view-failed write.
func (s *Heatmap) Insert(ctx context.Context, rows []heatmap.Row) error {
	ctx = clickhouse.Context(ctx, clickhouse.WithSettings(clickhouse.Settings{
		"insert_deduplicate": 0, "materialized_views_ignore_errors": 0,
	}))
	b, err := s.conn.PrepareBatch(ctx, `INSERT INTO heatmap_clicks
		(site_id,click_id,batch_id,timestamp,bucket,hostname,path,version,variant,
		viewport_width,viewport_height,document_width,document_height,x,y,element,relative_x,relative_y,position,sample_rate,raw_expires,aggregate_expires)`)
	if err != nil {
		return err
	}
	defer b.Abort()
	for _, r := range rows {
		p := r.Page
		if err := b.Append(r.SiteID, r.ID, r.BatchID, r.Timestamp, r.Bucket, p.Hostname, p.Path, p.Version, p.Variant,
			p.ViewportWidth, p.ViewportHeight, p.DocumentWidth, p.DocumentHeight,
			float32(r.X), float32(r.Y), r.Element, float32(r.RelativeX), float32(r.RelativeY), r.Position, r.SampleRate, r.RawExpires, r.AggregateExpires); err != nil {
			return err
		}
	}
	return b.Send()
}

const heatmapQuerySettings = ` SETTINGS max_threads=1, max_execution_time=5, max_memory_usage=134217728`
const heatmapPageFields = `hostname,path,version,variant,viewport_width,viewport_height,document_width,document_height`

func (s *Heatmap) Pages(ctx context.Context, siteID uint64, from, to time.Time) ([]heatmap.PageCount, error) {
	rows, err := s.conn.Query(ctx, `SELECT `+heatmapPageFields+`,uniqExactMerge(clicks),max(last_click)
		FROM heatmap_clicks_hourly WHERE site_id=? AND bucket>=? AND bucket<? AND expires>now()
		GROUP BY `+heatmapPageFields+` ORDER BY max(last_click) DESC LIMIT 200`+heatmapQuerySettings, siteID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []heatmap.PageCount{}
	for rows.Next() {
		var r heatmap.PageCount
		if err := rows.Scan(&r.Hostname, &r.Path, &r.Version, &r.Variant, &r.ViewportWidth, &r.ViewportHeight, &r.DocumentWidth, &r.DocumentHeight, &r.Clicks, &r.LastClick); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Heatmap) Data(ctx context.Context, siteID uint64, page heatmap.Page, from, to time.Time) (heatmap.Data, error) {
	out := heatmap.Data{Points: []heatmap.Point{}, Elements: []heatmap.ElementCount{}, GridSize: heatmap.GridSize}
	where := ` FROM heatmap_clicks_hourly WHERE site_id=? AND hostname=? AND path=? AND version=? AND variant=?
		AND viewport_width=? AND viewport_height=? AND document_width=? AND document_height=? AND bucket>=? AND bucket<? AND expires>now()`
	args := []any{siteID, page.Hostname, page.Path, page.Version, page.Variant, page.ViewportWidth, page.ViewportHeight, page.DocumentWidth, page.DocumentHeight, from, to}
	rows, err := s.conn.Query(ctx, `SELECT grid_x,grid_y,uniqExactMerge(clicks)`+where+` AND grid_x>=0 GROUP BY grid_x,grid_y ORDER BY uniqExactMerge(clicks) DESC LIMIT 10001`+heatmapQuerySettings, args...)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var p heatmap.Point
		if err := rows.Scan(&p.X, &p.Y, &p.Count); err != nil {
			rows.Close()
			return out, err
		}
		if len(out.Points) < 10000 {
			out.Points = append(out.Points, p)
		} else {
			out.Truncated = true
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return out, err
	}
	rows.Close()
	// Total is independent of the bounded point list and includes unmappable clicks.
	if err := s.conn.QueryRow(ctx, `SELECT uniqExactMerge(clicks),uniqExactMergeIf(clicks,grid_x<0)`+where+heatmapQuerySettings, args...).Scan(&out.Clicks, &out.Unmapped); err != nil {
		return out, err
	}
	rows, err = s.conn.Query(ctx, `SELECT element,uniqExactMerge(clicks)`+where+` AND element!='' GROUP BY element ORDER BY uniqExactMerge(clicks) DESC LIMIT 100`+heatmapQuerySettings, args...)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var r heatmap.ElementCount
		if err := rows.Scan(&r.Element, &r.Count); err != nil {
			return out, err
		}
		out.Elements = append(out.Elements, r)
	}
	return out, rows.Err()
}

// Replay repairs an interrupted view write within the raw retention window.
// Exact UUID states make replay safe alongside concurrent inserts and repeated
// maintenance calls; it cannot double the existing hourly counts.
func (s *Heatmap) Replay(ctx context.Context, siteID uint64, from, to time.Time) error {
	if !to.After(from) || to.Sub(from) > 24*time.Hour {
		return fmt.Errorf("replay range must be within 24 hours")
	}
	return s.conn.Exec(ctx, `INSERT INTO heatmap_clicks_hourly
		SELECT site_id,bucket,hostname,path,version,variant,viewport_width,viewport_height,document_width,document_height,
		if(position='document',intDiv(toInt32(x),16)*16,-1) AS grid_x,
		if(position='document',intDiv(toInt32(y),16)*16,-1) AS grid_y,
		element,sample_rate,uniqExactState(click_id),max(timestamp),aggregate_expires
		FROM heatmap_clicks WHERE site_id=? AND timestamp>=? AND timestamp<? AND raw_expires>now() AND aggregate_expires>now()
		GROUP BY site_id,bucket,hostname,path,version,variant,viewport_width,viewport_height,document_width,document_height,grid_x,grid_y,element,sample_rate,aggregate_expires`+heatmapQuerySettings, siteID, from, to)
}
