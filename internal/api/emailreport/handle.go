// Package emailreport HTTP request handler。
package emailreport

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/service"
	"github.com/zenstats/zenstats/pkg/globals"
	"github.com/zenstats/zenstats/pkg/response"
)

// Handler Email report configuration handler.
type Handler struct {
	siteService *service.SiteService
}

// NewHandler Creates a new Handler.
func NewHandler() *Handler {
	return &Handler{
		siteService: service.GetSiteService(),
	}
}

// Get Get site email report config GET /api/sites/:domain/email-reports
func (h *Handler) Get(c *gin.Context) {
	domain := c.Param("domain")

	site, err := h.siteService.GetSiteByDomain(c, domain)
	if err != nil {
		response.Error(c, http.StatusNotFound, err)
		return
	}

	response.Success(c, gin.H{
		"weekly":  site.EmailReportWeekly,
		"monthly": site.EmailReportMonthly,
	})
}

// Update Update site email report config PUT /api/sites/:domain/email-reports
func (h *Handler) Update(c *gin.Context) {
	siteID := c.GetInt64("site_id")

	var req struct {
		Weekly  *bool `json:"weekly"`
		Monthly *bool `json:"monthly"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	db := globals.GetDB()
	if db == nil || db.Client == nil {
		response.Error(c, http.StatusInternalServerError, errors.New("database unavailable"))
		return
	}

	update := db.Client.Site.UpdateOneID(siteID)
	if req.Weekly != nil {
		update = update.SetEmailReportWeekly(*req.Weekly)
	}
	if req.Monthly != nil {
		update = update.SetEmailReportMonthly(*req.Monthly)
	}

	site, err := update.Save(c)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	// Clear domain cache to prevent stale GET values
	h.siteService.InvalidateDomainCache(site.Domain)

	response.Success(c, gin.H{
		"weekly":  site.EmailReportWeekly,
		"monthly": site.EmailReportMonthly,
	})
}
