// Package sites HTTP request handler，包括站点的增删改查及屏蔽规则管理。
package sites

import (
	"github.com/zenstats/zenstats/internal/service"
)

// SitesHandler 站点管理HTTP request handler。
type SitesHandler struct {
	service *service.SiteService
}

// NewSitesHandler creates and returns a new SitesHandler instance.
func NewSitesHandler() *SitesHandler {
	service := service.GetSiteService()

	return &SitesHandler{service: service}
}
