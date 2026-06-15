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
		// 从 Header 或 Query 中获取 Token
		tokenString := c.GetHeader("Authorization")
		if tokenString == "" {
			tokenString = c.Query("token")
		}
		if len(tokenString) > 7 && tokenString[:7] == "Bearer " {
			tokenString = tokenString[7:]
		}
		if tokenString == "" {
			response.ErrorWithKey(c, http.StatusUnauthorized, "auth.token_not_provided")
			c.Abort()
			return
		}

		// 解析 Token
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

		// 将用户信息存入 Context
		c.Set("user_id", claims.UserID)
		c.Set("user_type", claims.UserType)
		c.Set("sub_account_id", claims.SubAccountID)
		c.Set("role", claims.Role)
		c.Next()
	}
}

// AdminAuth 管理员权限检查中间件
func AdminAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			response.ErrorWithKey(c, http.StatusUnauthorized, "auth.user_not_found")
			c.Abort()
			return
		}

		// 子账号不能访问管理员接口
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

// SubAccountReadOnly 子账号只读中间件，阻止子账号执行写操作
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

// SiteMembershipAuth 站点成员鉴权中间件，验证当前用户是否为请求站点的成员。
// 验证通过后将用户所属的具体站点 ID 存入 context（key: "site_id"）。
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

// SiteMembershipAndVerificationAuth 站点成员及验证鉴权中间件。
// 验证当前用户是否为请求站点的成员，且该域名下存在已验证的站点。
// 用于需要同时检查成员权限和站点验证状态的接口（如统计查询）。
// 验证通过后将用户所属的具体站点 ID 存入 context（key: "site_id"）。
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

		// 检查成员权限：用户是否为该域名站点的成员
		site, err := siteService.CheckSiteMembership(c.Request.Context(), userID.(int64), domain)
		if err != nil {
			response.ErrorWithKey(c, http.StatusNotFound, "sites.not_found")
			c.Abort()
			return
		}

		// 检查验证状态：该域名下是否存在已验证的站点
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
