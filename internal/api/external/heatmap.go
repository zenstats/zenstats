package external

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zenstats/zenstats/internal/heatmap"
	"github.com/zenstats/zenstats/internal/service"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent"
	"github.com/zenstats/zenstats/pkg/geoip"
	"github.com/zenstats/zenstats/pkg/iputil"
	uaparser "github.com/zenstats/zenstats/pkg/ua_parser"
	"github.com/zenstats/zenstats/pkg/utils"
)

var heatmapLimiter = newIngestLimiter()
var heatmapParser = sync.OnceValue(uaparser.New)

func heatmapRateAllowed(domain string, n int) bool {
	heatmapLimiter.mu.Lock()
	defer heatmapLimiter.mu.Unlock()
	w, ok := heatmapLimiter.windows[domain]
	now := time.Now()
	if !ok || !now.Before(w.resetTime) {
		w = &ingestWindow{resetTime: now.Add(time.Minute)}
		heatmapLimiter.windows[domain] = w
	}
	if w.count+n > 6000 {
		return false
	}
	w.count += n
	return true
}

func HeatmapConfig(s *service.HeatmapService) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age=30")
		site, err := service.GetSiteService().GetSiteByDomain(c.Request.Context(), c.Query("domain"))
		if err != nil || !site.IsVerified {
			c.JSON(200, gin.H{"enabled": false})
			return
		}
		cfg, err := s.Config(c.Request.Context(), site.ID)
		if err != nil {
			c.Header("Cache-Control", "no-store")
			c.Status(http.StatusServiceUnavailable)
			return
		}
		c.JSON(200, gin.H{"enabled": cfg.Enabled, "paths": cfg.Paths, "sample_rate": cfg.SampleRate, "max_clicks": cfg.MaxClicks, "protocol_version": heatmap.ProtocolVersion})
	}
}

func HeatmapClicks(s *service.HeatmapService) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, heatmap.MaxBodySize)
		var req heatmap.Batch
		decoder := json.NewDecoder(c.Request.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				c.Status(http.StatusRequestEntityTooLarge)
			} else {
				c.Status(http.StatusBadRequest)
			}
			return
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			c.Status(http.StatusBadRequest)
			return
		}
		n := len(req.Clicks)
		if req.Version != heatmap.ProtocolVersion || req.ID == uuid.Nil || n == 0 || n > heatmap.MaxBatchSize || req.SampleRate <= 0 || req.SampleRate > 1 || math.IsNaN(req.SampleRate) {
			c.Status(http.StatusBadRequest)
			return
		}
		s.Collector.Received(n)
		reject := func(status int) { s.Collector.Rejected(n); c.Header("X-Zenstats-Accepted", "0"); c.Status(status) }
		site, err := service.GetSiteService().GetSiteByDomain(c.Request.Context(), req.Domain)
		if err != nil || !site.IsVerified {
			reject(http.StatusBadRequest)
			return
		}
		if !verifyRequestOrigin(c, site.Domain, site.AllowedOrigins) {
			reject(http.StatusForbidden)
			return
		}
		cfg, err := s.Config(c.Request.Context(), site.ID)
		if err != nil {
			reject(http.StatusServiceUnavailable)
			return
		}
		if !cfg.Enabled {
			reject(http.StatusForbidden)
			return
		}
		if !heatmapRateAllowed(site.Domain, n) {
			c.Header("Retry-After", "30")
			reject(http.StatusTooManyRequests)
			return
		}
		if isDatacenterOrThreatIP(c) || heatmapParser().Parse(c.Request.UserAgent()).Screen.IsBot() {
			s.Collector.Filtered(n)
			c.Header("X-Zenstats-Accepted", "0")
			c.Status(http.StatusAccepted)
			return
		}
		ip := net.ParseIP(iputil.ClientIP(c.Request))
		rows := make([]heatmap.Row, 0, n)
		now := time.Now()
		seen := map[uuid.UUID]bool{}
		cfg.SampleRate = req.SampleRate
		for _, click := range req.Clicks {
			if err := click.Validate(now); err != nil || seen[click.ID] {
				reject(http.StatusBadRequest)
				return
			}
			seen[click.ID] = true
			if !cfg.Allows(click.Page.Path) {
				reject(http.StatusForbidden)
				return
			}
			// A click cannot claim an unrelated host merely by sharing a site ID.
			if !heatmap.HostAllowed(click.Page.Hostname, site.Domain, site.AllowedOrigins) {
				reject(http.StatusForbidden)
				return
			}
			allowed, err := heatmapShields(c.Request.Context(), site, click.Page.Hostname, ip)
			if err != nil {
				reject(http.StatusServiceUnavailable)
				return
			}
			if !allowed {
				continue
			}
			rows = append(rows, heatmap.NewRow(uint64(site.ID), req.ID, click, cfg))
		}
		s.Collector.Filtered(n - len(rows))
		if len(rows) > 0 {
			if err := s.Collector.Enqueue(rows); err != nil {
				c.Header("Retry-After", "5")
				c.Header("X-Zenstats-Accepted", "0")
				c.Status(http.StatusServiceUnavailable)
				return
			}
		}
		c.Header("X-Zenstats-Accepted", strconv.Itoa(len(rows)))
		c.Status(http.StatusAccepted)
	}
}

func heatmapShields(ctx context.Context, site *ent.Site, hostname string, ip net.IP) (bool, error) {
	sites := service.GetSiteService()
	ipRules, err := sites.ListShieldRuleIP(ctx, site.ID)
	if err != nil {
		return false, err
	}
	for _, r := range ipRules {
		if r.Inet.Contains(ip) {
			if r.Action == "deny" {
				return false, nil
			}
			break
		}
	}
	hosts, err := sites.ListShieldRuleHostname(ctx, site.ID)
	if err != nil {
		return false, err
	}
	if len(hosts) > 0 {
		allowed := false
		for _, r := range hosts {
			if r.Action != "allow" {
				continue
			}
			if utils.NormalizeHostname(r.Hostname) == hostname {
				allowed = true
				break
			}
			if r.HostnamePattern != "" {
				pattern, err := regexp.Compile(strings.ReplaceAll(r.HostnamePattern, `\\`, `\`))
				if err == nil && pattern.MatchString(hostname) {
					allowed = true
					break
				}
			}
		}
		if !allowed {
			return false, nil
		}
	}
	countries, err := sites.ListShieldRuleCountry(ctx, site.ID)
	if err != nil {
		return false, err
	}
	if len(countries) > 0 {
		data, err := geoip.GetGeoIP().GetCountryAndRegion(ip.String())
		if err != nil {
			return false, err
		}
		for _, r := range countries {
			if r.CountryCode == data.IsoCode && r.Action == "deny" {
				return false, nil
			}
		}
	}
	return true, nil
}
