// Package heatmap defines the click protocol independently of the event/session pipeline.
package heatmap

import (
	"errors"
	"math"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	ProtocolVersion = 1
	GridSize        = 16
	MaxBatchSize    = 20
	MaxBodySize     = 16 << 10
	MaxDocumentSize = 100000
)

var labelPattern = regexp.MustCompile(`^[a-zA-Z0-9_.:/@-]{1,120}$`)

// Page identifies one reproducible page layout. Dimensions are CSS pixels.
type Page struct {
	Hostname       string `json:"hostname"`
	Path           string `json:"path"`
	Version        string `json:"version"`
	Variant        string `json:"variant"`
	ViewportWidth  uint16 `json:"viewport_width"`
	ViewportHeight uint16 `json:"viewport_height"`
	DocumentWidth  uint32 `json:"document_width"`
	DocumentHeight uint32 `json:"document_height"`
}

func (p *Page) Validate() error {
	p.Hostname = strings.ToLower(strings.TrimSpace(p.Hostname))
	if len(p.Hostname) == 0 || len(p.Hostname) > 253 || strings.ContainsAny(p.Hostname, "/?#@: \t\r\n\\") {
		return errors.New("invalid page hostname")
	}
	if len(p.Path) == 0 || len(p.Path) > 2000 || p.Path[0] != '/' || strings.ContainsAny(p.Path, "?\r\n") {
		return errors.New("invalid canonical path")
	}
	if !labelPattern.MatchString(p.Version) || !labelPattern.MatchString(p.Variant) {
		return errors.New("version and variant must be short non-sensitive identifiers")
	}
	if p.ViewportWidth < 100 || p.ViewportWidth > 8192 || p.ViewportHeight < 100 || p.ViewportHeight > 8192 ||
		p.DocumentWidth < uint32(p.ViewportWidth) || p.DocumentHeight < uint32(p.ViewportHeight) ||
		p.DocumentWidth > MaxDocumentSize || p.DocumentHeight > MaxDocumentSize {
		return errors.New("invalid page dimensions")
	}
	return nil
}

