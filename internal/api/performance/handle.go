package performance

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	p "github.com/zenstats/zenstats/internal/performance"
	"github.com/zenstats/zenstats/internal/service"
	"github.com/zenstats/zenstats/pkg/response"
)

type Handler struct{ Service *service.PerformanceService }

func NewHandler(s *service.PerformanceService) *Handler { return &Handler{Service: s} }
func fail(c *gin.Context, status int, err error) {
	if status >= 500 {
		slog.Error("performance request failed", "error", err)
		err = errors.New("performance service unavailable; narrow the selection or retry")
	}
	response.ErrorWithHTTPStatus(c, status, status, err)
}
func (h *Handler) Config(c *gin.Context) {
	cfg, err := h.Service.Config(c.Request.Context(), c.GetInt64("site_id"))
	if err != nil {
		fail(c, 503, err)
		return
	}
	allowed, err := h.Service.Shared.CanWrite(c.Request.Context(), c.GetInt64("site_id"), c.GetInt64("user_id"))
	if err != nil {
		fail(c, 503, err)
		return
	}
	response.Success(c, struct {
		p.Config
		CanWrite bool `json:"can_write"`
	}{cfg, allowed && c.GetString("user_type") != "sub_account"})
}
func (h *Handler) SaveConfig(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	cfg := p.DefaultConfig()
	if err := c.ShouldBindJSON(&cfg); err != nil {
		fail(c, 400, err)
		return
	}
	cfg, err := h.Service.SaveConfig(c.Request.Context(), c.GetInt64("site_id"), cfg)
	if err != nil {
		status := 503
		if errors.Is(err, service.ErrPerformanceInvalid) {
			status = 400
		}
		fail(c, status, err)
		return
	}
	response.Success(c, cfg)
}
func (h *Handler) Data(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 6*time.Second)
	defer cancel()
	metric := c.DefaultQuery("metric", "LCP")
	f := p.Filter{Hostname: c.Query("hostname"), Path: c.Query("path"), Release: c.Query("release"), Device: c.Query("device"), Browser: c.Query("browser"), Country: c.Query("country"), Navigation: c.Query("navigation"), Metric: metric}
	data, err := h.Service.Data(ctx, c.GetInt64("site_id"), c.Query("start"), c.Query("end"), f)
	if err != nil {
		status := 503
		if errors.Is(err, service.ErrPerformanceInvalid) {
			status = 400
		}
		fail(c, status, err)
		return
	}
	response.Success(c, data)
}
func (h *Handler) Quality(c *gin.Context) { response.Success(c, h.Service.Collector.Snapshot()) }
