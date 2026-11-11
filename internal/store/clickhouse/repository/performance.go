package repository

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	p "github.com/zenstats/zenstats/internal/performance"
)

type Performance struct{ conn clickhouse.Conn }

// Reuse the optional analytics pool; this module never adds connections to the
// core event/session pool or assumes that 202 means a completed INSERT.
func NewPerformance(heatmaps *Heatmap) *Performance { return &Performance{conn: heatmaps.conn} }
func (s *Performance) Insert(ctx context.Context, rows []p.Row) error {
	ctx = clickhouse.Context(ctx, clickhouse.WithSettings(clickhouse.Settings{"insert_deduplicate": 0}))
	batch, err := s.conn.PrepareBatch(ctx, `INSERT INTO web_vitals_reports
 (site_id,lifecycle_id,record_kind,metric_name,metric_id,navigation_started_at,report_id,revision,reported_at,
 hostname,path,release,navigation_type,device,browser,country,value,supported_metrics,init_delay_ms,background_init,clipped,sample_rate,config_version,expires)`)
	if err != nil {
		return err
	}
	defer batch.Abort()
	for _, r := range rows {
		value := float64(0)
		if r.Value != nil {
			value = *r.Value
		}
		if err := batch.Append(r.SiteID, r.LifecycleID, r.Kind, r.MetricName, r.MetricID, r.NavigationStartedAt, r.ID, r.Revision, r.ReportedAt,
			r.Hostname, r.Path, r.Release, r.NavigationType, r.Device, r.Browser, r.Country, value, r.Supported, r.InitDelayMS, r.BackgroundInit, r.Clipped, r.SampleRate, r.ConfigVersion, r.Expires); err != nil {
			return err
		}
	}
	return batch.Send()
}

// Revisions can lower a metric. Select the whole latest report, never max(value),
// before filtering; background ReplacingMergeTree merges are not required.
const vitalsResolved = `SELECT lifecycle_id,record_kind,metric_name,metric_id,
 r.1 AS navigation_started_at,r.2 AS hostname,r.3 AS path,r.4 AS release,r.5 AS navigation_type,
 r.6 AS device,r.7 AS browser,r.8 AS country,r.9 AS value,r.10 AS supported_metrics,
 r.11 AS init_delay_ms,r.12 AS background_init,r.13 AS clipped,r.14 AS sample_rate,r.15 AS config_version FROM (
 SELECT lifecycle_id,record_kind,metric_name,metric_id,
 argMax(tuple(navigation_started_at,hostname,path,release,navigation_type,device,browser,country,value,supported_metrics,
 init_delay_ms,background_init,clipped,sample_rate,config_version),revision) AS r
 FROM web_vitals_reports WHERE site_id=? AND navigation_started_at>=? AND navigation_started_at<? AND expires>now()
 GROUP BY lifecycle_id,record_kind,metric_name,metric_id)`

