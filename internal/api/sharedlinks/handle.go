// Package sharedlinks 处理共享链接（可嵌入的公开仪表盘链接）管理。
package sharedlinks

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/service"
	"github.com/zenstats/zenstats/pkg/response"
)

// Handler 共享链接处理器。
type Handler struct {
	siteService *service.SiteService
}

// NewHandler 创建共享链接处理器实例。
func NewHandler() *Handler {
	return &Handler{
		siteService: service.GetSiteService(),
	}
}

// CreateSharedLinkRequest 创建共享链接请求。
type CreateSharedLinkRequest struct {
	Name     string `json:"name" binding:"required,max=255"`
	Password string `json:"password"` // 可选密码保护
}

// SharedLinkResponse 共享链接响应。
type SharedLinkResponse struct {
	ID               int64  `json:"id"`
	Name             string `json:"name"`
	Slug             string `json:"slug"`
	URL              string `json:"url"`
	PasswordProtected bool  `json:"password_protected"`
	CreatedAt        string `json:"created_at"`
}

// List 获取站点的所有共享链接。
//
//	@Summary		获取共享链接列表
//	@Description	获取指定站点的所有可嵌入共享链接。
//	@Tags			共享链接
//	@Security		BearerAuth
//	@Produce		json
//	@Param			domain	path		string	true	"站点域名"
//	@Success		200		{object}	response.SuccessResponse{data=[]SharedLinkResponse}
//	@Failure		401		{object}	response.ErrorResponse
//	@Router			/sites/{domain}/shared-links [get]
func (h *Handler) List() gin.HandlerFunc {
	return func(c *gin.Context) {
		siteID := c.GetInt64("site_id")
		links, err := service.GetSharedLinkService().List(c, siteID)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}
		result := make([]SharedLinkResponse, len(links))
		for i, l := range links {
			result[i] = toResponse(l, schemeFromContext(c), c.Request.Host)
		}
		response.Success(c, result)
	}
}

// Create 创建新的共享链接。
//
//	@Summary		创建共享链接
//	@Description	为站点创建可公开嵌入的仪表盘共享链接，支持可选密码保护。
//	@Tags			共享链接
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			domain	path		string					true	"站点域名"
//	@Param			body	body		CreateSharedLinkRequest	true	"共享链接参数"
//	@Success		201		{object}	response.SuccessResponse{data=SharedLinkResponse}
//	@Failure		400		{object}	response.ErrorResponse
//	@Failure		401		{object}	response.ErrorResponse
//	@Router			/sites/{domain}/shared-links [post]
func (h *Handler) Create() gin.HandlerFunc {
	return func(c *gin.Context) {
		siteID := c.GetInt64("site_id")

		var req CreateSharedLinkRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		slug, err := generateSlug()
		if err != nil {
			response.Error(c, http.StatusInternalServerError, fmt.Errorf("failed to generate slug"))
			return
		}

		link, err := service.GetSharedLinkService().Create(c, siteID, req.Name, slug, req.Password)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		response.Success(c, toResponse(link, schemeFromContext(c), c.Request.Host))
	}
}

// Delete 删除共享链接。
//
//	@Summary		删除共享链接
//	@Description	删除指定共享链接，链接立即失效。
//	@Tags			共享链接
//	@Security		BearerAuth
//	@Produce		json
//	@Param			domain		path		string	true	"站点域名"
//	@Param			linkId		path		string	true	"链接 ID"
//	@Success		200			{object}	response.SuccessResponse{data=map[string]bool}
//	@Failure		404			{object}	response.ErrorResponse
//	@Router			/sites/{domain}/shared-links/{linkId} [delete]
func (h *Handler) Delete() gin.HandlerFunc {
	return func(c *gin.Context) {
		siteID := c.GetInt64("site_id")
		linkIDStr := c.Param("linkId")
		linkID, err := strconv.ParseInt(linkIDStr, 10, 64)
		if err != nil {
			response.Error(c, http.StatusBadRequest, fmt.Errorf("invalid link id"))
			return
		}

		if err := service.GetSharedLinkService().Delete(c, siteID, linkID); err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		response.Success(c, gin.H{"deleted": true})
	}
}

// SharedLinkViewResponse 共享链接公开访问响应（含站点域名，供前端加载统计）。
type SharedLinkViewResponse struct {
	Slug              string `json:"slug"`
	Name              string `json:"name"`
	Domain            string `json:"domain"`
	PasswordProtected bool   `json:"password_protected"`
}

// View 公开接口，通过 slug 获取共享链接信息。
//
//	@Summary		获取共享链接信息
//	@Description	公开接口，根据 slug 返回共享链接关联的站点域名，供前端渲染只读统计页。
//	@Tags			共享链接
//	@Produce		json
//	@Param			slug	path		string	true	"链接 slug"
//	@Success		200		{object}	response.SuccessResponse{data=SharedLinkViewResponse}
//	@Failure		404		{object}	response.ErrorResponse
//	@Router			/share/{slug} [get]
func (h *Handler) View() gin.HandlerFunc {
	return func(c *gin.Context) {
		slug := c.Param("slug")
		link, err := service.GetSharedLinkService().GetBySlug(c, slug)
		if err != nil {
			response.Error(c, http.StatusNotFound, fmt.Errorf("shared link not found"))
			return
		}
		// 通过 site_id 查询 domain
		site, err := h.siteService.GetSiteByID(c, int(link.SiteID))
		if err != nil {
			response.Error(c, http.StatusNotFound, fmt.Errorf("site not found"))
			return
		}
		response.Success(c, SharedLinkViewResponse{
			Slug:              link.Slug,
			Name:              link.Name,
			Domain:            site.Domain,
			PasswordProtected: link.PasswordHash != "",
		})
	}
}

func generateSlug() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func schemeFromContext(c *gin.Context) string {
	if proto := c.GetHeader("X-Forwarded-Proto"); proto != "" {
		return proto
	}
	if c.Request.TLS != nil {
		return "https"
	}
	return "http"
}

func toResponse(l *service.SharedLink, scheme, host string) SharedLinkResponse {
	url := fmt.Sprintf("%s://%s/share/%s", scheme, host, l.Slug)
	return SharedLinkResponse{
		ID:               l.ID,
		Name:             l.Name,
		Slug:             l.Slug,
		URL:              url,
		PasswordProtected: l.PasswordHash != "",
		CreatedAt:        l.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
}
