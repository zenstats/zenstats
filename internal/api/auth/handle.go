// Package auth HTTP request handler，包括登录、令牌刷新、系统初始化等。
package auth

import (
	"github.com/zenstats/zenstats/internal/service"
)

// AuthHandler 认证处理器，封装用户服务以处理认证相关请求。
type AuthHandler struct {
	service           *service.UserService
	subAccountService *service.SubAccountService
}

// NewAuthHandler creates and returns a new AuthHandler instance.
func NewAuthHandler() *AuthHandler {
	userService := service.GetUserService()
	subAccountService := service.GetSubAccountService()
	return &AuthHandler{
		service:           userService,
		subAccountService: subAccountService,
	}
}
