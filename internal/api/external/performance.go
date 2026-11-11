package external

import (
	"encoding/json"
	"io"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zenstats/zenstats/internal/heatmap"
	p "github.com/zenstats/zenstats/internal/performance"
	"github.com/zenstats/zenstats/internal/service"
	"github.com/zenstats/zenstats/pkg/geoip"
	"github.com/zenstats/zenstats/pkg/iputil"
)

var performanceLimiter = newIngestLimiter()

func PerformanceConfig(s *service.PerformanceService) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		site, err := service.GetSiteService().GetSiteByDomain(c.Request.Context(), c.Query("domain"))
		if err != nil || !site.IsVerified {
			c.Status(404)
			return
		}
		cfg, err := s.Config(c.Request.Context(), site.ID)
		if err != nil {
			c.Status(503)
			return
		}
		c.JSON(200, gin.H{"enabled": cfg.Enabled, "paths": cfg.Paths, "sample_rate": cfg.SampleRate, "config_version": cfg.Version, "protocol_version": p.ProtocolVersion})
	}
}
func PerformanceReports(s *service.PerformanceService) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, heatmap.MaxBodySize)
		var req p.Batch
		decoder := json.NewDecoder(c.Request.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			status := 400
			if _, ok := err.(*http.MaxBytesError); ok {
				status = 413
			}
			c.Status(status)
			return
		}
		if decoder.Decode(new(any)) != io.EOF {
			c.Status(400)
			return
		}
		n := len(req.Reports)
		if req.Version != p.ProtocolVersion || req.ID == uuid.Nil || req.ConfigVersion == 0 || n == 0 || n > heatmap.MaxBatchSize || math.IsNaN(req.SampleRate) || math.IsInf(req.SampleRate, 0) || req.SampleRate <= 0 || req.SampleRate > 1 {
			c.Status(400)
			return
		}
		s.Collector.Received(n)
		reject := func(status int) { s.Collector.Rejected(n); c.Header("X-Zenstats-Accepted", "0"); c.Status(status) }
		site, err := service.GetSiteService().GetSiteByDomain(c.Request.Context(), req.Domain)
		if err != nil || !site.IsVerified {
			reject(400)
			return
		}
		if !verifyRequestOrigin(c, site.Domain, site.AllowedOrigins) {
			reject(403)
			return
		}
		cfg, err := s.Config(c.Request.Context(), site.ID)
		if err != nil {
			reject(503)
			return
		}
		if !cfg.Enabled {
			reject(403)
			return
		}
		if !heatmapBatchRateAllowed(performanceLimiter, site.Domain, n) {
			c.Header("Retry-After", "30")
			reject(429)
			return
		}
		ua := heatmapParser().Parse(c.Request.UserAgent())
		if isDatacenterOrThreatIP(c) || ua.Screen.IsBot() {
			s.Collector.Filtered(n)
			c.Header("X-Zenstats-Accepted", "0")
			c.Status(202)
			return
		}
		now := time.Now()
		seen := map[uuid.UUID]bool{}
		// Validate the entire batch before shielding, geolocation, or queue mutation.
		for _, r := range req.Reports {
			if r.Validate(now) != nil || seen[r.ID] {
				reject(400)
				return
			}
			seen[r.ID] = true
			if !cfg.Allows(r.Path) || !heatmap.HostAllowed(r.Hostname, site.Domain, site.AllowedOrigins) {
				reject(403)
				return
			}
		}
		ip := net.ParseIP(iputil.ClientIP(c.Request))
		country := ""
		if ip != nil && ip.IsGlobalUnicast() && !ip.IsPrivate() {
			if location, e := geoip.GetGeoIP().GetCountryAndRegion(ip.String()); e == nil {
				country = location.IsoCode
			}
		}
		device := strings.ToLower(ua.Screen.Family)
		if device != "mobile" && device != "tablet" {
			device = "desktop"
		}
		rows := make([]p.Row, 0, n)
		allowedHosts := map[string]bool{}
		for _, r := range req.Reports {
			allowed, known := allowedHosts[r.Hostname]
			if !known {
				allowed, err = heatmapShields(c.Request.Context(), site, r.Hostname, ip)
				if err != nil {
					reject(503)
					return
				}
				allowedHosts[r.Hostname] = allowed
			}
			expires := r.NavigationStartedAt.Add(time.Duration(cfg.RawDays) * 24 * time.Hour)
			if !allowed || !expires.After(now) {
				continue
			}
			r.Clipped = r.Clipped || r.Revision == p.MaxRevisions
			rows = append(rows, p.Row{Report: r, SiteID: uint64(site.ID), Device: device, Browser: ua.UserAgent.Family, Country: country, SampleRate: req.SampleRate, ConfigVersion: req.ConfigVersion, Expires: expires})
		}
		s.Collector.Filtered(n - len(rows))
		if len(rows) > 0 && s.Collector.Enqueue(rows) != nil {
			c.Header("Retry-After", "5")
			c.Header("X-Zenstats-Accepted", "0")
			c.Status(503)
			return
		}
		c.Header("X-Zenstats-Accepted", strconv.Itoa(len(rows)))
		c.Status(202)
	}
}
