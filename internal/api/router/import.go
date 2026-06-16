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

	queryGroup := router.Group("/sites/:domain/import", middleware.APIKeyOrJWTAuth(), middleware.SiteMembershipAndVerificationAuth())
	queryGroup.GET("/aggregate", handle.GetAggregate())
	queryGroup.GET("/breakdown", handle.GetBreakdown())
	queryGroup.GET("/timeseries", handle.GetTimeSeries())
}
