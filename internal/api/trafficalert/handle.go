// Package trafficalert HTTP request handler。
package trafficalert

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/service"
	"github.com/zenstats/zenstats/pkg/globals"
	"github.com/zenstats/zenstats/pkg/response"
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// Handler Traffic alert configuration handler.
type Handler struct {
	siteService *service.SiteService
}

// NewHandler Creates a new Handler.
func NewHandler() *Handler {
	return &Handler{
		siteService: service.GetSiteService(),
	}
}

// Get Get site traffic alert config GET /api/sites/:domain/traffic-alert
func (h *Handler) Get(c *gin.Context) {
	domain := c.Param("domain")

	site, err := h.siteService.GetSiteByDomain(c, domain)
	if err != nil {
		response.Error(c, http.StatusNotFound, err)
		return
	}

	recipients := ""
	if site.TrafficAlertRecipients != nil {
		recipients = *site.TrafficAlertRecipients
	}

	response.Success(c, gin.H{
		"enabled":    site.TrafficAlertEnabled,
		"threshold":  site.TrafficAlertThreshold,
		"recipients": recipients,
		"interval":   site.TrafficAlertInterval,
	})
}

// Update Update site traffic alert config PUT /api/sites/:domain/traffic-alert
func (h *Handler) Update(c *gin.Context) {
	siteID := c.GetInt64("site_id")

	var req struct {
		Enabled    *bool   `json:"enabled"`
		Threshold  *int    `json:"threshold"`
		Recipients *string `json:"recipients"`
		Interval   *string `json:"interval"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	// 校验参数
	if req.Threshold != nil && (*req.Threshold < 10 || *req.Threshold > 500) {
		response.Error(c, http.StatusBadRequest, fmt.Errorf("threshold must be between 10 and 500"))
		return
	}
	if req.Interval != nil && *req.Interval != "hourly" && *req.Interval != "daily" {
		response.Error(c, http.StatusBadRequest, fmt.Errorf("interval must be 'hourly' or 'daily'"))
		return
	}
	if err := validateRecipients(req.Recipients); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	db := globals.GetDB()
	if db == nil || db.Client == nil {
		response.Error(c, http.StatusInternalServerError, errors.New("database unavailable"))
		return
	}

	update := db.Client.Site.UpdateOneID(siteID)
	if req.Enabled != nil {
		update = update.SetTrafficAlertEnabled(*req.Enabled)
	}
	if req.Threshold != nil {
		update = update.SetTrafficAlertThreshold(*req.Threshold)
	}
	if req.Recipients != nil {
		update = update.SetTrafficAlertRecipients(*req.Recipients)
	}
	if req.Interval != nil {
		update = update.SetTrafficAlertInterval(*req.Interval)
	}

	site, err := update.Save(c)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	h.siteService.InvalidateDomainCache(site.Domain)

	recipients := ""
	if site.TrafficAlertRecipients != nil {
		recipients = *site.TrafficAlertRecipients
	}

	response.Success(c, gin.H{
		"enabled":    site.TrafficAlertEnabled,
		"threshold":  site.TrafficAlertThreshold,
		"recipients": recipients,
		"interval":   site.TrafficAlertInterval,
	})
}

// validateRecipients Validate email format for each recipient.
func validateRecipients(recipients *string) error {
	if recipients == nil || *recipients == "" {
		return nil
	}
	parts := strings.Split(*recipients, ",")
	for _, p := range parts {
		email := strings.TrimSpace(p)
		if email == "" {
			continue
		}
		if !emailRegex.MatchString(email) {
			return fmt.Errorf("invalid email: %s", email)
		}
	}
	return nil
}
