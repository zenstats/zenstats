package heatmaps

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/heatmap"
	"github.com/zenstats/zenstats/internal/service"
	"github.com/zenstats/zenstats/pkg/response"
)

type Handler struct{ Service *service.HeatmapService }

func NewHandler(s *service.HeatmapService) *Handler { return &Handler{Service: s} }

func fail(c *gin.Context, status int, err error) {
	if status >= 500 {
		slog.Error("heatmap request failed", "error", err)
		err = errors.New("heatmap service unavailable")
	}
	response.ErrorWithHTTPStatus(c, status, status, err)
}

func (h *Handler) Config(c *gin.Context) {
	cfg, err := h.Service.Config(c.Request.Context(), c.GetInt64("site_id"))
	if err != nil {
		fail(c, 503, err)
		return
	}
	canWrite, err := h.Service.CanWrite(c.Request.Context(), c.GetInt64("site_id"), c.GetInt64("user_id"))
	if err != nil {
		fail(c, 503, err)
		return
	}
	response.Success(c, struct {
		heatmap.Config
		CanWrite bool `json:"can_write"`
	}{cfg, canWrite && c.GetString("user_type") != "sub_account"})
}

func (h *Handler) WriteAccess() gin.HandlerFunc {
	return func(c *gin.Context) {
		allowed, err := h.Service.CanWrite(c.Request.Context(), c.GetInt64("site_id"), c.GetInt64("user_id"))
		if err != nil {
			fail(c, 503, err)
			c.Abort()
			return
		}
		if !allowed {
			fail(c, 403, errors.New("site administrator required"))
			c.Abort()
			return
		}
		c.Next()
	}
}
func (h *Handler) SaveConfig(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	var cfg heatmap.Config
	if err := c.ShouldBindJSON(&cfg); err != nil {
		fail(c, 400, err)
		return
	}
	if cfg.Timezone == "" {
		cfg.Timezone = "UTC"
	}
	if err := cfg.Validate(); err != nil {
		fail(c, 400, err)
		return
	}
	cfg, err := h.Service.SaveConfig(c.Request.Context(), c.GetInt64("site_id"), cfg)
	if err != nil {
		fail(c, 503, err)
		return
	}
	response.Success(c, cfg)
}

func (h *Handler) Quality(c *gin.Context) { response.Success(c, h.Service.Collector.Snapshot()) }

func (h *Handler) Pages(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 6*time.Second)
	defer cancel()
	siteID := c.GetInt64("site_id")
	from, to, err := h.Service.DateRange(ctx, siteID, c.Query("start"), c.Query("end"))
	if err != nil {
		fail(c, 400, err)
		return
	}
	release, err := h.Service.QuerySlot(ctx)
	if err != nil {
		fail(c, 503, err)
		return
	}
	defer release()
	pages, err := h.Service.Repo.Pages(ctx, uint64(siteID), from, to)
	if err != nil {
		fail(c, 503, err)
		return
	}
	response.Success(c, pages)
}

func (h *Handler) Data(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 6*time.Second)
	defer cancel()
	siteID := c.GetInt64("site_id")
	from, to, err := h.Service.DateRange(ctx, siteID, c.Query("start"), c.Query("end"))
	if err != nil {
		fail(c, 400, err)
		return
	}
	var page heatmap.Page
	if err := json.Unmarshal([]byte(c.Query("page")), &page); err != nil {
		fail(c, 400, err)
		return
	}
	if err := page.Validate(); err != nil {
		fail(c, 400, err)
		return
	}
	release, err := h.Service.QuerySlot(ctx)
	if err != nil {
		fail(c, 503, err)
		return
	}
	defer release()
	data, err := h.Service.Repo.Data(ctx, uint64(siteID), page, from, to)
	if err != nil {
		fail(c, 503, err)
		return
	}
	response.Success(c, data)
}

func (h *Handler) Snapshots(c *gin.Context) {
	rows, err := h.Service.Snapshots(c.Request.Context(), c.GetInt64("site_id"))
	if err != nil {
		fail(c, 503, err)
		return
	}
	response.Success(c, rows)
}

func (h *Handler) CreateSnapshot(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 6<<20)
	if err := c.Request.ParseMultipartForm(1 << 20); err != nil {
		fail(c, 400, err)
		return
	}
	if c.Request.MultipartForm != nil {
		defer c.Request.MultipartForm.RemoveAll()
	}
	var page heatmap.Page
	if err := json.Unmarshal([]byte(c.PostForm("page")), &page); err != nil {
		fail(c, 400, err)
		return
	}
	f, _, err := c.Request.FormFile("image")
	if err != nil {
		fail(c, 400, err)
		return
	}
	defer f.Close()
	row, err := h.Service.CreateSnapshot(c.Request.Context(), c.GetInt64("site_id"), page, f)
	if err != nil {
		status := 503
		if errors.Is(err, service.ErrHeatmapInvalid) {
			status = 400
		}
		fail(c, status, err)
		return
	}
	response.Success(c, row)
}

func (h *Handler) Image(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		fail(c, 400, err)
		return
	}
	r, err := h.Service.Snapshot(c.Request.Context(), c.GetInt64("site_id"), id)
	if err != nil {
		if errors.Is(err, service.ErrHeatmapNotFound) {
			fail(c, 404, err)
		} else {
			fail(c, 503, err)
		}
		return
	}
	f, err := h.Service.SnapshotFile(r)
	if err != nil {
		fail(c, 404, err)
		return
	}
	defer f.Close()
	c.Header("Content-Type", r.ContentType)
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	http.ServeContent(c.Writer, c.Request, r.FileKey, r.CreatedAt, f)
}

func (h *Handler) DeleteSnapshot(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		fail(c, 400, err)
		return
	}
	err = h.Service.DeleteSnapshot(c.Request.Context(), c.GetInt64("site_id"), id)
	if err != nil {
		if errors.Is(err, service.ErrHeatmapNotFound) {
			fail(c, 404, err)
		} else {
			fail(c, 503, err)
		}
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

// Replay uses retained raw clicks to repair an interrupted dependent-view write.
// Repeating it is safe because aggregate counts merge exact click ID sets.
func (h *Handler) Replay(c *gin.Context) {
	var req struct {
		From time.Time `json:"from"`
		To   time.Time `json:"to"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err)
		return
	}
	cfg, err := h.Service.Config(c.Request.Context(), c.GetInt64("site_id"))
	if err != nil {
		fail(c, 503, err)
		return
	}
	if !req.To.After(req.From) || req.To.Sub(req.From) > 24*time.Hour || req.From.Before(time.Now().Add(-time.Duration(cfg.RawDays)*24*time.Hour)) || req.To.After(time.Now()) {
		fail(c, 400, errors.New("replay must cover at most 24 hours within raw retention"))
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 6*time.Second)
	defer cancel()
	release, err := h.Service.QuerySlot(ctx)
	if err != nil {
		fail(c, 503, err)
		return
	}
	defer release()
	if err := h.Service.Repo.Replay(ctx, uint64(c.GetInt64("site_id")), req.From, req.To); err != nil {
		fail(c, 503, err)
		return
	}
	response.Success(c, gin.H{"replayed": true})
}
