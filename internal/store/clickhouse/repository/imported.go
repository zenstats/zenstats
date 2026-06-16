package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	cl "github.com/zenstats/zenstats/internal/store/clickhouse"
	"github.com/zenstats/zenstats/internal/store/clickhouse/models"
)

var (
	importedInstance *ImportedRepository
)

type ImportedRepository struct {
	conn clickhouse.Conn
}

func GetImportedRepository() *ImportedRepository {
	importOnce.Do(func() {
		conn := cl.GetConnection()
		if conn == nil {
			return
		}
		importedInstance = &ImportedRepository{conn: conn}
	})
	return importedInstance
}

func (r *ImportedRepository) InsertVisitors(ctx context.Context, rows []*models.ImportedVisitors) error {
	batch, err := r.conn.PrepareBatch(ctx, `INSERT INTO zenstats_events_db.imported_visitors (
		site_id, date, visitors, pageviews, bounces, visits, visit_duration, import_id
	)`)
	if err != nil {
		return fmt.Errorf("prepare batch: %w", err)
	}
	for _, row := range rows {
		if err := batch.Append(row.SiteID, row.Date, row.Visitors, row.Pageviews, row.Bounces, row.Visits, row.VisitDuration, row.ImportID); err != nil {
			return fmt.Errorf("append: %w", err)
		}
	}
	return batch.Send()
}

func (r *ImportedRepository) InsertSources(ctx context.Context, rows []*models.ImportedSource) error {
	batch, err := r.conn.PrepareBatch(ctx, `INSERT INTO zenstats_events_db.imported_sources (
		site_id, date, source, utm_medium, utm_campaign, utm_content, utm_term,
		visitors, visits, visit_duration, bounces, import_id, pageviews, referrer, utm_source
	)`)
	if err != nil {
		return fmt.Errorf("prepare batch: %w", err)
	}
	for _, row := range rows {
		if err := batch.Append(
			row.SiteID, row.Date, row.Source, row.UtmMedium, row.UtmCampaign, row.UtmContent, row.UtmTerm,
			row.Visitors, row.Visits, row.VisitDuration, row.Bounces, row.ImportID, row.Pageviews, row.Referrer, row.UtmSource,
		); err != nil {
			return fmt.Errorf("append: %w", err)
		}
	}
	return batch.Send()
}

func (r *ImportedRepository) InsertPages(ctx context.Context, rows []*models.ImportedPage) error {
	batch, err := r.conn.PrepareBatch(ctx, `INSERT INTO zenstats_events_db.imported_pages (
		site_id, date, hostname, page, visitors, pageviews, exits, time_on_page, import_id, visits, active_visitors
	)`)
	if err != nil {
		return fmt.Errorf("prepare batch: %w", err)
	}
	for _, row := range rows {
		if err := batch.Append(row.SiteID, row.Date, row.Hostname, row.Page, row.Visitors, row.Pageviews, row.Exits, row.TimeOnPage, row.ImportID, row.Visits, row.ActiveVisitors); err != nil {
			return fmt.Errorf("append: %w", err)
		}
	}
	return batch.Send()
}

func (r *ImportedRepository) InsertOS(ctx context.Context, rows []*models.ImportedOS) error {
	batch, err := r.conn.PrepareBatch(ctx, `INSERT INTO zenstats_events_db.imported_operating_systems (
		site_id, date, operating_system, visitors, visits, visit_duration, bounces, import_id, pageviews, operating_system_version
	)`)
	if err != nil {
		return fmt.Errorf("prepare batch: %w", err)
	}
	for _, row := range rows {
		if err := batch.Append(row.SiteID, row.Date, row.OperatingSystem, row.Visitors, row.Visits, row.VisitDuration, row.Bounces, row.ImportID, row.Pageviews, row.OperatingSystemVersion); err != nil {
			return fmt.Errorf("append: %w", err)
		}
	}
	return batch.Send()
}

func (r *ImportedRepository) InsertLocations(ctx context.Context, rows []*models.ImportedLocation) error {
	batch, err := r.conn.PrepareBatch(ctx, `INSERT INTO zenstats_events_db.imported_locations (
		site_id, date, country, region, city, visitors, visits, visit_duration, bounces, import_id, pageviews
	)`)
	if err != nil {
		return fmt.Errorf("prepare batch: %w", err)
	}
	for _, row := range rows {
		if err := batch.Append(row.SiteID, row.Date, row.Country, row.Region, row.City, row.Visitors, row.Visits, row.VisitDuration, row.Bounces, row.ImportID, row.Pageviews); err != nil {
			return fmt.Errorf("append: %w", err)
		}
	}
	return batch.Send()
}