func performanceContexts(siteID uint64, from, to time.Time, f p.Filter) (string, []any) {
	where := ` WHERE record_kind='context'`
	args := []any{siteID, from, to}
	for _, v := range []struct{ name, value string }{{"hostname", f.Hostname}, {"release", f.Release}, {"device", f.Device}, {"browser", f.Browser}, {"country", f.Country}, {"navigation_type", f.Navigation}} {
		if v.value != "" {
			where += " AND " + v.name + "=?"
			args = append(args, v.value)
		}
	}
	if f.Path != "" {
		where += " AND match(path,?)"
		args = append(args, "^"+strings.ReplaceAll(regexp.QuoteMeta(f.Path), `\*`, `[^/]*`)+"$")
	}
	return "SELECT * FROM (" + vitalsResolved + ")" + where, args
}
func (s *Performance) Data(ctx context.Context, siteID uint64, from, to time.Time, f p.Filter, timezone string) (p.Data, error) {
	out := p.Data{Metrics: []p.Metric{}, Trend: []p.Point{}, Pages: []p.Page{}, Overall: "insufficient", Timezone: timezone, GeneratedAt: time.Now().UTC()}
	contexts, args := performanceContexts(siteID, from, to, f)
	var rates, versions uint64
	supported := make([]uint64, 5)
	err := s.conn.QueryRow(ctx, `SELECT count(),countIf(NOT background_init),countIf(init_delay_ms>2000),countIf(background_init),
 uniqExact(sample_rate),uniqExact(config_version),
 countIf(NOT background_init AND has(supported_metrics,'LCP')),countIf(NOT background_init AND has(supported_metrics,'INP')),
 countIf(NOT background_init AND has(supported_metrics,'CLS')),countIf(NOT background_init AND has(supported_metrics,'TTFB')),
 countIf(NOT background_init AND has(supported_metrics,'FCP')) FROM (`+contexts+`)`+heatmapQuerySettings, args...).Scan(
		&out.Navigations, &out.Eligible, &out.LateInit, &out.BackgroundInit, &rates, &versions, &supported[0], &supported[1], &supported[2], &supported[3], &supported[4])
	if err != nil {
		return out, err
	}
	out.MixedSampling = rates > 1
	out.MixedConfig = versions > 1
	// A metric without its lifecycle context cannot invent a navigation denominator.
	source := `SELECT c.navigation_started_at AS navigation_started_at,c.hostname AS hostname,c.path AS path,
 m.metric_name AS metric_name,m.value AS value,m.clipped AS clipped,
 NOT c.background_init AND NOT m.clipped AS valid FROM (` + vitalsResolved + `) AS m
 INNER JOIN (` + contexts + `) AS c ON m.lifecycle_id=c.lifecycle_id WHERE m.record_kind='metric'`
	metricArgs := append([]any{siteID, from, to}, args...)
	rows, err := s.conn.Query(ctx, `SELECT metric_name,countIf(valid),round(toFloat64(ifNotFinite(quantileTDigestIf(.75)(value,valid),0)),6),
 countIf(valid AND value<=multiIf(metric_name='LCP',2500,metric_name='INP',200,metric_name='CLS',.1,metric_name='FCP',1800,800)),
 countIf(valid AND value>multiIf(metric_name='LCP',4000,metric_name='INP',500,metric_name='CLS',.25,metric_name='FCP',3000,1800)),
 countIf(clipped) FROM (`+source+`) GROUP BY metric_name`+heatmapQuerySettings, metricArgs...)
	if err != nil {
		return out, err
	}
	found := map[string]p.Metric{}
	for rows.Next() {
		var m p.Metric
		var value float64
		if err := rows.Scan(&m.Name, &m.Samples, &value, &m.Good, &m.Poor, &m.Clipped); err != nil {
			rows.Close()
			return out, err
		}
		m.P75 = &value
		m.NeedsImprovement = m.Samples - m.Good - m.Poor
		found[m.Name] = m
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	out.Overall = "good"
	for i, name := range p.Names {
		m := found[name]
		m.Name = name
		m.Supported = supported[i]
		m.Complete(out.Eligible)
		out.Metrics = append(out.Metrics, m)
		if name == "LCP" || name == "INP" || name == "CLS" {
			if m.Rating == "insufficient" {
				out.Overall = "insufficient"
			} else if out.Overall != "insufficient" {
				if m.Rating == "poor" {
					out.Overall = "poor"
				} else if m.Rating == "needs_improvement" && out.Overall == "good" {
					out.Overall = "needs_improvement"
				}
			}
		}
	}
	good, poor := p.Thresholds(f.Metric)
	// Percentiles are calculated across the selected raw samples, never by
	// averaging daily/hourly P75s. The bounded raw range is at most seven days.
	trendArgs := append([]any{timezone}, metricArgs...)
	trendArgs = append(trendArgs, f.Metric)
	rows, err = s.conn.Query(ctx, `SELECT toString(toDate(navigation_started_at,?)) AS day,count(),round(toFloat64(quantileTDigest(.75)(value)),6)
 FROM (`+source+`) WHERE valid AND metric_name=? GROUP BY day ORDER BY day`+heatmapQuerySettings, trendArgs...)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var point p.Point
		var value float64
		if err := rows.Scan(&point.Day, &point.Samples, &value); err != nil {
			rows.Close()
			return out, err
		}
		point.P75 = &value
		out.Trend = append(out.Trend, point)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	pageArgs := append([]any{good, poor}, metricArgs...)
	pageArgs = append(pageArgs, f.Metric)
	rows, err = s.conn.Query(ctx, `SELECT hostname,path,count(),round(toFloat64(quantileTDigest(.75)(value)),6),countIf(value<=?),countIf(value>?)
 FROM (`+source+`) WHERE valid AND metric_name=? GROUP BY hostname,path ORDER BY quantileTDigest(.75)(value) DESC,hostname,path LIMIT 50`+heatmapQuerySettings, pageArgs...)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var page p.Page
		var value float64
		if err := rows.Scan(&page.Hostname, &page.Path, &page.Samples, &value, &page.Good, &page.Poor); err != nil {
			rows.Close()
			return out, err
		}
		page.P75 = &value
		page.NeedsImprovement = page.Samples - page.Good - page.Poor
		out.Pages = append(out.Pages, page)
	}
	err = rows.Err()
	rows.Close()
	return out, err
}
