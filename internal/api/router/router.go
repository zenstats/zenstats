package router

import "github.com/gin-gonic/gin"

func RegisterRouter(router *gin.RouterGroup) {
	// event api
	RegisterExternalRouter(router)

	// user api
	RegisterUserRouter(router)
}
