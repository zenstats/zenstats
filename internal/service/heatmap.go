package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/zenstats/zenstats/config"
	"github.com/zenstats/zenstats/internal/heatmap"
	cl "github.com/zenstats/zenstats/internal/store/clickhouse"
	"github.com/zenstats/zenstats/internal/store/clickhouse/repository"
	"github.com/zenstats/zenstats/internal/store/postgresql"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent"
	hcfg "github.com/zenstats/zenstats/internal/store/postgresql/ent/heatmapconfig"
	hsnap "github.com/zenstats/zenstats/internal/store/postgresql/ent/heatmapsnapshot"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent/sitemembership"
	"github.com/zenstats/zenstats/pkg/globals"
)

var ErrHeatmapNotFound = errors.New("heatmap snapshot not found")
var ErrHeatmapInvalid = errors.New("invalid heatmap snapshot")

type HeatmapService struct {
	db        *postgresql.Client
	Repo      *repository.Heatmap
	Collector *heatmap.Collector
	configs   *expirable.LRU[int64, heatmap.Config]
	directory string
	queries   chan struct{}
	captures  chan struct{}
	filesMu   sync.Mutex
	closeConn func() error
}

var GetHeatmapService = sync.OnceValue(func() *HeatmapService {
	conn, err := cl.NewHeatmapConnection()
	if err != nil {
		panic(err)
	}
	s := NewHeatmapService(globals.GetDB(), repository.NewHeatmap(conn), config.Conf.DataPath)
	s.closeConn = conn.Close
	return s
})

func (s *HeatmapService) Shutdown() {
	s.Collector.Shutdown()
	if s.closeConn != nil {
		_ = s.closeConn()
	}
}

func NewHeatmapService(db *postgresql.Client, repo *repository.Heatmap, dataPath string) *HeatmapService {
	return &HeatmapService{db: db, Repo: repo, Collector: heatmap.NewCollector(repo),
		configs:   expirable.NewLRU[int64, heatmap.Config](1000, nil, 10*time.Second),
		directory: filepath.Join(dataPath, "heatmaps"), queries: make(chan struct{}, 2), captures: make(chan struct{}, 1)}
}

func configFromEntity(r *ent.HeatmapConfig) heatmap.Config {
	return heatmap.Config{Enabled: r.Enabled, Paths: r.Paths, Groups: r.Groups, SampleRate: r.SampleRate, MaxClicks: r.MaxClicks, RawDays: r.RawDays, AggregateDays: r.AggregateDays, Timezone: r.Timezone}
}

func (s *HeatmapService) CanWrite(ctx context.Context, siteID, userID int64) (bool, error) {
	return s.db.Client.SiteMembership.Query().Where(sitemembership.SiteID(siteID), sitemembership.UserID(userID),
		sitemembership.RoleIn(sitemembership.RoleOwner, sitemembership.RoleAdmin)).Exist(ctx)
}

func (s *HeatmapService) Config(ctx context.Context, siteID int64) (heatmap.Config, error) {
	if cfg, ok := s.configs.Get(siteID); ok {
		return cfg, nil
	}
	r, err := s.db.Client.HeatmapConfig.Query().Where(hcfg.SiteID(siteID)).Only(ctx)
	if ent.IsNotFound(err) {
		cfg := heatmap.DefaultConfig()
		s.configs.Add(siteID, cfg)
		return cfg, nil
	}
	if err != nil {
		return heatmap.Config{}, err
	}
	cfg := configFromEntity(r)
	if err := cfg.Validate(); err != nil {
		return heatmap.Config{}, err
	}
	s.configs.Add(siteID, cfg)
	return cfg, nil
}

