package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/zenstats/zenstats/internal/store/clickhouse/models"
	"github.com/zenstats/zenstats/internal/store/clickhouse/repository"
)

var (
	importServiceInstance *ImportService
	importServiceOnce     sync.Once
)

type ImportService struct {
	repo *repository.ImportedRepository
}

func GetImportService() *ImportService {
	importServiceOnce.Do(func() {
		importServiceInstance = &ImportService{
			repo: repository.GetImportedRepository(),
		}
	})
	return importServiceInstance
}

func (s *ImportService) NextImportID() uint64 {
	return uint64(time.Now().UnixMilli())
}

func (s *ImportService) InsertVisitors(ctx context.Context, rows []*models.ImportedVisitors) error {
	return s.repo.InsertVisitors(ctx, rows)
}

func (s *ImportService) InsertSources(ctx context.Context, rows []*models.ImportedSource) error {
	return s.repo.InsertSources(ctx, rows)
}

func (s *ImportService) InsertPages(ctx context.Context, rows []*models.ImportedPage) error {
	return s.repo.InsertPages(ctx, rows)
}

func (s *ImportService) InsertOS(ctx context.Context, rows []*models.ImportedOS) error {
	return s.repo.InsertOS(ctx, rows)
}

func (s *ImportService) InsertLocations(ctx context.Context, rows []*models.ImportedLocation) error {
	return s.repo.InsertLocations(ctx, rows)
}

func (s *ImportService) InsertExitPages(ctx context.Context, rows []*models.ImportedExitPage) error {
	return s.repo.InsertExitPages(ctx, rows)
}

func (s *ImportService) InsertEntryPages(ctx context.Context, rows []*models.ImportedEntryPage) error {
	return s.repo.InsertEntryPages(ctx, rows)
}

func (s *ImportService) InsertDevices(ctx context.Context, rows []*models.ImportedDevice) error {
	return s.repo.InsertDevices(ctx, rows)
}

func (s *ImportService) InsertCustomEvents(ctx context.Context, rows []*models.ImportedCustomEvent) error {
	return s.repo.InsertCustomEvents(ctx, rows)
}

func (s *ImportService) InsertBrowsers(ctx context.Context, rows []*models.ImportedBrowser) error {
	return s.repo.InsertBrowsers(ctx, rows)
}

type insertFunc func(ctx context.Context, rows any) error

func (s *ImportService) InsertByTable(ctx context.Context, table string, rows any) error {
	fn, ok := s.insertFuncMap()[table]
	if !ok {
		return fmt.Errorf("unknown imported table: %s", table)
	}
	return fn(ctx, rows)
}

func (s *ImportService) insertFuncMap() map[string]insertFunc {
	return map[string]insertFunc{
		"imported_visitors":           func(ctx context.Context, rows any) error { return s.InsertVisitors(ctx, rows.([]*models.ImportedVisitors)) },
		"imported_sources":            func(ctx context.Context, rows any) error { return s.InsertSources(ctx, rows.([]*models.ImportedSource)) },
		"imported_pages":              func(ctx context.Context, rows any) error { return s.InsertPages(ctx, rows.([]*models.ImportedPage)) },
		"imported_operating_systems":  func(ctx context.Context, rows any) error { return s.InsertOS(ctx, rows.([]*models.ImportedOS)) },
		"imported_locations":          func(ctx context.Context, rows any) error { return s.InsertLocations(ctx, rows.([]*models.ImportedLocation)) },
		"imported_exit_pages":         func(ctx context.Context, rows any) error { return s.InsertExitPages(ctx, rows.([]*models.ImportedExitPage)) },
		"imported_entry_pages":        func(ctx context.Context, rows any) error { return s.InsertEntryPages(ctx, rows.([]*models.ImportedEntryPage)) },
		"imported_devices":            func(ctx context.Context, rows any) error { return s.InsertDevices(ctx, rows.([]*models.ImportedDevice)) },
		"imported_custom_events":      func(ctx context.Context, rows any) error { return s.InsertCustomEvents(ctx, rows.([]*models.ImportedCustomEvent)) },
		"imported_browsers":           func(ctx context.Context, rows any) error { return s.InsertBrowsers(ctx, rows.([]*models.ImportedBrowser)) },
	}
}

func (s *ImportService) QueryAggregate(ctx context.Context, siteID uint64, start, end time.Time) (*repository.ImportedAggregate, error) {
	return s.repo.QueryAggregate(ctx, siteID, start, end)
}

func (s *ImportService) QueryBreakdown(ctx context.Context, siteID uint64, start, end time.Time, property string, limit, offset int) ([]repository.ImportedBreakdownRow, error) {
	table, column, ok := s.repo.TableForProperty(property)
	if !ok {
		return nil, fmt.Errorf("unsupported breakdown property: %s", property)
	}
	return s.repo.QueryBreakdown(ctx, siteID, start, end, table, column, limit, offset)
}

func (s *ImportService) QueryTimeSeries(ctx context.Context, siteID uint64, start, end time.Time) ([]repository.ImportedTimeSeriesPoint, error) {
	return s.repo.QueryTimeSeries(ctx, siteID, start, end)
}

func (s *ImportService) TableNameForReport(reportType string) string {
	m := map[string]string{
		"visitors":    "imported_visitors",
		"pages":       "imported_pages",
		"sources":     "imported_sources",
		"browsers":    "imported_browsers",
		"os":          "imported_operating_systems",
		"devices":     "imported_devices",
		"locations":   "imported_locations",
		"entry_pages": "imported_entry_pages",
		"exit_pages":  "imported_exit_pages",
	}
	if t, ok := m[reportType]; ok {
		return t
	}
	return "imported_visitors"
}
