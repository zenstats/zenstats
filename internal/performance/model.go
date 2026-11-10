// Package performance defines sampled document-navigation measurements, separate
// from pageviews, visitors, sessions, and business events.
package performance

import (
	"errors"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/zenstats/zenstats/internal/heatmap"
)

const ProtocolVersion = 1
const MaxRevisions = 32

var Names = []string{"LCP", "INP", "CLS", "TTFB", "FCP"}
var label = regexp.MustCompile(`^[a-zA-Z0-9_.:/@-]{1,120}$`)

func IsMetric(name string) bool {
	for _, n := range Names {
		if name == n {
			return true
		}
	}
	return false
}
func IsNavigation(n string) bool {
	switch n {
	case "navigate", "reload", "back-forward", "back-forward-cache", "prerender", "restore":
		return true
	}
	return false
}
func Thresholds(name string) (float64, float64) {
	switch name {
	case "LCP":
		return 2500, 4000
	case "INP":
		return 200, 500
	case "CLS":
		return .1, .25
	case "FCP":
		return 1800, 3000
	default:
		return 800, 1800
	}
}

type Config struct {
	Enabled    bool     `json:"enabled"`
	Paths      []string `json:"paths"`
	SampleRate float64  `json:"sample_rate"`
	RawDays    int      `json:"raw_days"`
	Timezone   string   `json:"timezone"`
	Version    uint64   `json:"config_version"`
}

func DefaultConfig() Config {
	return Config{Paths: []string{"/*"}, SampleRate: .1, RawDays: 7, Timezone: "UTC"}
}
func (c Config) Validate() error {
	if math.IsNaN(c.SampleRate) || math.IsInf(c.SampleRate, 0) || c.SampleRate <= 0 || c.SampleRate > 1 || c.RawDays < 1 || c.RawDays > 7 {
		return errors.New("invalid performance sampling or retention")
	}
	// Reuse only path grammar, never heatmap template configuration or identity.
	rules := heatmap.DefaultConfig()
	rules.Paths = c.Paths
	rules.Enabled = c.Enabled
	if err := rules.Validate(); err != nil {
		return err
	}
	_, err := time.LoadLocation(c.Timezone)
	return err
}
func (c Config) Allows(path string) bool {
	rules := heatmap.Config{Paths: c.Paths}
	return rules.Allows(path)
}

// Context has no numeric value. Missing metrics are never synthesized as zero.
type Report struct {
	ID                  uuid.UUID `json:"id"`
	LifecycleID         uuid.UUID `json:"lifecycle_id"`
	Kind                string    `json:"record_kind"`
	Revision            uint16    `json:"revision"`
	NavigationStartedAt time.Time `json:"navigation_started_at"`
	ReportedAt          time.Time `json:"reported_at"`
	Hostname            string    `json:"hostname"`
	Path                string    `json:"path"`
	Release             string    `json:"release"`
	NavigationType      string    `json:"navigation_type"`
	MetricID            string    `json:"metric_id,omitempty"`
	MetricName          string    `json:"metric_name,omitempty"`
	Value               *float64  `json:"value,omitempty"`
	Supported           []string  `json:"supported_metrics,omitempty"`
	InitDelayMS         uint32    `json:"init_delay_ms"`
	BackgroundInit      bool      `json:"background_init"`
	Clipped             bool      `json:"clipped"`
}

func (r Report) Validate(now time.Time) error {
	if r.ID == uuid.Nil || r.LifecycleID == uuid.Nil || r.Revision < 1 || r.Revision > MaxRevisions || r.NavigationStartedAt.Before(now.Add(-48*time.Hour)) || r.NavigationStartedAt.After(now.Add(time.Minute)) || r.ReportedAt.Before(r.NavigationStartedAt) || r.ReportedAt.Before(now.Add(-time.Hour)) || r.ReportedAt.After(now.Add(time.Minute)) || !IsNavigation(r.NavigationType) || r.InitDelayMS > 172800000 {
		return errors.New("invalid performance identity, navigation or timestamp")
	}
	p := heatmap.Page{Hostname: r.Hostname, Path: r.Path, Version: r.Release, Variant: "default", ViewportWidth: 100, ViewportHeight: 100, DocumentWidth: 100, DocumentHeight: 100}
	if err := p.Validate(); err != nil {
		return err
	}
	if r.Kind == "context" {
		if r.Value != nil || r.MetricName != "" || r.MetricID != "" || r.Clipped || len(r.Supported) > 5 {
			return errors.New("invalid navigation context")
		}
		seen := map[string]bool{}
		for _, n := range r.Supported {
			if !IsMetric(n) || seen[n] {
				return errors.New("invalid metric capabilities")
			}
			seen[n] = true
		}
		return nil
	}
	if r.Kind != "metric" || !IsMetric(r.MetricName) || !label.MatchString(r.MetricID) || r.Value == nil || len(r.Supported) != 0 {
		return errors.New("invalid metric report")
	}
	maxValue := float64(3600000)
	if r.MetricName == "CLS" {
		maxValue = 100
	}
	if math.IsNaN(*r.Value) || math.IsInf(*r.Value, 0) || *r.Value < 0 || *r.Value > maxValue {
		return errors.New("invalid metric value")
	}
	return nil
}