func (s *HeatmapService) SaveConfig(ctx context.Context, siteID int64, cfg heatmap.Config) (heatmap.Config, error) {
	siteForRules, err := s.db.Client.Site.Get(ctx, siteID)
	if err != nil {
		return cfg, err
	}
	for _, g := range cfg.Groups {
		if g.ExampleURL != "" {
			u, e := url.Parse(g.ExampleURL)
			if e != nil || !heatmap.HostAllowed(u.Hostname(), siteForRules.Domain, siteForRules.AllowedOrigins) {
				return cfg, fmt.Errorf("%w: example URL must belong to this site", ErrHeatmapInvalid)
			}
		}
	}
	r, err := s.db.Client.HeatmapConfig.Query().Where(hcfg.SiteID(siteID)).Only(ctx)
	if ent.IsNotFound(err) {
		site, err := s.db.Client.Site.Get(ctx, siteID)
		if err != nil {
			return cfg, err
		}
		cfg.Timezone = site.Timezone
		if err := cfg.Validate(); err != nil {
			return cfg, err
		}
		r, err = s.db.Client.HeatmapConfig.Create().SetSiteID(siteID).SetEnabled(cfg.Enabled).SetPaths(cfg.Paths).SetGroups(cfg.Groups).
			SetSampleRate(cfg.SampleRate).SetMaxClicks(cfg.MaxClicks).SetRawDays(cfg.RawDays).SetAggregateDays(cfg.AggregateDays).SetTimezone(cfg.Timezone).Save(ctx)
		if err != nil {
			return cfg, err
		}
	} else {
		if err != nil {
			return cfg, err
		}
		// Freeze the bucket timezone once configured. Changing the site's timezone
		// must not silently reinterpret retained hourly aggregates.
		cfg.Timezone = r.Timezone
		if err := cfg.Validate(); err != nil {
			return cfg, err
		}
		r, err = s.db.Client.HeatmapConfig.UpdateOne(r).SetEnabled(cfg.Enabled).SetPaths(cfg.Paths).SetGroups(cfg.Groups).
			SetSampleRate(cfg.SampleRate).SetMaxClicks(cfg.MaxClicks).SetRawDays(cfg.RawDays).SetAggregateDays(cfg.AggregateDays).Save(ctx)
		if err != nil {
			return cfg, err
		}
	}
	cfg = configFromEntity(r)
	s.configs.Remove(siteID)
	return cfg, nil
}

// DateRange accepts dates in the heatmap's immutable site timezone. The exclusive
// upper bound follows local midnight, including DST and fractional UTC offsets.
func (s *HeatmapService) DateRange(ctx context.Context, siteID int64, start, end string) (time.Time, time.Time, error) {
	cfg, err := s.Config(ctx, siteID)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	from, err := time.ParseInLocation("2006-01-02", start, loc)
	if err != nil {
		return from, from, errors.New("invalid start date")
	}
	last, err := time.ParseInLocation("2006-01-02", end, loc)
	if err != nil {
		return from, last, errors.New("invalid end date")
	}
	to := last.AddDate(0, 0, 1)
	if last.Before(from) || !last.Before(from.AddDate(0, 0, 180)) {
		return from, to, errors.New("date range must be within 180 days")
	}
	return from.UTC(), to.UTC(), nil
}

func (s *HeatmapService) QuerySlot(ctx context.Context) (func(), error) {
	select {
	case s.queries <- struct{}{}:
		return func() { <-s.queries }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

type HeatmapSnapshot struct {
	ID        int64  `json:"id"`
	SourceURL string `json:"source_url"`
	heatmap.Page
	ImageWidth  int       `json:"image_width"`
	ImageHeight int       `json:"image_height"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at"`
}

func snapshotDTO(r *ent.HeatmapSnapshot) HeatmapSnapshot {
	return HeatmapSnapshot{ID: r.ID, SourceURL: r.SourceURL, Page: heatmap.Page{AggregationKey: r.AggregationKey, Hostname: r.Hostname, Path: r.Path, Version: r.Version, Variant: r.Variant,
		ViewportWidth: uint16(r.ViewportWidth), ViewportHeight: uint16(r.ViewportHeight), DocumentWidth: uint32(r.DocumentWidth), DocumentHeight: uint32(r.DocumentHeight)},
		ImageWidth: r.ImageWidth, ImageHeight: r.ImageHeight, CreatedAt: r.CreatedAt, ExpiresAt: r.ExpiresAt}
}

func (s *HeatmapService) Snapshots(ctx context.Context, siteID int64) ([]HeatmapSnapshot, error) {
	rows, err := s.db.Client.HeatmapSnapshot.Query().Where(hsnap.SiteID(siteID), hsnap.ExpiresAtGT(time.Now())).Order(ent.Desc(hsnap.FieldCreatedAt)).Limit(100).All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]HeatmapSnapshot, 0, len(rows))
	for _, r := range rows {
		out = append(out, snapshotDTO(r))
	}
	return out, nil
}

