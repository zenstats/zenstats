package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/zenstats/zenstats/internal/heatmap"
	p "github.com/zenstats/zenstats/internal/performance"
	"github.com/zenstats/zenstats/internal/store/clickhouse/repository"
	"github.com/zenstats/zenstats/internal/store/postgresql"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent"
	cfgtable "github.com/zenstats/zenstats/internal/store/postgresql/ent/siteperformanceconfig"
	"github.com/zenstats/zenstats/pkg/globals"
)

var ErrPerformanceInvalid = errors.New("invalid performance configuration")

type PerformanceService struct {
	db        *postgresql.Client
	Shared    *HeatmapService
	Repo      *repository.Performance
	Collector *heatmap.BatchCollector[p.Row]
	configs   *expirable.LRU[int64, p.Config]
	results   *expirable.LRU[string, p.Data]
}

var GetPerformanceService = sync.OnceValue(func() *PerformanceService { return NewPerformanceService(globals.GetDB(), GetHeatmapService()) })

func NewPerformanceService(db *postgresql.Client, shared *HeatmapService) *PerformanceService {
	repo := repository.NewPerformance(shared.Repo)
	return &PerformanceService{db: db, Shared: shared, Repo: repo, Collector: heatmap.NewBatchCollector[p.Row](repo), configs: expirable.NewLRU[int64, p.Config](1000, nil, 10*time.Second), results: expirable.NewLRU[string, p.Data](100, nil, 5*time.Second)}
}
func (s *PerformanceService) Shutdown() { s.Collector.Shutdown() }
func performanceConfig(r *ent.SitePerformanceConfig) p.Config {
	return p.Config{Enabled: r.Enabled, Paths: r.Paths, SampleRate: r.SampleRate, RawDays: r.RawDays, Timezone: r.Timezone, Version: r.ConfigVersion}
}
func (s *PerformanceService) Config(ctx context.Context, siteID int64) (p.Config, error) {
	if c, ok := s.configs.Get(siteID); ok {
		return c, nil
	}
	r, err := s.db.Client.SitePerformanceConfig.Query().Where(cfgtable.SiteID(siteID)).Only(ctx)
	if ent.IsNotFound(err) {
		c := p.DefaultConfig()
		site, e := s.db.Client.Site.Get(ctx, siteID)
		if e != nil {
			return c, e
		}
		c.Timezone = site.Timezone
		s.configs.Add(siteID, c)
		return c, nil
	}
	if err != nil {
		return p.Config{}, err
	}
	c := performanceConfig(r)
	if err = c.Validate(); err != nil {
		return c, err
	}
	s.configs.Add(siteID, c)
	return c, nil
}
func (s *PerformanceService) SaveConfig(ctx context.Context, siteID int64, c p.Config) (p.Config, error) {
	r, err := s.db.Client.SitePerformanceConfig.Query().Where(cfgtable.SiteID(siteID)).Only(ctx)
	if ent.IsNotFound(err) {
		site, e := s.db.Client.Site.Get(ctx, siteID)
		if e != nil {
			return c, e
		}
		c.Timezone = site.Timezone
		if err := c.Validate(); err != nil {
			return c, fmt.Errorf("%w: %v", ErrPerformanceInvalid, err)
		}
		r, err = s.db.Client.SitePerformanceConfig.Create().SetSiteID(siteID).SetEnabled(c.Enabled).SetPaths(c.Paths).SetSampleRate(c.SampleRate).SetRawDays(c.RawDays).SetTimezone(c.Timezone).SetConfigVersion(1).Save(ctx)
	} else {
		if err != nil {
			return c, err
		}
		c.Timezone = r.Timezone
		if err := c.Validate(); err != nil {
			return c, fmt.Errorf("%w: %v", ErrPerformanceInvalid, err)
		}
		r, err = s.db.Client.SitePerformanceConfig.UpdateOne(r).SetEnabled(c.Enabled).SetPaths(c.Paths).SetSampleRate(c.SampleRate).SetRawDays(c.RawDays).AddConfigVersion(1).Save(ctx)
	}
	if err != nil {
		return c, err
	}
	s.configs.Remove(siteID)
	return performanceConfig(r), nil
}
func (s *PerformanceService) DateRange(ctx context.Context, siteID int64, start, end string) (time.Time, time.Time, p.Config, error) {
	c, err := s.Config(ctx, siteID)
	if err != nil {
		return time.Time{}, time.Time{}, c, err
	}
	loc, err := time.LoadLocation(c.Timezone)
	if err != nil {
		return time.Time{}, time.Time{}, c, err
	}
	from, err := time.ParseInLocation("2006-01-02", start, loc)
	if err != nil {
		return from, from, c, fmt.Errorf("%w: invalid start date", ErrPerformanceInvalid)
	}
	last, err := time.ParseInLocation("2006-01-02", end, loc)
	if err != nil {
		return from, last, c, fmt.Errorf("%w: invalid end date", ErrPerformanceInvalid)
	}
	to := last.AddDate(0, 0, 1)
	now := time.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	if !to.After(from) || from.Before(today.AddDate(0, 0, 1-c.RawDays)) || to.After(today.AddDate(0, 0, 1)) {
		return from, to, c, fmt.Errorf("%w: performance data is available only within configured raw retention, up to 7 recent calendar days", ErrPerformanceInvalid)
	}
	return from.UTC(), to.UTC(), c, nil
}
func (s *PerformanceService) Data(ctx context.Context, siteID int64, start, end string, f p.Filter) (p.Data, error) {
	from, to, c, err := s.DateRange(ctx, siteID, start, end)
	if err != nil {
		return p.Data{}, err
	}
	if err := f.Validate(); err != nil {
		return p.Data{}, fmt.Errorf("%w: %v", ErrPerformanceInvalid, err)
	}
	encoded, _ := json.Marshal([]any{siteID, from, to, c.Version, f})
	key := string(encoded)
	if result, ok := s.results.Get(key); ok {
		return result, nil
	}
	release, err := s.Shared.QuerySlot(ctx)
	if err != nil {
		return p.Data{}, err
	}
	defer release()
	if result, ok := s.results.Get(key); ok {
		return result, nil
	}
	result, err := s.Repo.Data(ctx, uint64(siteID), from, to, f, c.Timezone)
	if err == nil {
		result.RawDays = c.RawDays
		s.results.Add(key, result)
	}
	return result, err
}
