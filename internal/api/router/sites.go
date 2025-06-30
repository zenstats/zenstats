package router

import (
	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/api/sites"
	"github.com/zenstats/zenstats/internal/middleware"
)

func RegisterSitesRouter(router *gin.RouterGroup) {
	siteHandle := sites.NewSitesHandler()

	site := router.Use(middleware.JWTAuth())
	// 域名列表
	site.GET("/sites", siteHandle.List())
	// 域名详情
	site.GET("/sites/:id", siteHandle.Info())
	// 新增域名
	site.POST("/sites", siteHandle.Create())
	// 编辑域名
	site.PUT("/sites/:id", siteHandle.Update())
	// 删除域名
	site.DELETE("/sites/:id", siteHandle.Delete())
}
