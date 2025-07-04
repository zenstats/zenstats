package router

import (
	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/api/state"
)

func RegisterStatsRouter(router *gin.RouterGroup) {

	handle := state.NewStateHandle()

	router.Group("/stats")

	router.GET("/:site/top_stats", handle.GetTopStats())
}