func (r *ImportedRepository) InsertExitPages(ctx context.Context, rows []*models.ImportedExitPage) error {
	batch, err := r.conn.PrepareBatch(ctx, `INSERT INTO zenstats_events_db.imported_exit_pages (
		site_id, date, exit_page, visitors, exits, import_id, pageviews, bounces, visit_duration
	)`)
	if err != nil {
		return fmt.Errorf("prepare batch: %w", err)
	}
	for _, row := range rows {
		if err := batch.Append(row.SiteID, row.Date, row.ExitPage, row.Visitors, row.Exits, row.ImportID, row.Pageviews, row.Bounces, row.VisitDuration); err != nil {
			return fmt.Errorf("append: %w", err)
		}
	}
	return batch.Send()
}

func (r *ImportedRepository) InsertEntryPages(ctx context.Context, rows []*models.ImportedEntryPage) error {
	batch, err := r.conn.PrepareBatch(ctx, `INSERT INTO zenstats_events_db.imported_entry_pages (
		site_id, date, entry_page, visitors, entrances, visit_duration, bounces, import_id, pageviews
	)`)
	if err != nil {
		return fmt.Errorf("prepare batch: %w", err)
	}
	for _, row := range rows {
		if err := batch.Append(row.SiteID, row.Date, row.EntryPage, row.Visitors, row.Entrances, row.VisitDuration, row.Bounces, row.ImportID, row.Pageviews); err != nil {
			return fmt.Errorf("append: %w", err)
		}
	}
	return batch.Send()
}

func (r *ImportedRepository) InsertDevices(ctx context.Context, rows []*models.ImportedDevice) error {
	batch, err := r.conn.PrepareBatch(ctx, `INSERT INTO zenstats_events_db.imported_devices (
		site_id, date, device, visitors, visits, visit_duration, bounces, import_id, pageviews
	)`)
	if err != nil {
		return fmt.Errorf("prepare batch: %w", err)
	}
	for _, row := range rows {
		if err := batch.Append(row.SiteID, row.Date, row.Device, row.Visitors, row.Visits, row.VisitDuration, row.Bounces, row.ImportID, row.Pageviews); err != nil {
			return fmt.Errorf("append: %w", err)
		}
	}
	return batch.Send()
}

func (r *ImportedRepository) InsertCustomEvents(ctx context.Context, rows []*models.ImportedCustomEvent) error {
	batch, err := r.conn.PrepareBatch(ctx, `INSERT INTO zenstats_events_db.imported_custom_events (
		site_id, import_id, date, name, link_url, path, visitors, events
	)`)
	if err != nil {
		return fmt.Errorf("prepare batch: %w", err)
	}
	for _, row := range rows {
		if err := batch.Append(row.SiteID, row.ImportID, row.Date, row.Name, row.LinkURL, row.Path, row.Visitors, row.Events); err != nil {
			return fmt.Errorf("append: %w", err)
		}
	}
	return batch.Send()
}

func (r *ImportedRepository) InsertBrowsers(ctx context.Context, rows []*models.ImportedBrowser) error {
	batch, err := r.conn.PrepareBatch(ctx, `INSERT INTO zenstats_events_db.imported_browsers (
		site_id, date, browser, visitors, visits, visit_duration, bounces, import_id, pageviews, browser_version
	)`)
	if err != nil {
		return fmt.Errorf("prepare batch: %w", err)
	}
	for _, row := range rows {
		if err := batch.Append(row.SiteID, row.Date, row.Browser, row.Visitors, row.Visits, row.VisitDuration, row.Bounces, row.ImportID, row.Pageviews, row.BrowserVersion); err != nil {
			return fmt.Errorf("append: %w", err)
		}
	}
	return batch.Send()
}

type ImportedAggregate struct {
	Visitors      uint64  `json:"visitors"`
	Pageviews     uint64  `json:"pageviews"`
	Visits        uint64  `json:"visits"`
	Bounces       uint64  `json:"bounces"`
	BounceRate    float64 `json:"bounce_rate"`
	VisitDuration uint64  `json:"visit_duration"`
	ViewsPerVisit float64 `json:"views_per_visit"`
}

