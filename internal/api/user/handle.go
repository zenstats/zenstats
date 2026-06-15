package user

import (
	"github.com/zenstats/zenstats/internal/service"
)

// UserHandler 用户处理器
type UserHandler struct {
	userService              *service.UserService
	customSearchEngineService *service.CustomSearchEngineService
	userGroupService         *service.UserGroupService
	subAccountService        *service.SubAccountService
	siteService              *service.SiteService
	apiKeyService            *service.APIKeyService
}

// NewUserHandler 创建并返回一个新的 UserHandler 实例
func NewUserHandler() *UserHandler {
	return &UserHandler{
		userService:              service.GetUserService(),
		customSearchEngineService: service.GetCustomSearchEngineService(),
		userGroupService:         service.GetUserGroupService(),
		subAccountService:        service.GetSubAccountService(),
		siteService:              service.GetSiteService(),
		apiKeyService:            service.GetAPIKeyService(),
	}
}
