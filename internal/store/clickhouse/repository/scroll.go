package repository

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/zenstats/zenstats/internal/heatmap"
)

type ScrollWriter struct{ repo *Heatmap }

func NewScrollWriter(repo *Heatmap) *ScrollWriter { return &ScrollWriter{repo: repo} }

func (s *ScrollWriter) Insert(ctx context.Context, rows []heatmap.ScrollRow) error {
	ctx = clickhouse.Context(ctx, clickhouse.WithSettings(clickhouse.Settings{"insert_deduplicate": 0}))
	b, err := s.repo.conn.PrepareBatch(ctx, `INSERT INTO heatmap_scroll_reports
		(site_id,page_view_id,started_at,revision,report_id,reported_at,hostname,path,aggregation_key,version,variant,
		viewport_width,viewport_height,document_width,document_height,max_bottom,max_height,late_init,clipped,unsupported,sample_rate,expires)`)
	if err != nil {
		return err
	}
	defer b.Abort()
	for _, r := range rows {
		p := r.Page
		if err := b.Append(r.SiteID, r.PageViewID, r.StartedAt, r.Revision, r.ID, r.Timestamp, p.Hostname, p.Path, p.AggregationKey, p.Version, p.Variant,
			p.ViewportWidth, p.ViewportHeight, p.DocumentWidth, p.DocumentHeight, r.MaxBottom, r.MaxHeight, r.LateInit, r.Clipped, r.Unsupported, r.SampleRate, r.Expires); err != nil {
			return err
		}
	}
	return b.Send()
}

// Resolve whole snapshots before dimension filtering. Newer revisions can reduce
// normalized depth as lazy-loaded content grows. Background merges are optional.
const scrollCanonical = `SELECT page_view_id,
	argMax(tuple(hostname,path,aggregation_key,version,variant,viewport_width,viewport_height,document_width,document_height,
	max_bottom,max_height,late_init,clipped,unsupported,sample_rate,reported_at),revision) AS r
	FROM heatmap_scroll_reports WHERE site_id=? AND started_at>=? AND started_at<? AND expires>now()
	GROUP BY page_view_id`

const scrollResolved = `SELECT page_view_id,r.1 AS hostname,r.2 AS path,r.3 AS aggregation_key,r.4 AS version,r.5 AS variant,
	r.6 AS viewport_width,r.7 AS viewport_height,r.8 AS document_width,r.9 AS document_height,
	r.10 AS max_bottom,r.11 AS max_height,r.12 AS late_init,r.13 AS clipped,r.14 AS unsupported,
	r.15 AS sample_rate,r.16 AS reported_at FROM (` + scrollCanonical + `)`

func (s *Heatmap) ScrollPages(ctx context.Context, siteID uint64, from, to time.Time, filter HeatmapPageFilter) ([]heatmap.ScrollPageCount, error) {
	fields := heatmapPageFields
	where := ""
	if filter.ByKey {
		fields = strings.Replace(fields, "path,", "'/' AS path,", 1)
		where += " AND aggregation_key!=''"
	}
	args := []any{siteID, from, to}
	if filter.Key != "" {
		where += " AND aggregation_key=?"
		args = append(args, filter.Key)
	}
	if filter.Path != "" {
		where += " AND match(path,?)"
		args = append(args, "^"+strings.ReplaceAll(regexp.QuoteMeta(filter.Path), `\*`, `[^/]*`)+"$")
	}
	group := strings.Replace(fields, "'/' AS path,", "", 1)
	group = strings.Replace(group, heatmapDevice+" AS device", "device", 1)
	rows, err := s.conn.Query(ctx, `SELECT `+fields+`,count(),max(reported_at) FROM (`+scrollResolved+`) WHERE 1`+where+` GROUP BY `+group+` ORDER BY max(reported_at) DESC LIMIT 200`+heatmapQuerySettings, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []heatmap.ScrollPageCount{}
	for rows.Next() {
		var p heatmap.ScrollPageCount
		var device string
		if err := rows.Scan(&p.AggregationKey, &p.Hostname, &p.Path, &p.Version, &p.Variant, &device, &p.SampledViews, &p.LastView); err != nil {
			return nil, err
		}
		switch device {
		case "mobile":
			p.ViewportWidth = 390
		case "tablet":
			p.ViewportWidth = 768
		default:
			p.ViewportWidth = 1440
		}
		p.Page = p.Page.Representative()
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Heatmap) ScrollData(ctx context.Context, siteID uint64, page heatmap.Page, from, to time.Time, byKey bool) (heatmap.ScrollData, error) {
	out := heatmap.ScrollData{Regions: make([]heatmap.ScrollRegion, heatmap.GridRows)}
	field, value := "path", page.Path
	if byKey {
		field, value = "aggregation_key", page.AggregationKey
	}
	args := []any{siteID, from, to, page.Hostname, value, page.Version, page.Variant, page.Device()}
	where := ` WHERE hostname=? AND ` + field + `=? AND version=? AND variant=? AND ` + heatmapDevice + `=?`
	if !byKey {
		where += ` AND aggregation_key=?`
		args = append(args, page.AggregationKey)
	}
	// 25% growth is excluded from a static screenshot projection. Clipped and
	// unsupported visits remain visible in quality totals, but not in its denominator.
	source := `SELECT *,toUInt64(max_height)*4>toUInt64(document_height)*5 AS dynamic,
		NOT dynamic AND NOT clipped AND NOT unsupported AS valid FROM (` + scrollResolved + `)` + where
	hist := `sumMapIf([toUInt8(least(80,intDiv(toUInt64(max_bottom)*80,greatest(max_height,1))))],[toUInt64(1)],valid)`
	var buckets []uint8
	var counts []uint64
	var sampleRates uint64
	err := s.conn.QueryRow(ctx, `SELECT count(),countIf(valid),countIf(dynamic),countIf(clipped),countIf(unsupported),countIf(late_init),
		ifNotFinite(avgIf(max_bottom/max_height*100,valid),0),ifNotFinite(avgIf(least(1,viewport_height/document_height)*100,valid),0),
		uniqExact(sample_rate),tupleElement(`+hist+`,1),tupleElement(`+hist+`,2) FROM (`+source+`)`+heatmapQuerySettings, args...).Scan(
		&out.SampledViews, &out.ValidViews, &out.DynamicViews, &out.ClippedViews, &out.Unsupported, &out.LateInitViews,
		&out.AverageMaxDepth, &out.AverageFold, &sampleRates, &buckets, &counts)
	if err != nil {
		return out, err
	}
	out.MixedSampling = sampleRates > 1
	for i := range heatmap.GridRows {
		p := heatmap.ScrollRegion{Depth: float64(i+1) * 100 / heatmap.GridRows}
		for j, b := range buckets {
			if int(b) >= i+1 {
				p.Views += counts[j]
			}
		}
		if out.ValidViews > 0 {
			p.Rate = float64(p.Views) * 100 / float64(out.ValidViews)
		}
		out.Regions[i] = p
	}
	return out, nil
}
