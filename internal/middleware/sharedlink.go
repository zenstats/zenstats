package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/service"
	"github.com/zenstats/zenstats/pkg/response"
)

// SharedLinkOrAPIKeyOrJWTAuth 优先尝试 shared link slug 鉴权，否则回退到 API Key / JWT 鉴权。
// 通过 ?slug= 查询参数传入共享链接的 slug。
func SharedLinkOrAPIKeyOrJWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if slug := c.Query("slug"); slug != "" {
			link, err := service.GetSharedLinkService().GetBySlug(c, slug)
			if err == nil {
				// slug 有效，将 site_id 和鉴权类型写入 context
				c.Set("shared_link_site_id", link.SiteID)
				c.Set("auth_type", "shared_link")
				c.Next()
				return
			}
		}
		// 无 slug 或 slug 无效，走正常鉴权
		APIKeyOrJWTAuth()(c)
	}
}

// SharedLinkOrSiteMembershipAndVerificationAuth 在 shared link 鉴权模式下跳过成员权限检查，
// 直接通过 slug 对应的 site_id 鉴权；普通用户则走完整的成员 + 验证状态检查。
func SharedLinkOrSiteMembershipAndVerificationAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if authType, _ := c.Get("auth_type"); authType == "shared_link" {
			// shared link 模式：通过 slug 已经拿到了 site_id，验证 domain 参数是否与之一致
			sharedSiteID := c.GetInt64("shared_link_site_id")
			domain := c.Param("domain")
			if domain == "" {
				response.ErrorWithKey(c, http.StatusBadRequest, "sites.domain_required")
				c.Abort()
				return
			}
			siteService := service.GetSiteService()
			site, err := siteService.GetSiteByDomain(c.Request.Context(), domain)
			if err != nil || site.ID != sharedSiteID {
				response.ErrorWithKey(c, http.StatusForbidden, "auth.invalid_token")
				c.Abort()
				return
			}
			c.Set("site_id", site.ID)
			c.Next()
			return
		}
		// 正常用户鉴权
		SiteMembershipAndVerificationAuth()(c)
	}
}
