// Package router 定义 API 路由注册，将所有路由分组注册到 gin 引擎。
package router

import (
	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/api/health"
)

// RegisterRouter 注册所有 API 路由到指定的路由组。
// 包括外部事件采集、认证、统计分析、用户管理、站点管理、API Key 管理和管理员管理等路由。
func RegisterRouter(router *gin.RouterGroup) {
	// health check
	router.GET("/health", health.NewHandler().Health())
	// event api
	RegisterExternalRouter(router)
	// auth api
	RegisterAuthRouter(router)
	// stats api
	RegisterStatsRouter(router)
	// user api
	RegisterUserRouter(router)
	// site api
	RegisterSitesRouter(router)
	// apikey api
	RegisterAPIKeyRouter(router)
	// goals api
	RegisterGoalsRouter(router)
	// funnels api
	RegisterFunnelsRouter(router)
	// import api (GA4 historical data import)
	RegisterImportRouter(router)
	// admin api
	RegisterAdminRouter(router)
}