type Batch struct {
	Version       int       `json:"version"`
	Domain        string    `json:"domain"`
	ID            uuid.UUID `json:"batch_id"`
	SampleRate    float64   `json:"sample_rate"`
	ConfigVersion uint64    `json:"config_version"`
	Reports       []Report  `json:"reports"`
}
type Row struct {
	Report
	SiteID        uint64
	Device        string
	Browser       string
	Country       string
	SampleRate    float64
	ConfigVersion uint64
	Expires       time.Time
}

type Filter struct {
	Hostname   string `json:"hostname"`
	Path       string `json:"path"`
	Release    string `json:"release"`
	Device     string `json:"device"`
	Browser    string `json:"browser"`
	Country    string `json:"country"`
	Navigation string `json:"navigation"`
	Metric     string `json:"metric"`
}

func (f Filter) Validate() error {
	if !IsMetric(f.Metric) || f.Device != "" && f.Device != "mobile" && f.Device != "tablet" && f.Device != "desktop" {
		return errors.New("invalid performance metric or device")
	}
	if f.Navigation != "" && !IsNavigation(f.Navigation) {
		return errors.New("invalid navigation filter")
	}
	if len(f.Hostname) > 253 || len(f.Path) > 1000 || len(f.Release) > 120 || len(f.Browser) > 100 || len(f.Country) > 2 || strings.ContainsAny(f.Hostname+f.Path+f.Release+f.Browser, "\r\n\x00") || f.Path != "" && (!strings.HasPrefix(f.Path, "/") || strings.ContainsAny(f.Path, "?[]\\")) {
		return errors.New("invalid performance filter")
	}
	return nil
}

type Metric struct {
	Name             string   `json:"name"`
	Unit             string   `json:"unit"`
	P75              *float64 `json:"p75"`
	Samples          uint64   `json:"samples"`
	Supported        uint64   `json:"supported_navigations"`
	Missing          uint64   `json:"missing_navigations"`
	Coverage         float64  `json:"coverage"`
	Good             uint64   `json:"good"`
	NeedsImprovement uint64   `json:"needs_improvement"`
	Poor             uint64   `json:"poor"`
	Clipped          uint64   `json:"clipped"`
	SmallSample      bool     `json:"small_sample"`
	Rating           string   `json:"rating"`
}

func (m *Metric) Complete(total uint64) {
	m.Unit = "ms"
	if m.Name == "CLS" {
		m.Unit = "score"
	}
	m.SmallSample = m.Samples < 100
	m.Rating = "insufficient"
	if total > m.Samples {
		m.Missing = total - m.Samples
	}
	if total > 0 {
		m.Coverage = float64(m.Samples) * 100 / float64(total)
	}
	if m.Samples == 0 {
		m.P75 = nil
		return
	}
	if m.SmallSample {
		return
	}
	good, poor := Thresholds(m.Name)
	m.Rating = "poor"
	if *m.P75 <= poor {
		m.Rating = "needs_improvement"
	}
	if *m.P75 <= good {
		m.Rating = "good"
	}
}

type Point struct {
	Day     string   `json:"day"`
	Samples uint64   `json:"samples"`
	P75     *float64 `json:"p75"`
}
type Page struct {
	Hostname         string   `json:"hostname"`
	Path             string   `json:"path"`
	Samples          uint64   `json:"samples"`
	P75              *float64 `json:"p75"`
	Good             uint64   `json:"good"`
	NeedsImprovement uint64   `json:"needs_improvement"`
	Poor             uint64   `json:"poor"`
}
type Data struct {
	Navigations    uint64    `json:"sampled_navigations"`
	Eligible       uint64    `json:"eligible_navigations"`
	LateInit       uint64    `json:"late_init_navigations"`
	BackgroundInit uint64    `json:"background_init_navigations"`
	MixedSampling  bool      `json:"mixed_sampling"`
	MixedConfig    bool      `json:"mixed_config"`
	Overall        string    `json:"overall"`
	Metrics        []Metric  `json:"metrics"`
	Trend          []Point   `json:"trend"`
	Pages          []Page    `json:"pages"`
	RawDays        int       `json:"raw_days"`
	Timezone       string    `json:"timezone"`
	GeneratedAt    time.Time `json:"generated_at"`
}
