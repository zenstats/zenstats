package router

import (
	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/api/user"
	"github.com/zenstats/zenstats/internal/middleware"
)

// RegisterUserRouter 注册用户相关路由。
// 包括自定义搜索引擎管理和子账号管理等接口。
func RegisterUserRouter(router *gin.RouterGroup) {
	userHandler := user.NewUserHandler()

	userRouter := router.Group("/user")
	userRouter.Use(middleware.JWTAuth())

	// 自定义搜索引擎管理
	userRouter.GET("/search-engines", userHandler.ListSearchEngines())
	userRouter.POST("/search-engines", middleware.SubAccountReadOnly(), userHandler.CreateSearchEngine())
	userRouter.PUT("/search-engines/:id", middleware.SubAccountReadOnly(), userHandler.UpdateSearchEngine())
	userRouter.DELETE("/search-engines/:id", middleware.SubAccountReadOnly(), userHandler.DeleteSearchEngine())

	// 子账号管理
	userRouter.GET("/sub-accounts", userHandler.ListSubAccounts())
	userRouter.POST("/sub-accounts", middleware.SubAccountReadOnly(), userHandler.CreateSubAccount())
	userRouter.PUT("/sub-accounts/:id", middleware.SubAccountReadOnly(), userHandler.UpdateSubAccount())
	userRouter.DELETE("/sub-accounts/:id", middleware.SubAccountReadOnly(), userHandler.DeleteSubAccount())
	userRouter.POST("/sub-accounts/:id/reset-password", middleware.SubAccountReadOnly(), userHandler.ResetSubAccountPassword())

	// 用户额度
	userRouter.GET("/quota", userHandler.GetQuota())

	// 用户资料
	userRouter.PUT("/profile", middleware.SubAccountReadOnly(), userHandler.UpdateProfile())
}
