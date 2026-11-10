package heatmaps

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/heatmap"
	"github.com/zenstats/zenstats/internal/service"
	"github.com/zenstats/zenstats/pkg/response"
)

func (h *Handler) Capture(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	var req struct {
		URL   string       `json:"url"`
		Page  heatmap.Page `json:"page"`
		Scope string       `json:"scope"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err)
		return
	}
	row, err := h.Service.Capture(c.Request.Context(), c.GetInt64("site_id"), req.URL, req.Page, req.Scope == "key")
	if err != nil {
		status := 503
		if errors.Is(err, service.ErrHeatmapCaptureBusy) {
			status = 429
			c.Header("Retry-After", "2")
		}
		if errors.Is(err, service.ErrHeatmapInvalid) {
			status = 400
		}
		fail(c, status, err)
		return
	}
	response.Success(c, row)
}
