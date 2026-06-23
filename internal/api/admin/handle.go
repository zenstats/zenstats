package admin

import (
	"github.com/zenstats/zenstats/internal/service"
)

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
