package sites

import (
	"github.com/zenstats/zenstats/internal/service"
)

type SitesHandler struct {
	service *service.SiteService
}

func NewSitesHandler() *SitesHandler {
	service := service.GetSiteService()

	return &SitesHandler{service: service}
}
