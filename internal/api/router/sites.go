package router

import (
	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/api/emailreport"
	"github.com/zenstats/zenstats/internal/api/segments"
	"github.com/zenstats/zenstats/internal/api/sharedlinks"
	"github.com/zenstats/zenstats/internal/api/sites"
	"github.com/zenstats/zenstats/internal/api/trafficalert"
	"github.com/zenstats/zenstats/internal/middleware"
)

// RegisterSitesRouter 注册站点管理相关路由。
// 包括站点的增删改查以及 IP、Hostname、Country 屏蔽规则的管理接口。
// 所有接口均需要 JWT 认证。
func RegisterSitesRouter(router *gin.RouterGroup) {
	siteHandle := sites.NewSitesHandler()

	site := router.Group("", middleware.JWTAuth())
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
	shieldRules.GET("/ip", middleware.SubAccountHasPerm("shields:write"), siteHandle.ListShieldRuleIP())
	shieldRules.POST("/ip", middleware.SubAccountHasPerm("shields:write"), siteHandle.AddShieldRuleIP())
	shieldRules.DELETE("/ip/:ruleId", middleware.SubAccountHasPerm("shields:write"), siteHandle.RemoveShieldRuleIP())

	// Hostname Rules
	shieldRules.GET("/hostname", middleware.SubAccountHasPerm("shields:write"), siteHandle.ListShieldRuleHostname())
	shieldRules.POST("/hostname", middleware.SubAccountHasPerm("shields:write"), siteHandle.AddShieldRuleHostname())
	shieldRules.DELETE("/hostname/:ruleId", middleware.SubAccountHasPerm("shields:write"), siteHandle.RemoveShieldRuleHostname())

	// Country Rules
	shieldRules.GET("/country", middleware.SubAccountHasPerm("shields:write"), siteHandle.ListShieldRuleCountry())
	shieldRules.POST("/country", middleware.SubAccountHasPerm("shields:write"), siteHandle.AddShieldRuleCountry())
	shieldRules.DELETE("/country/:ruleId", middleware.SubAccountHasPerm("shields:write"), siteHandle.RemoveShieldRuleCountry())

	// Referrer Rules
	shieldRules.GET("/referrer", middleware.SubAccountHasPerm("shields:write"), siteHandle.ListShieldRuleReferrer())
	shieldRules.POST("/referrer", middleware.SubAccountHasPerm("shields:write"), siteHandle.AddShieldRuleReferrer())
	shieldRules.DELETE("/referrer/:ruleId", middleware.SubAccountHasPerm("shields:write"), siteHandle.RemoveShieldRuleReferrer())

	// Email Reports
	emailReportHandle := emailreport.NewHandler()
	emailReports := router.Group("sites/:domain/email-reports", middleware.JWTAuth(), middleware.SiteMembershipAuth())
	emailReports.GET("", middleware.SubAccountHasPerm("email_reports:write"), emailReportHandle.Get)
	emailReports.PUT("", middleware.SubAccountHasPerm("email_reports:write"), emailReportHandle.Update)

	// Traffic Alerts
	alertHandle := trafficalert.NewHandler()
	alerts := router.Group("sites/:domain/traffic-alert", middleware.JWTAuth(), middleware.SiteMembershipAuth())
	alerts.GET("", middleware.SubAccountHasPerm("traffic_alerts:write"), alertHandle.Get)
	alerts.PUT("", middleware.SubAccountHasPerm("traffic_alerts:write"), alertHandle.Update)

	// Shared Links
	slHandle := sharedlinks.NewHandler()
	sharedLinksGroup := router.Group("sites/:domain/shared-links", middleware.JWTAuth(), middleware.SiteMembershipAuth())
	sharedLinksGroup.GET("", slHandle.List())
	sharedLinksGroup.POST("", middleware.SubAccountHasPerm("shared_links:write"), slHandle.Create())
	sharedLinksGroup.DELETE("/:linkId", middleware.SubAccountHasPerm("shared_links:write"), slHandle.Delete())

	// Segments
	segHandle := segments.NewHandler()
	segGroup := router.Group("sites/:domain/segments", middleware.JWTAuth(), middleware.SiteMembershipAuth())
	segGroup.GET("", segHandle.List())
	segGroup.POST("", middleware.SubAccountHasPerm("segments:write"), segHandle.Create())
	segGroup.PATCH("/:segmentId", middleware.SubAccountHasPerm("segments:write"), segHandle.Update())
	segGroup.DELETE("/:segmentId", middleware.SubAccountHasPerm("segments:write"), segHandle.Delete())
}