func (r *ImportedRepository) QueryAggregate(ctx context.Context, siteID uint64, start, end time.Time) (*ImportedAggregate, error) {
	var result ImportedAggregate
	err := r.conn.QueryRow(ctx, `
		SELECT
			coalesce(sum(visitors), 0),
			coalesce(sum(pageviews), 0),
			coalesce(sum(visits), 0),
			coalesce(sum(bounces), 0),
			coalesce(avg(visit_duration), 0)
		FROM zenstats_events_db.imported_visitors
		WHERE site_id = ? AND date >= ? AND date <= ?
	`, siteID, start, end).Scan(
		&result.Visitors, &result.Pageviews, &result.Visits, &result.Bounces, &result.VisitDuration,
	)
	if err != nil {
		return nil, fmt.Errorf("query aggregate: %w", err)
	}
	if result.Visits > 0 {
		result.BounceRate = float64(result.Bounces) / float64(result.Visits) * 100
		result.ViewsPerVisit = float64(result.Pageviews) / float64(result.Visits)
	}
	return &result, nil
}

type ImportedBreakdownRow struct {
	Name     string  `json:"name"`
	Visitors uint64  `json:"visitors"`
	Pageviews uint64 `json:"pageviews,omitempty"`
	Visits   uint64  `json:"visits,omitempty"`
}

func (r *ImportedRepository) QueryBreakdown(ctx context.Context, siteID uint64, start, end time.Time, tableName, dimColumn string, limit, offset int) ([]ImportedBreakdownRow, error) {
	sql := fmt.Sprintf(`
		SELECT %s, sum(visitors), sum(pageviews), sum(visits)
		FROM zenstats_events_db.%s
		WHERE site_id = ? AND date >= ? AND date <= ? AND %s != ''
		GROUP BY %s
		ORDER BY sum(visitors) DESC
		LIMIT ? OFFSET ?
	`, dimColumn, tableName, dimColumn, dimColumn)

	rows, err := r.conn.Query(ctx, sql, siteID, start, end, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query breakdown: %w", err)
	}
	defer rows.Close()

	var result []ImportedBreakdownRow
	for rows.Next() {
		var row ImportedBreakdownRow
		if err := rows.Scan(&row.Name, &row.Visitors, &row.Pageviews, &row.Visits); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		result = append(result, row)
	}
	return result, nil
}

type ImportedTimeSeriesPoint struct {
	Date      string `json:"date"`
	Visitors  uint64 `json:"visitors"`
	Pageviews uint64 `json:"pageviews"`
}

func (r *ImportedRepository) QueryTimeSeries(ctx context.Context, siteID uint64, start, end time.Time) ([]ImportedTimeSeriesPoint, error) {
	rows, err := r.conn.Query(ctx, `
		SELECT toDate(date) as d, sum(visitors), sum(pageviews)
		FROM zenstats_events_db.imported_visitors
		WHERE site_id = ? AND date >= ? AND date <= ?
		GROUP BY d
		ORDER BY d
	`, siteID, start, end)
	if err != nil {
		return nil, fmt.Errorf("query time series: %w", err)
	}
	defer rows.Close()

	var result []ImportedTimeSeriesPoint
	for rows.Next() {
		var p ImportedTimeSeriesPoint
		var d time.Time
		if err := rows.Scan(&d, &p.Visitors, &p.Pageviews); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		p.Date = d.Format("2006-01-02")
		result = append(result, p)
	}
	return result, nil
}

func (r *ImportedRepository) TableForProperty(property string) (tableName, dimColumn string, ok bool) {
	switch property {
	case "visit:source":
		return "imported_sources", "source", true
	case "visit:country":
		return "imported_locations", "country", true
	case "visit:region":
		return "imported_locations", "region", true
	case "visit:city":
		return "imported_locations", "city", true
	case "visit:browser":
		return "imported_browsers", "browser", true
	case "visit:os":
		return "imported_operating_systems", "operating_system", true
	case "visit:device":
		return "imported_devices", "device", true
	case "visit:entry_page":
		return "imported_entry_pages", "entry_page", true
	case "visit:exit_page":
		return "imported_exit_pages", "exit_page", true
	case "event:page":
		return "imported_pages", "page", true
	case "event:hostname":
		return "imported_pages", "hostname", true
	default:
		return "", "", false
	}
}
