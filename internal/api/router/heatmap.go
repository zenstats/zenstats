package router

import (
	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/api/external"
	"github.com/zenstats/zenstats/internal/api/heatmaps"
	"github.com/zenstats/zenstats/internal/middleware"
	"github.com/zenstats/zenstats/internal/service"
)

func RegisterHeatmapRouter(router *gin.RouterGroup) {
	registerHeatmapRouterWithService(router, service.GetHeatmapService())
}

func registerHeatmapRouterWithService(router *gin.RouterGroup, s *service.HeatmapService) {
	router.GET("/heatmap/config", external.HeatmapConfig(s))
	router.POST("/heatmap/clicks", external.HeatmapClicks(s))
	router.POST("/heatmap/scroll", external.HeatmapScroll(s))
	h := heatmaps.NewHandler(s)
	g := router.Group("/sites/:domain/heatmap", middleware.JWTAuth(), middleware.SiteMembershipAuth())
	g.GET("/config", h.Config)
	g.GET("/pages", h.Pages)
	g.GET("/data", h.Data)
	g.GET("/scroll/pages", h.ScrollPages)
	g.GET("/scroll/data", h.ScrollData)
	g.GET("/snapshots", h.Snapshots)
	g.GET("/snapshots/:id/image", h.Image)
	// All members can inspect data; only parent-account site admins can write.
	w := g.Group("", middleware.SubAccountReadOnly(), h.WriteAccess())
	w.PUT("/config", h.SaveConfig)
	w.POST("/snapshots", h.CreateSnapshot)
	w.POST("/capture", h.Capture)
	w.DELETE("/snapshots/:id", h.DeleteSnapshot)
	w.POST("/replay", h.Replay)
	router.GET("/admin/heatmap-quality", middleware.JWTAuth(), middleware.AdminAuth(), h.Quality)
	router.GET("/admin/scroll-quality", middleware.JWTAuth(), middleware.AdminAuth(), h.ScrollQuality)
}
