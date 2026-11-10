package external

import (
	"encoding/json"
	"io"
	"math"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zenstats/zenstats/internal/heatmap"
	"github.com/zenstats/zenstats/internal/service"
	"github.com/zenstats/zenstats/pkg/iputil"
)

var scrollLimiter = newIngestLimiter()

func HeatmapScroll(s *service.HeatmapService) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, heatmap.MaxBodySize)
		var req heatmap.ScrollBatch
		decoder := json.NewDecoder(c.Request.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			status := http.StatusBadRequest
			if _, ok := err.(*http.MaxBytesError); ok {
				status = http.StatusRequestEntityTooLarge
			}
			c.Status(status)
			return
		}
		if decoder.Decode(new(any)) != io.EOF {
			c.Status(http.StatusBadRequest)
			return
		}
		n := len(req.Reports)
		if req.Version != 1 || req.ID == uuid.Nil || n == 0 || n > heatmap.MaxBatchSize || math.IsNaN(req.SampleRate) || math.IsInf(req.SampleRate, 0) || req.SampleRate <= 0 || req.SampleRate > 1 {
			c.Status(http.StatusBadRequest)
			return
		}
		s.ScrollCollector.Received(n)
		reject := func(status int) {
			s.ScrollCollector.Rejected(n)
			c.Header("X-Zenstats-Accepted", "0")
			c.Status(status)
		}
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
		if !cfg.Enabled || !cfg.ScrollEnabled {
			reject(http.StatusForbidden)
			return
		}
		if !heatmapBatchRateAllowed(scrollLimiter, site.Domain, n) {
			c.Header("Retry-After", "30")
			reject(http.StatusTooManyRequests)
			return
		}
		if isDatacenterOrThreatIP(c) || heatmapParser().Parse(c.Request.UserAgent()).Screen.IsBot() {
			s.ScrollCollector.Filtered(n)
			c.Header("X-Zenstats-Accepted", "0")
			c.Status(http.StatusAccepted)
			return
		}
		ip := net.ParseIP(iputil.ClientIP(c.Request))
		now := time.Now()
		seen := map[uuid.UUID]bool{}
		rows := make([]heatmap.ScrollRow, 0, n)
		for _, r := range req.Reports {
			if r.Validate(now) != nil || seen[r.ID] {
				reject(http.StatusBadRequest)
				return
			}
			seen[r.ID] = true
			if !cfg.Allows(r.Page.Path) || !heatmap.HostAllowed(r.Page.Hostname, site.Domain, site.AllowedOrigins) {
				reject(http.StatusForbidden)
				return
			}
			allowed, err := heatmapShields(c.Request.Context(), site, r.Page.Hostname, ip)
			if err != nil {
				reject(http.StatusServiceUnavailable)
				return
			}
			if !allowed {
				continue
			}
			row := heatmap.NewScrollRow(uint64(site.ID), r, cfg, req.SampleRate)
			if !row.Expires.After(now) {
				continue
			}
			rows = append(rows, row)
		}
		s.ScrollCollector.Filtered(n - len(rows))
		if len(rows) > 0 {
			if s.ScrollCollector.Enqueue(rows) != nil {
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
