// Package emailreport 处理站点邮件报告配置的 HTTP 请求。
package emailreport

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/service"
	"github.com/zenstats/zenstats/pkg/globals"
	"github.com/zenstats/zenstats/pkg/response"
)

// Handler 邮件报告配置处理器。
type Handler struct {
	siteService *service.SiteService
}

// NewHandler 创建新的 Handler。
func NewHandler() *Handler {
	return &Handler{
		siteService: service.GetSiteService(),
	}
}

// Get 获取站点邮件报告配置 GET /api/sites/:domain/email-reports
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

// Update 更新站点邮件报告配置 PUT /api/sites/:domain/email-reports
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
		response.Error(c, http.StatusInternalServerError, nil)
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

	// 清除域名缓存，避免 GET 返回旧值
	h.siteService.InvalidateDomainCache(site.Domain)

	response.Success(c, gin.H{
		"weekly":  site.EmailReportWeekly,
		"monthly": site.EmailReportMonthly,
	})
}
