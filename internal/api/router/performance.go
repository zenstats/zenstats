package router

import (
	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/api/external"
	"github.com/zenstats/zenstats/internal/api/heatmaps"
	"github.com/zenstats/zenstats/internal/api/performance"
	"github.com/zenstats/zenstats/internal/middleware"
	"github.com/zenstats/zenstats/internal/service"
)

func RegisterPerformanceRouter(r *gin.RouterGroup) {
	registerPerformanceRouterWithService(r, service.GetPerformanceService())
}
func registerPerformanceRouterWithService(r *gin.RouterGroup, s *service.PerformanceService) {
	r.GET("/performance/config", external.PerformanceConfig(s))
	r.POST("/performance/reports", external.PerformanceReports(s))
	h := performance.NewHandler(s)
	g := r.Group("/sites/:domain/performance", middleware.JWTAuth(), middleware.SiteMembershipAuth())
	g.GET("/config", h.Config)
	g.GET("/data", h.Data)
	access := heatmaps.NewHandler(s.Shared)
	g.PUT("/config", middleware.SubAccountReadOnly(), access.WriteAccess(), h.SaveConfig)
	r.GET("/admin/performance-quality", middleware.JWTAuth(), middleware.AdminAuth(), h.Quality)
}
