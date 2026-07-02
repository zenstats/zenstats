// Package apikeys HTTP request handler，包括创建、列表和删除。
package apikeys

import (
	"github.com/zenstats/zenstats/internal/service"
)

// APIKeyHandler API Key 管理处理器。
type APIKeyHandler struct {
	service *service.APIKeyService
}

// NewAPIKeyHandler creates and returns a new APIKeyHandler instance.
func NewAPIKeyHandler() *APIKeyHandler {
	return &APIKeyHandler{
		service: service.GetAPIKeyService(),
	}
}
