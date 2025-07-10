package router

import (
	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/api/sites"
	"github.com/zenstats/zenstats/internal/middleware"
)

func RegisterSitesRouter(router *gin.RouterGroup) {
	siteHandle := sites.NewSitesHandler()

	site := router.Use(middleware.JWTAuth())
	// 域名列表
	site.GET("/sites", siteHandle.List())
	// 域名详情
	site.GET("/sites/:domain", siteHandle.Info())
	// 新增域名
	site.POST("/sites", siteHandle.Create())
	// 编辑域名
	site.PUT("/sites/:domain", siteHandle.Update())
	// 删除域名
	site.DELETE("/sites/:domain", siteHandle.Delete())

	// Shield Rules Management
	shieldRules := router.Group("sites/:domain/shield")

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