func (s *HeatmapService) Snapshot(ctx context.Context, siteID, id int64) (*ent.HeatmapSnapshot, error) {
	r, err := s.db.Client.HeatmapSnapshot.Query().Where(hsnap.SiteID(siteID), hsnap.ID(id), hsnap.ExpiresAtGT(time.Now())).Only(ctx)
	if ent.IsNotFound(err) {
		return nil, ErrHeatmapNotFound
	}
	return r, err
}

func (s *HeatmapService) SnapshotFile(row *ent.HeatmapSnapshot) (*os.File, error) {
	if filepath.Base(row.FileKey) != row.FileKey {
		return nil, ErrHeatmapNotFound
	}
	return os.Open(filepath.Join(s.directory, row.FileKey))
}

func (s *HeatmapService) CreateSnapshot(ctx context.Context, siteID int64, page heatmap.Page, reader io.Reader, sourceURL ...string) (HeatmapSnapshot, error) {
	if err := page.Validate(); err != nil {
		return HeatmapSnapshot{}, fmt.Errorf("%w: %v", ErrHeatmapInvalid, err)
	}
	site, err := s.db.Client.Site.Get(ctx, siteID)
	if err != nil {
		return HeatmapSnapshot{}, err
	}
	if !heatmap.HostAllowed(page.Hostname, site.Domain, site.AllowedOrigins) {
		return HeatmapSnapshot{}, fmt.Errorf("%w: unrelated hostname", ErrHeatmapInvalid)
	}
	s.filesMu.Lock()
	defer s.filesMu.Unlock()
	n, err := s.db.Client.HeatmapSnapshot.Query().Where(hsnap.SiteID(siteID), hsnap.ExpiresAtGT(time.Now())).Count(ctx)
	if err != nil {
		return HeatmapSnapshot{}, err
	}
	if n >= 100 {
		return HeatmapSnapshot{}, fmt.Errorf("%w: snapshot limit reached (100 per site)", ErrHeatmapInvalid)
	}
	body, err := io.ReadAll(io.LimitReader(reader, (5<<20)+1))
	if err != nil {
		return HeatmapSnapshot{}, err
	}
	if len(body) > 5<<20 {
		return HeatmapSnapshot{}, fmt.Errorf("%w: snapshot must be at most 5 MiB", ErrHeatmapInvalid)
	}
	imageCfg, format, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil || (format != "png" && format != "jpeg") || imageCfg.Width < 1 || imageCfg.Height < 1 || imageCfg.Width > 8192 || imageCfg.Height > 100000 || int64(imageCfg.Width)*int64(imageCfg.Height) > 25000000 {
		return HeatmapSnapshot{}, fmt.Errorf("%w: invalid PNG/JPEG or image dimensions exceed 25 megapixels", ErrHeatmapInvalid)
	}
	// Uploaded backgrounds identify a device and version, not an exact recorded
	// document size. Infer their CSS aspect at that device's viewport width.
	// Automatic captures already supply the actual browser document dimensions.
	if len(sourceURL) == 0 {
		page.DocumentWidth = uint32(page.ViewportWidth)
		page.DocumentHeight = uint32(math.Round(float64(page.DocumentWidth) * float64(imageCfg.Height) / float64(imageCfg.Width)))
		if page.DocumentHeight < uint32(page.ViewportHeight) {
			page.ViewportHeight = uint16(page.DocumentHeight)
		}
		if err := page.Validate(); err != nil {
			return HeatmapSnapshot{}, fmt.Errorf("%w: invalid full-page image proportions", ErrHeatmapInvalid)
		}
	}
	expectedHeight := float64(imageCfg.Width) * float64(page.DocumentHeight) / float64(page.DocumentWidth)
	rounding := math.Max(2, float64(imageCfg.Width)/float64(page.DocumentWidth))
	if math.Abs(float64(imageCfg.Height)-expectedHeight) > rounding {
		return HeatmapSnapshot{}, fmt.Errorf("%w: image aspect ratio does not match document dimensions", ErrHeatmapInvalid)
	}
	if err := os.MkdirAll(s.directory, 0700); err != nil {
		return HeatmapSnapshot{}, err
	}
	key := uuid.NewString() + "." + format
	p := filepath.Join(s.directory, key)
	if err := os.WriteFile(p, body, 0600); err != nil {
		return HeatmapSnapshot{}, err
	}
	cfg, err := s.Config(ctx, siteID)
	if err != nil {
		os.Remove(p)
		return HeatmapSnapshot{}, err
	}
	source := ""
	if len(sourceURL) > 0 {
		source = sourceURL[0]
	}
	r, err := s.db.Client.HeatmapSnapshot.Create().SetSourceURL(source).SetSiteID(siteID).SetHostname(page.Hostname).SetPath(page.Path).SetAggregationKey(page.AggregationKey).SetVersion(page.Version).SetVariant(page.Variant).
		SetViewportWidth(int(page.ViewportWidth)).SetViewportHeight(int(page.ViewportHeight)).SetDocumentWidth(int(page.DocumentWidth)).SetDocumentHeight(int(page.DocumentHeight)).
		SetImageWidth(imageCfg.Width).SetImageHeight(imageCfg.Height).SetFileKey(key).SetContentType("image/" + format).SetFileSize(len(body)).
		SetExpiresAt(time.Now().Add(time.Duration(cfg.AggregateDays) * 24 * time.Hour)).Save(ctx)
	if err != nil {
		os.Remove(p)
		return HeatmapSnapshot{}, err
	}
	return snapshotDTO(r), nil
}

