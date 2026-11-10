package heatmap

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

const ScrollMaxReports = 12

// ScrollReport is a revision of one in-memory browser visit, not a visitor ID.
// Page dimensions and identity are frozen at the beginning of the visit.
type ScrollReport struct {
	ID          uuid.UUID `json:"id"`
	PageViewID  uuid.UUID `json:"page_view_id"`
	StartedAt   time.Time `json:"started_at"`
	Timestamp   time.Time `json:"timestamp"`
	Revision    uint16    `json:"revision"`
	Page        Page      `json:"page"`
	MaxBottom   uint32    `json:"max_bottom"`
	MaxHeight   uint32    `json:"max_height"`
	LateInit    bool      `json:"late_init"`
	Clipped     bool      `json:"clipped"`
	Unsupported bool      `json:"unsupported"`
}

func (r *ScrollReport) Validate(now time.Time) error {
	if r.ID == uuid.Nil || r.PageViewID == uuid.Nil || r.Revision < 1 || r.Revision > ScrollMaxReports ||
		r.StartedAt.Before(now.Add(-48*time.Hour)) || r.StartedAt.After(now.Add(time.Minute)) ||
		r.Timestamp.Before(r.StartedAt) || r.Timestamp.Before(now.Add(-time.Hour)) || r.Timestamp.After(now.Add(time.Minute)) {
		return errors.New("invalid scroll identity, revision or timestamp")
	}
	if err := r.Page.Validate(); err != nil {
		return err
	}
	if r.MaxHeight < r.Page.DocumentHeight || r.MaxHeight > MaxDocumentSize || r.MaxBottom == 0 || r.MaxBottom > r.MaxHeight {
		return errors.New("invalid scroll dimensions")
	}
	return nil
}

type ScrollBatch struct {
	Version    int            `json:"version"`
	Domain     string         `json:"domain"`
	ID         uuid.UUID      `json:"batch_id"`
	SampleRate float64        `json:"sample_rate"`
	Reports    []ScrollReport `json:"reports"`
}

type ScrollRow struct {
	ScrollReport
	SiteID     uint64
	SampleRate float64
	Expires    time.Time
}

func NewScrollRow(siteID uint64, report ScrollReport, cfg Config, rate float64) ScrollRow {
	report.Page.AggregationKey = cfg.KeyFor(report.Page.Path, report.Page.AggregationKey)
	return ScrollRow{ScrollReport: report, SiteID: siteID, SampleRate: rate,
		Expires: report.StartedAt.Add(time.Duration(min(cfg.RawDays, 7)) * 24 * time.Hour)}
}

type ScrollPageCount struct {
	Page
	SampledViews uint64    `json:"sampled_views"`
	LastView     time.Time `json:"last_view"`
}

type ScrollRegion struct {
	Depth float64 `json:"depth"`
	Views uint64  `json:"views"`
	Rate  float64 `json:"rate"`
}

type ScrollData struct {
	Regions         []ScrollRegion `json:"regions"`
	SampledViews    uint64         `json:"sampled_views"`
	ValidViews      uint64         `json:"valid_views"`
	DynamicViews    uint64         `json:"dynamic_views"`
	ClippedViews    uint64         `json:"clipped_views"`
	Unsupported     uint64         `json:"unsupported_views"`
	LateInitViews   uint64         `json:"late_init_views"`
	AverageMaxDepth float64        `json:"average_max_depth"`
	AverageFold     float64        `json:"average_fold"`
	MixedSampling   bool           `json:"mixed_sampling"`
	RawDays         int            `json:"raw_days"`
}
