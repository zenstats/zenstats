package router

import (
	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/api/funnels"
	"github.com/zenstats/zenstats/internal/middleware"
)

// RegisterFunnelsRouter 注册漏斗管理相关路由。
// 包括漏斗的增删改查接口，所有接口均需要 JWT 认证。
// 漏斗分析接口支持 API Key 或 JWT 认证。
func RegisterFunnelsRouter(router *gin.RouterGroup) {
	funnelHandle := funnels.NewFunnelsHandler()

	// 漏斗管理接口（需要 JWT 认证）
	funnelsGroup := router.Group("/sites/:domain/funnels", middleware.JWTAuth(), middleware.SiteMembershipAuth())
	funnelsGroup.GET("", funnelHandle.List())
	funnelsGroup.GET("/:funnelId", funnelHandle.Get())
	funnelsGroup.POST("", middleware.SubAccountHasPerm("funnels:write"), funnelHandle.Create())
	funnelsGroup.PUT("/:funnelId", middleware.SubAccountHasPerm("funnels:write"), funnelHandle.Update())
	funnelsGroup.DELETE("/:funnelId", middleware.SubAccountHasPerm("funnels:write"), funnelHandle.Delete())

	// 漏斗分析接口（支持 API Key 或 JWT 认证）
	router.GET("/stats/:domain/funnel/:funnelId", middleware.APIKeyOrJWTAuth(), middleware.SiteMembershipAndVerificationAuth(), funnelHandle.Analyze())
}
