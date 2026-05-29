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
	site.GET("/sites/:domain", siteHandle.Info())
	site.POST("/sites", siteHandle.Create())
	site.PUT("/sites/:domain", siteHandle.Update())
	site.DELETE("/sites/:domain", siteHandle.Delete())

	// Shield Rules Management
	shieldRules := router.Group("sites/:domain/shield", middleware.JWTAuth())

	// IP Rules
	shieldRules.GET("/ip", siteHandle.ListShieldRuleIP())
	shieldRules.POST("/ip", siteHandle.AddShieldRuleIP())
	shieldRules.DELETE("/ip/:ruleId", siteHandle.RemoveShieldRuleIP())

	// Hostname Rules
	shieldRules.GET("/hostname", siteHandle.ListShieldRuleHostname())
	shieldRules.POST("/hostname", siteHandle.AddShieldRuleHostname())
	shieldRules.DELETE("/hostname/:ruleId", siteHandle.RemoveShieldRuleHostname())

	// Country Rules
	shieldRules.GET("/country", siteHandle.ListShieldRuleCountry())
	shieldRules.POST("/country", siteHandle.AddShieldRuleCountry())
	shieldRules.DELETE("/country/:ruleId", siteHandle.RemoveShieldRuleCountry())
}
