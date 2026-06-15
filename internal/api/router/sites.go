package router

import (
	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/api/sites"
	"github.com/zenstats/zenstats/internal/middleware"
)

// RegisterSitesRouter 注册站点管理相关路由。
// 包括站点的增删改查以及 IP、Hostname、Country 屏蔽规则的管理接口。
// 所有接口均需要 JWT 认证。
func RegisterSitesRouter(router *gin.RouterGroup) {
	siteHandle := sites.NewSitesHandler()

	site := router.Use(middleware.JWTAuth())
	site.GET("/sites", siteHandle.List())
	site.GET("/sites/:domain", middleware.SiteMembershipAuth(), siteHandle.Info())
	site.POST("/sites", middleware.SubAccountReadOnly(), siteHandle.Create())
	site.PUT("/sites/:domain", middleware.SubAccountReadOnly(), middleware.SiteMembershipAuth(), siteHandle.Update())
	site.DELETE("/sites/:domain", middleware.SubAccountReadOnly(), middleware.SiteMembershipAuth(), siteHandle.Delete())

	// Site Verification
	site.GET("/sites/:domain/verification-status", middleware.SiteMembershipAuth(), siteHandle.VerificationStatus())
	site.POST("/sites/:domain/verify", middleware.SiteMembershipAuth(), siteHandle.Verify())

	// Shield Rules Management
	shieldRules := router.Group("sites/:domain/shield", middleware.JWTAuth(), middleware.SiteMembershipAuth())

	// IP Rules
	shieldRules.GET("/ip", siteHandle.ListShieldRuleIP())
	shieldRules.POST("/ip", middleware.SubAccountReadOnly(), siteHandle.AddShieldRuleIP())
	shieldRules.DELETE("/ip/:ruleId", middleware.SubAccountReadOnly(), siteHandle.RemoveShieldRuleIP())

	// Hostname Rules
	shieldRules.GET("/hostname", siteHandle.ListShieldRuleHostname())
	shieldRules.POST("/hostname", middleware.SubAccountReadOnly(), siteHandle.AddShieldRuleHostname())
	shieldRules.DELETE("/hostname/:ruleId", middleware.SubAccountReadOnly(), siteHandle.RemoveShieldRuleHostname())

	// Country Rules
	shieldRules.GET("/country", siteHandle.ListShieldRuleCountry())
	shieldRules.POST("/country", middleware.SubAccountReadOnly(), siteHandle.AddShieldRuleCountry())
	shieldRules.DELETE("/country/:ruleId", middleware.SubAccountReadOnly(), siteHandle.RemoveShieldRuleCountry())
}
