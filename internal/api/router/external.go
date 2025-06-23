package router

import (
	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/api/external"
)

func RegisterExternalRouter(router *gin.RouterGroup) {

	router.POST("/event", external.Event())
}
