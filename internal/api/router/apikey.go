package router

import (
	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/api/apikeys"
	"github.com/zenstats/zenstats/internal/middleware"
)

// RegisterAPIKeyRouter 注册 API Key 管理相关路由。
// 包括 API Key 的创建、列表和删除接口，均需 JWT 认证。
func RegisterAPIKeyRouter(router *gin.RouterGroup) {
	handler := apikeys.NewAPIKeyHandler()

	apikeyGroup := router.Group("/apikeys", middleware.JWTAuth())

	apikeyGroup.GET("", handler.List())
	apikeyGroup.POST("", middleware.SubAccountReadOnly(), handler.Create())
	apikeyGroup.DELETE("/:id", middleware.SubAccountReadOnly(), handler.Delete())
}
