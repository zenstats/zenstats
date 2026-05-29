package router

import (
	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/api/auth"
	"github.com/zenstats/zenstats/internal/middleware"
)

// RegisterAuthRouter 注册认证相关路由。
// 包括用户登录、令牌刷新、系统状态检查和系统初始化接口。
func RegisterAuthRouter(router *gin.RouterGroup) {
	authHandle := auth.NewAuthHandler()

	auth := router.Group("/auth")

	auth.POST("/login", authHandle.Login())
	auth.GET("/refresh", authHandle.Refresh())

	auth.GET("/state", authHandle.State())

	auth.Use(middleware.CheckInitialization()).POST("/init", authHandle.Initialize())
}
