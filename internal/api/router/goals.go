package router

import (
	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/api/goals"
	"github.com/zenstats/zenstats/internal/middleware"
)

// RegisterGoalsRouter 注册目标管理相关路由。
// 包括目标的增删改查接口，所有接口均需要 JWT 认证。
func RegisterGoalsRouter(router *gin.RouterGroup) {
	goalHandle := goals.NewGoalsHandler()

	goalsGroup := router.Group("/sites/:domain/goals", middleware.JWTAuth())
	goalsGroup.GET("", goalHandle.List())
	goalsGroup.POST("", goalHandle.Create())
	goalsGroup.PUT("/:goalId", goalHandle.Update())
	goalsGroup.DELETE("/:goalId", goalHandle.Delete())
}