type Click struct {
	ID        uuid.UUID `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Page      Page      `json:"page"`
	X         float64   `json:"x"`
	Y         float64   `json:"y"`
	Element   string    `json:"element"`
	RelativeX float64   `json:"relative_x"`
	RelativeY float64   `json:"relative_y"`
	// document, fixed, sticky, scroll, or unsupported. Only document is projected.
	Position string `json:"position"`
}

func (c *Click) Validate(now time.Time) error {
	if c.ID == uuid.Nil || c.Timestamp.Before(now.Add(-time.Hour)) || c.Timestamp.After(now.Add(time.Minute)) {
		return errors.New("invalid click id or timestamp")
	}
	if err := c.Page.Validate(); err != nil {
		return err
	}
	if !finite(c.X) || !finite(c.Y) || c.X < 0 || c.Y < 0 || c.X >= float64(c.Page.DocumentWidth) || c.Y >= float64(c.Page.DocumentHeight) {
		return errors.New("click outside document bounds")
	}
	if c.Element != "" && !labelPattern.MatchString(c.Element) {
		return errors.New("invalid stable element identifier")
	}
	if !finite(c.RelativeX) || !finite(c.RelativeY) || c.RelativeX < 0 || c.RelativeX > 1 || c.RelativeY < 0 || c.RelativeY > 1 {
		return errors.New("invalid element coordinates")
	}
	switch c.Position {
	case "document", "fixed", "sticky", "scroll", "unsupported":
	default:
		return errors.New("invalid positioning type")
	}
	return nil
}

func finite(n float64) bool { return !math.IsNaN(n) && !math.IsInf(n, 0) }

type Batch struct {
	Version    int       `json:"version"`
	Domain     string    `json:"domain"`
	ID         uuid.UUID `json:"batch_id"`
	Clicks     []Click   `json:"clicks"`
	SampleRate float64   `json:"sample_rate"`
}

type Config struct {
	Enabled       bool     `json:"enabled"`
	Paths         []string `json:"paths"`
	SampleRate    float64  `json:"sample_rate"`
	MaxClicks     int      `json:"max_clicks"`
	RawDays       int      `json:"raw_days"`
	AggregateDays int      `json:"aggregate_days"`
	Timezone      string   `json:"timezone"`
}

func DefaultConfig() Config {
	return Config{Paths: []string{}, SampleRate: .1, MaxClicks: 100, RawDays: 7, AggregateDays: 90, Timezone: "UTC"}
}

func (c Config) Validate() error {
	if !finite(c.SampleRate) || c.SampleRate <= 0 || c.SampleRate > 1 || c.MaxClicks < 1 || c.MaxClicks > 1000 ||
		c.RawDays < 1 || c.RawDays > 14 || c.AggregateDays < c.RawDays || c.AggregateDays > 180 || len(c.Paths) > 50 {
		return errors.New("invalid heatmap limits or retention")
	}
	if c.Enabled && len(c.Paths) == 0 {
		return errors.New("enable at least one page path")
	}
	for _, p := range c.Paths {
		if !strings.HasPrefix(p, "/") || len(p) > 1000 || strings.ContainsAny(p, "?[]\\\r\n") {
			return errors.New("invalid page rule")
		}
	}
	if _, err := time.LoadLocation(c.Timezone); err != nil {
		return errors.New("invalid timezone")
	}
	return nil
}

func (c Config) Allows(pagePath string) bool {
	for _, rule := range c.Paths {
		pattern := "^" + strings.ReplaceAll(regexp.QuoteMeta(rule), `\*`, `[^/]*`) + "$"
		if ok, _ := regexp.MatchString(pattern, pagePath); ok {
			return true
		}
	}
	return false
}

// CanonicalPath never retains query values, including inside SPA hash routes.
func CanonicalPath(raw string, hash bool) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return "", errors.New("invalid page URL")
	}
	p := u.EscapedPath()
	if p == "" {
		p = "/"
	}
	if hash {
		fragment := strings.SplitN(u.Fragment, "?", 2)[0]
		if fragment != "" {
			p += "#" + fragment
		}
	}
	return p, nil
}

type Row struct {
	Click
	SiteID           uint64
	BatchID          uuid.UUID
	Bucket           time.Time
	RawExpires       time.Time
	AggregateExpires time.Time
	SampleRate       float64
}

func NewRow(siteID uint64, batchID uuid.UUID, click Click, cfg Config) Row {
	loc, _ := time.LoadLocation(cfg.Timezone)
	_, offset := click.Timestamp.In(loc).Zone()
	bucket := click.Timestamp.Add(time.Duration(offset) * time.Second).Truncate(time.Hour).Add(-time.Duration(offset) * time.Second).UTC()
	return Row{Click: click, SiteID: siteID, BatchID: batchID, Bucket: bucket, SampleRate: cfg.SampleRate,
		RawExpires:       click.Timestamp.Add(time.Duration(cfg.RawDays) * 24 * time.Hour),
		AggregateExpires: bucket.Add(time.Duration(cfg.AggregateDays) * 24 * time.Hour)}
}

type PageCount struct {
	Page
	Clicks    uint64    `json:"clicks"`
	LastClick time.Time `json:"last_click"`
}

type Point struct {
	X     int32  `json:"x"`
	Y     int32  `json:"y"`
	Count uint64 `json:"count"`
}
type ElementCount struct {
	Element string `json:"element"`
	Count   uint64 `json:"count"`
}
type Data struct {
	Points    []Point        `json:"points"`
	Elements  []ElementCount `json:"elements"`
	Clicks    uint64         `json:"clicks"`
	Unmapped  uint64         `json:"unmapped"`
	Truncated bool           `json:"truncated"`
	GridSize  int            `json:"grid_size"`
}

// HostAllowed applies the same explicit-host / wildcard-origin policy to clicks
// and administrator-supplied screenshot metadata.
func HostAllowed(host, domain, origins string) bool {
	for _, raw := range append([]string{domain}, strings.Split(origins, ",")...) {
		raw = strings.TrimSpace(raw)
		if !strings.Contains(raw, "://") {
			raw = "https://" + raw
		}
		u, err := url.Parse(raw)
		if err != nil {
			continue
		}
		pattern := strings.ToLower(u.Hostname())
		if host == pattern {
			return true
		}
		if strings.HasPrefix(pattern, "*.") {
			base := pattern[2:]
			if host == base || strings.HasSuffix(host, "."+base) {
				return true
			}
		}
	}
	return false
}
