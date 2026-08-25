package middleware

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/zenstats/zenstats/internal/auth"
	"github.com/zenstats/zenstats/internal/service"
	"github.com/zenstats/zenstats/pkg/response"
)

func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get token from Header or Query
		// 仅从 Authorization 头读取凭据。不再接受 ?token= 查询参数：
		// 令牌会随 URL 进入网关/后端访问日志与 Referer，泄露风险高。
		tokenString := c.GetHeader("Authorization")
		if len(tokenString) > 7 && tokenString[:7] == "Bearer " {
			tokenString = tokenString[7:]
		}
		if tokenString == "" {
			response.ErrorWithKey(c, http.StatusUnauthorized, "auth.token_not_provided")
			c.Abort()
			return
		}

		// Parse token
		claims, err := auth.ParseToken(tokenString)
		if err != nil {
			if errors.Is(err, jwt.ErrTokenExpired) {
				response.ErrorWithKey(c, 430, "auth.token_expired")
				c.Abort()
				return
			}
			response.ErrorWithKey(c, http.StatusUnauthorized, "auth.invalid_token")
			c.Abort()
			return
		}
		// 刷新令牌（24h）只能用于 /auth/refresh，不能直接当访问令牌使用，
		// 否则访问令牌 1h 的有效期形同虚设。
		if claims.Subject != auth.SubjectAccessToken {
			response.ErrorWithKey(c, http.StatusUnauthorized, "auth.invalid_token")
			c.Abort()
			return
		}

		// Store user info in context
		c.Set("user_id", claims.UserID)
		c.Set("user_type", claims.UserType)
		c.Set("sub_account_id", claims.SubAccountID)
		c.Set("role", claims.Role)

		// Convert to map for O(1) lookup in SubAccountHasPerm
		permSet := make(map[string]struct{}, len(claims.Permissions))
		for _, p := range claims.Permissions {
			permSet[p] = struct{}{}
		}
		c.Set("permissions", permSet)
		c.Next()
	}
}

// AdminAuth checks whether the current user is an administrator
func AdminAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			response.ErrorWithKey(c, http.StatusUnauthorized, "auth.user_not_found")
			c.Abort()
			return
		}

		// Sub-accounts cannot access admin endpoints
		userType, _ := c.Get("user_type")
		if userType == "sub_account" {
			response.ErrorWithKey(c, http.StatusForbidden, "auth.admin_required")
			c.Abort()
			return
		}

		userService := service.GetUserService()
		isAdmin, err := userService.IsAdmin(c.Request.Context(), userID.(int64))
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			c.Abort()
			return
		}

		if !isAdmin {
			response.ErrorWithKey(c, http.StatusForbidden, "auth.admin_required")
			c.Abort()
			return
		}

		c.Next()
	}
}

// SubAccountReadOnly blocks all sub-accounts (regardless of role) from performing write operations.
// Used for parent-account exclusive endpoints (site create/delete, API key management, sub-account management).
func SubAccountReadOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		userType, _ := c.Get("user_type")
		if userType == "sub_account" {
			response.ErrorWithKey(c, http.StatusForbidden, "auth.sub_account_read_only")
			c.Abort()
			return
		}
		c.Next()
	}
}

// SubAccountHasPerm provides fine-grained permission checking.
// Parent accounts pass through; sub-accounts require the perm in their permissions set (O(1) map lookup).
func SubAccountHasPerm(perm string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userType, _ := c.Get("user_type")
		if userType != "sub_account" {
			c.Next()
			return
		}
		permsVal, _ := c.Get("permissions")
		if perms, ok := permsVal.(map[string]struct{}); ok {
			if _, has := perms[perm]; has {
				c.Next()
				return
			}
		}

		response.ErrorWithKey(c, http.StatusForbidden, "auth.sub_account_insufficient_role")
		c.Abort()
	}
}

// SiteMembershipAuth verifies the current user is a member of the requested site.
// On success, stores the site ID in context (key: "site_id").
func SiteMembershipAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			response.ErrorWithKey(c, http.StatusUnauthorized, "auth.user_not_found")
			c.Abort()
			return
		}

		domain := c.Param("domain")
		if domain == "" {
			response.ErrorWithKey(c, http.StatusBadRequest, "sites.domain_required")
			c.Abort()
			return
		}

		siteService := service.GetSiteService()
		site, err := siteService.CheckSiteMembership(c.Request.Context(), userID.(int64), domain)
		if err != nil {
			response.ErrorWithKey(c, http.StatusNotFound, "sites.not_found")
			c.Abort()
			return
		}

		c.Set("site_id", site.ID)
		c.Next()
	}
}

// SiteMembershipAndVerificationAuth checks both site membership and verification status.
// Verifies the user is a member and that a verified site exists for the domain.
// Used for endpoints requiring both membership and verification (e.g. stats queries).
// On success, stores the site ID in context (key: "site_id").
func SiteMembershipAndVerificationAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			response.ErrorWithKey(c, http.StatusUnauthorized, "auth.user_not_found")
			c.Abort()
			return
		}

		domain := c.Param("domain")
		if domain == "" {
			response.ErrorWithKey(c, http.StatusBadRequest, "sites.domain_required")
			c.Abort()
			return
		}

		siteService := service.GetSiteService()

		// Check membership: is the user a member of this domain
		site, err := siteService.CheckSiteMembership(c.Request.Context(), userID.(int64), domain)
		if err != nil {
			response.ErrorWithKey(c, http.StatusNotFound, "sites.not_found")
			c.Abort()
			return
		}

		// Check verification: does a verified site exist for this domain
		_, err = siteService.GetVerifiedSiteByDomain(c.Request.Context(), domain)
		if err != nil {
			response.ErrorWithKey(c, http.StatusForbidden, "sites.not_verified")
			c.Abort()
			return
		}

		c.Set("site_id", site.ID)
		c.Next()
	}
}
