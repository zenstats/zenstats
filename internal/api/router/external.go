package router

import (
	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/api/external"
	"github.com/zenstats/zenstats/internal/service"
)

// RegisterExternalRouter 注册外部事件采集相关路由。
// 提供事件数据上报接口，供前端埋点 SDK 调用。
func RegisterExternalRouter(router *gin.RouterGroup) {
	siteService := service.GetSiteService()

	router.POST("/event", external.Event(siteService))
}
