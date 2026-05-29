package apikeys

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/pkg/response"
)

// List 获取当前用户的 API Key 列表。
//
//	@Summary		获取 API Key 列表
//	@Description	获取当前用户的所有 API Key（不包含明文 key）
//	@Tags			API Key 管理
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	response.SuccessResponse{data=[]service.APIKeyInfo}	"成功响应"
//	@Failure		500	{object}	response.ErrorResponse								"服务器内部错误"
//	@Router			/apikeys [get]
func (h *APIKeyHandler) List() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetInt64("user_id")

		keys, err := h.service.ListAPIKeys(c, userID)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		response.Success(c, keys)
	}
}
