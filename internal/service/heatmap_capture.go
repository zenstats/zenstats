package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/zenstats/zenstats/config"
	"github.com/zenstats/zenstats/internal/heatmap"
)

var ErrHeatmapCaptureBusy = errors.New("screenshot worker busy; try again shortly")

func ScreenshotAvailable() bool {
	return config.Conf.Heatmap.ScreenshotURL != "" && config.Conf.Heatmap.ScreenshotToken != ""
}

// Capture is isolated from event ingestion. The worker enforces network policy;
// the API enforces site membership, URL ownership and exact layout compatibility.
func (s *HeatmapService) Capture(ctx context.Context, siteID int64, rawURL string, expected heatmap.Page, byKey bool) (HeatmapSnapshot, error) {
	select {
	case s.captures <- struct{}{}:
		defer func() { <-s.captures }()
	default:
		return HeatmapSnapshot{}, ErrHeatmapCaptureBusy
	}
	invalid := func(reason string) (HeatmapSnapshot, error) {
		return HeatmapSnapshot{}, fmt.Errorf("%w: %s", ErrHeatmapInvalid, reason)
	}
	if !ScreenshotAvailable() {
		return invalid("automatic screenshots are not configured")
	}
	if err := expected.Validate(); err != nil {
		return invalid(err.Error())
	}
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || len(rawURL) > 2000 || u.RawQuery != "" || strings.Contains(u.Fragment, "?") {
		return invalid("enter an HTTP(S) URL without credentials or query values")
	}
	site, err := s.db.Client.Site.Get(ctx, siteID)
	if err != nil {
		return HeatmapSnapshot{}, err
	}
	if !heatmap.HostAllowed(u.Hostname(), site.Domain, site.AllowedOrigins) || u.Hostname() != expected.Hostname {
		return invalid("screenshot URL must belong to the selected hostname")
	}
	cfg, err := s.Config(ctx, siteID)
	if err != nil {
		return HeatmapSnapshot{}, err
	}
	path, err := heatmap.CanonicalPath(rawURL, u.Fragment != "")
	if err != nil {
		return invalid("invalid URL")
	}
	if byKey {
		if expected.AggregationKey == "" || cfg.KeyFor(path, expected.AggregationKey) != expected.AggregationKey {
			return invalid("URL does not match the selected aggregation key")
		}
	} else if path != expected.Path {
		return invalid("URL does not match the selected path")
	}
	ctx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()
	payload, _ := json.Marshal(map[string]any{"url": rawURL, "viewport_width": expected.ViewportWidth, "viewport_height": expected.ViewportHeight})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, config.Conf.Heatmap.ScreenshotURL, bytes.NewReader(payload))
	if err != nil {
		return HeatmapSnapshot{}, err
	}
	req.Header.Set("Authorization", "Bearer "+config.Conf.Heatmap.ScreenshotToken)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 40 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Do(req)
	if err != nil {
		return HeatmapSnapshot{}, err
	}
	defer res.Body.Close()
	if res.StatusCode == 429 {
		return HeatmapSnapshot{}, ErrHeatmapCaptureBusy
	}
	if res.StatusCode >= 500 {
		return HeatmapSnapshot{}, fmt.Errorf("screenshot worker unavailable: status %d", res.StatusCode)
	}
	if res.StatusCode != 200 {
		return invalid("capture failed: check URL accessibility, browser service availability and layout")
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return HeatmapSnapshot{}, err
	}
	var result struct {
		URL   string       `json:"url"`
		Page  heatmap.Page `json:"page"`
		Image string       `json:"image"`
	}
	if err = json.Unmarshal(body, &result); err != nil {
		return HeatmapSnapshot{}, err
	}
	final, err := url.Parse(result.URL)
	if err != nil || final.User != nil || final.RawQuery != "" || strings.Contains(final.Fragment, "?") || (final.Scheme != "https" && final.Scheme != "http") || final.Hostname() != expected.Hostname {
		return invalid("redirected URL does not match the selected hostname")
	}
	finalPath, err := heatmap.CanonicalPath(result.URL, final.Fragment != "")
	if err != nil {
		return invalid("invalid final URL")
	}
	actual := result.Page
	if actual.Hostname != final.Hostname() || actual.Path != finalPath {
		return invalid("inconsistent screenshot metadata")
	}
	actual.AggregationKey = cfg.KeyFor(actual.Path, actual.AggregationKey)
	if actual.Version != expected.Version || actual.Variant != expected.Variant || actual.ViewportWidth != expected.ViewportWidth || actual.ViewportHeight != expected.ViewportHeight || actual.DocumentWidth != expected.DocumentWidth || actual.DocumentHeight != expected.DocumentHeight {
		return invalid(fmt.Sprintf("layout mismatch: current %s/%s %dx%d -> %dx%d, selected %s/%s %dx%d -> %dx%d; choose a matching layout or upload a historical screenshot", actual.Version, actual.Variant, actual.ViewportWidth, actual.ViewportHeight, actual.DocumentWidth, actual.DocumentHeight, expected.Version, expected.Variant, expected.ViewportWidth, expected.ViewportHeight, expected.DocumentWidth, expected.DocumentHeight))
	}
	if actual.AggregationKey != expected.AggregationKey {
		return invalid("captured page aggregation key differs from selection")
	}
	if !byKey && actual.Path != expected.Path {
		return invalid("redirect changed the selected path")
	}
	image, err := base64.StdEncoding.DecodeString(result.Image)
	if err != nil {
		return invalid("invalid screenshot response")
	}
	return s.CreateSnapshot(ctx, siteID, actual, bytes.NewReader(image), result.URL)
}
