package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/service"
	"github.com/zenstats/zenstats/pkg/response"
)

// SharedLinkOrAPIKeyOrJWTAuth tries shared link slug auth first, falls back to API Key / JWT.
// Pass the shared link slug via the ?slug= query parameter.
func SharedLinkOrAPIKeyOrJWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if slug := c.Query("slug"); slug != "" {
			link, err := service.GetSharedLinkService().GetBySlug(c, slug)
			if err == nil {
				// Slug is valid, write site_id and auth type to context
				c.Set("shared_link_site_id", link.SiteID)
				c.Set("auth_type", "shared_link")
				c.Next()
				return
			}
		}
		// No slug or invalid slug, fall through to normal auth
		APIKeyOrJWTAuth()(c)
	}
}

// SharedLinkOrSiteMembershipAndVerificationAuth skips membership checks for shared link auth,
// authorizing via the slug-bound site_id; regular users go through full membership + verification checks.
func SharedLinkOrSiteMembershipAndVerificationAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if authType, _ := c.Get("auth_type"); authType == "shared_link" {
			// Shared link mode: site_id already obtained from slug, verify domain param matches
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
		// Normal user authentication
		SiteMembershipAndVerificationAuth()(c)
	}
}
