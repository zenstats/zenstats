// Package auth 处理用户认证相关的 HTTP 请求，包括登录、令牌刷新、系统初始化等。
package auth

import (
	"github.com/zenstats/zenstats/internal/service"
)

// AuthHandler 认证处理器，封装用户服务以处理认证相关请求。
type AuthHandler struct {
	service *service.UserService
}

// NewAuthHandler 创建并返回一个新的 AuthHandler 实例。
func NewAuthHandler() *AuthHandler {
	service := service.GetUserService()
	return &AuthHandler{service: service}
}
