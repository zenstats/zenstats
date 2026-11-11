package heatmaps

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/heatmap"
	"github.com/zenstats/zenstats/internal/store/clickhouse/repository"
	"github.com/zenstats/zenstats/pkg/response"
)

func (h *Handler) scrollRange(ctx context.Context, siteID int64, start, end string) (time.Time, time.Time, int, error) {
	from, to, err := h.Service.DateRange(ctx, siteID, start, end)
	if err != nil {
		return from, to, 0, err
	}
	cfg, err := h.Service.Config(ctx, siteID)
	if err != nil {
		return from, to, 0, err
	}
	days := min(cfg.RawDays, 7)
	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		return from, to, 0, err
	}
	now := time.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	if from.Before(today.AddDate(0, 0, -days+1)) || to.After(today.AddDate(0, 0, 1)) {
		return from, to, days, errors.New("scroll data is available only within the configured raw retention, up to 7 calendar days")
	}
	return from, to, days, nil
}

func (h *Handler) ScrollQuality(c *gin.Context) {
	response.Success(c, h.Service.ScrollCollector.Snapshot())
}

func (h *Handler) ScrollPages(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 6*time.Second)
	defer cancel()
	siteID := c.GetInt64("site_id")
	from, to, _, err := h.scrollRange(ctx, siteID, c.Query("start"), c.Query("end"))
	if err != nil {
		fail(c, 400, err)
		return
	}
	key, path := c.Query("key"), c.Query("path")
	if len(key) > 120 || len(path) > 1000 || path != "" && (!strings.HasPrefix(path, "/") || strings.ContainsAny(path, "?[]\\\r\n")) {
		fail(c, 400, errors.New("invalid heatmap filter"))
		return
	}
	release, err := h.Service.QuerySlot(ctx)
	if err != nil {
		fail(c, 503, err)
		return
	}
	defer release()
	pages, err := h.Service.Repo.ScrollPages(ctx, uint64(siteID), from, to, repository.HeatmapPageFilter{ByKey: c.Query("scope") == "key", Key: key, Path: path})
	if err != nil {
		fail(c, 503, err)
		return
	}
	response.Success(c, pages)
}

func (h *Handler) ScrollData(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 6*time.Second)
	defer cancel()
	siteID := c.GetInt64("site_id")
	from, to, days, err := h.scrollRange(ctx, siteID, c.Query("start"), c.Query("end"))
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
	if c.Query("scope") == "key" && page.AggregationKey == "" {
		fail(c, 400, errors.New("aggregation key required"))
		return
	}
	release, err := h.Service.QuerySlot(ctx)
	if err != nil {
		fail(c, 503, err)
		return
	}
	defer release()
	data, err := h.Service.Repo.ScrollData(ctx, uint64(siteID), page, from, to, c.Query("scope") == "key")
	if err != nil {
		fail(c, 503, err)
		return
	}
	data.RawDays = days
	response.Success(c, data)
}
