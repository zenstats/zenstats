package router

import "github.com/gin-gonic/gin"

func RegisterRouter(router *gin.RouterGroup) {
	// event api
	RegisterExternalRouter(router)

	// auth api
	RegisterAuthRouter(router)

	// user api
	RegisterUserRouter(router)

	// site api
	RegisterSitesRouter(router)
}
