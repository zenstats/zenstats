// Package sites 处理站点管理相关的 HTTP 请求，包括站点的增删改查及屏蔽规则管理。
package sites

import (
	"github.com/zenstats/zenstats/internal/service"
)

// SitesHandler 站点管理处理器，封装站点服务以处理站点相关的 HTTP 请求。
type SitesHandler struct {
	service *service.SiteService
}

// NewSitesHandler 创建并返回一个新的 SitesHandler 实例。
func NewSitesHandler() *SitesHandler {
	service := service.GetSiteService()

	return &SitesHandler{service: service}
}
