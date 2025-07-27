package router

import (
	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/api/state"
)

func RegisterStatsRouter(router *gin.RouterGroup) {

	handle := state.NewStateHandle()

	router.Group("/stats")

	router.GET("/:domain/top_stats", handle.GetTopStats())
	router.GET("/:domain/curve", handle.GetCurve())
	router.GET("/:domain/device_rank", handle.GetDeviceRank())
	router.GET("/:domain/source_rank", handle.GetSourceRank())
	router.GET("/:domain/page_rank", handle.GetPageRank())
	router.GET("/:domain/meta", handle.MetaStats())
}
