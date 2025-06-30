package auth

import (
	"github.com/zenstats/zenstats/internal/service"
)

type AuthHandler struct {
	service *service.UserService
}

func NewAuthHandler() *AuthHandler {
	service := service.GetUserService()
	return &AuthHandler{service: service}
}
