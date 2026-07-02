package router

import (
	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/api/stats"
	"github.com/zenstats/zenstats/internal/middleware"
)

// RegisterStatsRouter Register stats analysis routes.
func RegisterStatsRouter(router *gin.RouterGroup) {

	handle := stats.NewStatsHandle()

	statsGroup := router.Group("/stats", middleware.SharedLinkOrAPIKeyOrJWTAuth(), middleware.SharedLinkOrSiteMembershipAndVerificationAuth())

	// Overview metrics (aggregate data with comparison)
	statsGroup.GET("/:domain/aggregate", handle.GetAggregate())
	// Time series (main graph)
	statsGroup.GET("/:domain/main-graph", handle.GetMainGraph())
	// Dimension breakdown (source/page/device/country rankings)
	statsGroup.GET("/:domain/breakdown", handle.GetBreakdown())
	// Export breakdown as CSV
	statsGroup.GET("/:domain/export", handle.ExportBreakdown())
	// Real-time current visitors
	statsGroup.GET("/:domain/current-visitors", handle.GetCurrentVisitors())
	// Time series (alias)
	statsGroup.GET("/:domain/time_series", handle.GetTimeSeries())
	// Filter suggestions (autocomplete)
	statsGroup.GET("/:domain/suggestions", handle.GetSuggestions())

	// v2 unified query API (POST body, multi-dimension / order_by / offset)
	v2Group := router.Group("/v2", middleware.APIKeyOrJWTAuth(), middleware.SiteMembershipAndVerificationAuth())
	v2Group.POST("/query/:domain", handle.QueryV2())
}
