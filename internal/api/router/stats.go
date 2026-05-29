package router

import (
	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/api/stats"
	"github.com/zenstats/zenstats/internal/middleware"
)

// RegisterStatsRouter 注册统计分析相关路由。
// 基于 Plausible 兼容 API 设计，支持 JWT Token 和 API Key 双重认证。
func RegisterStatsRouter(router *gin.RouterGroup) {

	handle := stats.NewStatsHandle()

	// Plausible 兼容接口（支持 API Key 和 JWT 认证）
	statsGroup := router.Group("/stats", middleware.APIKeyOrJWTAuth())

	// 总览指标（聚合数据，含对比）
	statsGroup.GET("/:domain/aggregate", handle.GetAggregate())
	// 时间序列（主图表）
	statsGroup.GET("/:domain/main-graph", handle.GetMainGraph())
	// 维度细分（来源/页面/设备/国家等排行，替代旧版 source_rank/device_rank/page_rank）
	statsGroup.GET("/:domain/breakdown", handle.GetBreakdown())
	// 实时在线访客数
	statsGroup.GET("/:domain/current-visitors", handle.GetCurrentVisitors())
	// 时间序列（别名）
	statsGroup.GET("/:domain/time_series", handle.GetTimeSeries())
}
