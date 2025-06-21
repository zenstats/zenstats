package router

import (
	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/api/external"
)

func RegisterExternalAPI(router *gin.Engine) {
	// 创建路由组并应用中间件
	v1 := router.Group("/api")
	// v1.Use(
	// 	middleware.RequestID(),         // 请求ID追踪
	// 	middleware.JSONLogMiddleware(), // JSON日志
	// )

	v1.POST("/event", external.Event())
}