func (s *HeatmapService) DeleteSnapshot(ctx context.Context, siteID, id int64) error {
	r, err := s.db.Client.HeatmapSnapshot.Query().Where(hsnap.SiteID(siteID), hsnap.ID(id)).Only(ctx)
	if ent.IsNotFound(err) {
		return ErrHeatmapNotFound
	}
	if err != nil {
		return err
	}
	if err := s.db.Client.HeatmapSnapshot.DeleteOne(r).Exec(ctx); err != nil {
		return err
	}
	if filepath.Base(r.FileKey) != r.FileKey {
		return fmt.Errorf("invalid snapshot file key")
	}
	err = os.Remove(filepath.Join(s.directory, r.FileKey))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// CleanupSnapshots removes expired metadata and orphaned files (including sites
// deleted through PostgreSQL cascading). It only examines this feature's folder.
func (s *HeatmapService) CleanupSnapshots(ctx context.Context) error {
	s.filesMu.Lock()
	defer s.filesMu.Unlock()
	rows, err := s.db.Client.HeatmapSnapshot.Query().Where(hsnap.ExpiresAtLTE(time.Now())).Limit(100).All(ctx)
	if err != nil {
		return err
	}
	for _, r := range rows {
		if err := s.DeleteSnapshot(ctx, r.SiteID, r.ID); err != nil {
			return err
		}
	}
	files, err := os.ReadDir(s.directory)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, f := range files {
		if err := ctx.Err(); err != nil {
			return err
		}
		if f.IsDir() {
			continue
		}
		info, err := f.Info()
		if err != nil {
			return err
		}
		if time.Since(info.ModTime()) < time.Hour {
			continue
		}
		exists, err := s.db.Client.HeatmapSnapshot.Query().Where(hsnap.FileKey(f.Name())).Exist(ctx)
		if err != nil {
			return err
		}
		if !exists {
			if err := os.Remove(filepath.Join(s.directory, f.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}
