package router

import (
	"github.com/gin-gonic/gin"
	imports "github.com/zenstats/zenstats/internal/api/import"
	"github.com/zenstats/zenstats/internal/middleware"
)

func RegisterImportRouter(router *gin.RouterGroup) {
	handle := imports.NewImportHandle()

	uploadGroup := router.Group("/sites/:domain/import", middleware.JWTAuth(), middleware.SiteMembershipAuth())
	uploadGroup.POST("/upload", handle.Upload())

	// 导入数据的查询接口与上传保持一致，仅需站点成员身份（不要求站点已验证）：
	// 导入的是历史/手动数据，与追踪脚本是否验证无关，未验证站点也应能查看导入结果。
	queryGroup := router.Group("/sites/:domain/import", middleware.APIKeyOrJWTAuth(), middleware.SiteMembershipAuth())
	queryGroup.GET("/aggregate", handle.GetAggregate())
	queryGroup.GET("/breakdown", handle.GetBreakdown())
	queryGroup.GET("/timeseries", handle.GetTimeSeries())
}
