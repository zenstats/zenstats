// Package apikeys 处理 API Key 管理相关的 HTTP 请求，包括创建、列表和删除。
package apikeys

import (
	"github.com/zenstats/zenstats/internal/service"
)

// APIKeyHandler API Key 管理处理器。
type APIKeyHandler struct {
	service *service.APIKeyService
}

// NewAPIKeyHandler 创建并返回一个新的 APIKeyHandler 实例。
func NewAPIKeyHandler() *APIKeyHandler {
	return &APIKeyHandler{
		service: service.GetAPIKeyService(),
	}
}
