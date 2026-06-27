package router

import (
	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/api/admin"
	"github.com/zenstats/zenstats/internal/middleware"
)

// RegisterAdminRouter 注册管理员相关路由。
// 包括用户管理、套餐管理、系统配置和系统统计接口。
func RegisterAdminRouter(router *gin.RouterGroup) {
	adminHandler := admin.NewAdminHandler()
	configHandler := admin.NewSystemConfigHandler()
	engineHandler := admin.NewSearchEngineHandler()

	adminGroup := router.Group("/admin")
	adminGroup.Use(middleware.JWTAuth())
	adminGroup.Use(middleware.AdminAuth())

	// 用户管理
	adminGroup.GET("/users", adminHandler.ListUsers())
	adminGroup.GET("/users/:userId", adminHandler.GetUser())
	adminGroup.PUT("/users/:userId/group", adminHandler.UpdateUserGroup())
	adminGroup.PUT("/users/:userId/status", adminHandler.UpdateUserStatus())

	// 套餐管理
	adminGroup.GET("/groups", adminHandler.ListGroups())
	adminGroup.POST("/groups", adminHandler.CreateGroup())
	adminGroup.PUT("/groups/:groupId", adminHandler.UpdateGroup())
	adminGroup.DELETE("/groups/:groupId", adminHandler.DeleteGroup())

	// 站点管理
	adminGroup.GET("/sites", adminHandler.ListSites())
	adminGroup.DELETE("/sites/:siteId", adminHandler.DeleteSite())
	adminGroup.PUT("/sites/:siteId/verify", adminHandler.VerifySite())
	adminGroup.POST("/sites/:siteId/traffic-alert/test", adminHandler.TestTrafficAlert())

	// 系统配置
	adminGroup.GET("/configs", configHandler.GetConfigs())
	adminGroup.PUT("/configs", configHandler.UpdateConfigs())

	// 内置来源管理
	adminGroup.GET("/sources", engineHandler.ListSearchEngines())
	adminGroup.POST("/sources", engineHandler.CreateSearchEngine())
	adminGroup.PUT("/sources/:id", engineHandler.UpdateSearchEngine())
	adminGroup.DELETE("/sources/:id", engineHandler.DeleteSearchEngine())

	// 系统统计
	adminGroup.GET("/stats", adminHandler.GetStats())
}
