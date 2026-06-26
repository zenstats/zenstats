package router

import (
	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/api/stats"
	"github.com/zenstats/zenstats/internal/middleware"
)

// RegisterStatsRouter 注册统计分析相关路由。
func RegisterStatsRouter(router *gin.RouterGroup) {

	handle := stats.NewStatsHandle()

	statsGroup := router.Group("/stats", middleware.SharedLinkOrAPIKeyOrJWTAuth(), middleware.SharedLinkOrSiteMembershipAndVerificationAuth())

	// 总览指标（聚合数据，含对比）
	statsGroup.GET("/:domain/aggregate", handle.GetAggregate())
	// 时间序列（主图表）
	statsGroup.GET("/:domain/main-graph", handle.GetMainGraph())
	// 维度细分（来源/页面/设备/国家等排行）
	statsGroup.GET("/:domain/breakdown", handle.GetBreakdown())
	// 维度细分导出 CSV
	statsGroup.GET("/:domain/export", handle.ExportBreakdown())
	// 实时在线访客数
	statsGroup.GET("/:domain/current-visitors", handle.GetCurrentVisitors())
	// 时间序列（别名）
	statsGroup.GET("/:domain/time_series", handle.GetTimeSeries())
	// 筛选器建议（自动补全）
	statsGroup.GET("/:domain/suggestions", handle.GetSuggestions())

	// v2 通用查询 API（POST body，支持多维度 / order_by / offset）
	v2Group := router.Group("/api/v2", middleware.APIKeyOrJWTAuth(), middleware.SiteMembershipAndVerificationAuth())
	v2Group.POST("/query/:domain", handle.QueryV2())
}
