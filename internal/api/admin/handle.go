package admin

import (
	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/middleware"
	"github.com/zenstats/zenstats/internal/service"
)

// RegisterAdminRouter 注册管理员相关路由。
// 包括用户管理、套餐管理、系统配置和系统统计接口。
func RegisterAdminRouter(router *gin.RouterGroup) {
	adminHandler := NewAdminHandler()
	configHandler := NewSystemConfigHandler()

	admin := router.Group("/admin")
	admin.Use(middleware.JWTAuth())
	admin.Use(middleware.AdminAuth())

	// 用户管理
	admin.GET("/users", adminHandler.ListUsers())
	admin.GET("/users/:userId", adminHandler.GetUser())
	admin.PUT("/users/:userId/group", adminHandler.UpdateUserGroup())
	admin.PUT("/users/:userId/status", adminHandler.UpdateUserStatus())

	// 套餐管理
	admin.GET("/groups", adminHandler.ListGroups())
	admin.POST("/groups", adminHandler.CreateGroup())
	admin.PUT("/groups/:groupId", adminHandler.UpdateGroup())
	admin.DELETE("/groups/:groupId", adminHandler.DeleteGroup())

	// 站点管理
	admin.GET("/sites", adminHandler.ListSites())
	admin.DELETE("/sites/:siteId", adminHandler.DeleteSite())
	admin.PUT("/sites/:siteId/verify", adminHandler.VerifySite())

	// 系统配置
	admin.GET("/configs", configHandler.GetConfigs())
	admin.PUT("/configs", configHandler.UpdateConfigs())

	// 系统统计
	admin.GET("/stats", adminHandler.GetStats())
}

// AdminHandler 管理员处理器
type AdminHandler struct {
	userService      *service.UserService
	userGroupService *service.UserGroupService
	siteService      *service.SiteService
}

// NewAdminHandler 创建并返回一个新的 AdminHandler 实例
func NewAdminHandler() *AdminHandler {
	return &AdminHandler{
		userService:      service.GetUserService(),
		userGroupService: service.GetUserGroupService(),
		siteService:      service.GetSiteService(),
	}
}
