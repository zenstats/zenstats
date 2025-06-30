package router

import (
	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/api/external"
	"github.com/zenstats/zenstats/internal/service"
)

func RegisterExternalRouter(router *gin.RouterGroup) {
	siteService := service.GetSiteService()

	router.POST("/event", external.Event(siteService))
}
