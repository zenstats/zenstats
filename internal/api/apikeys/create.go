package apikeys

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/pkg/response"
)

// CreateAPIKeyRequest 创建 API Key 请求参数。
type CreateAPIKeyRequest struct {
	Name      string `json:"name" binding:"required,max=255"`
	ExpiresAt string `json:"expires_at,omitempty"` // 过期时间，格式：2006-01-02 15:04:05，为空则永不过期
}

// CreateAPIKeyResponse 创建 API Key 响应。
type CreateAPIKeyResponse struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Key       string `json:"key"`
	CreatedAt string `json:"created_at"`
	ExpiresAt string `json:"expires_at,omitempty"`
}

// Create 创建新的 API Key。
//
//	@Summary		创建 API Key
//	@Description	为当前用户创建一个新的 API Key，明文 key 仅此一次返回
//	@Tags			API Key 管理
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		CreateAPIKeyRequest									true	"API Key 参数"
//	@Success		200		{object}	response.SuccessResponse{data=CreateAPIKeyResponse}	"创建成功"
//	@Failure		400		{object}	response.ErrorResponse								"请求参数错误"
//	@Failure		500		{object}	response.ErrorResponse								"服务器内部错误"
//	@Router			/apikeys [post]
func (h *APIKeyHandler) Create() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req CreateAPIKeyRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		userID := c.GetInt64("user_id")

		var expiresAt *time.Time
		if req.ExpiresAt != "" {
			t, err := time.Parse("2006-01-02 15:04:05", req.ExpiresAt)
			if err != nil {
				// 也尝试只传日期
				t, err = time.Parse("2006-01-02", req.ExpiresAt)
			}
			if err != nil {
				response.Error(c, http.StatusBadRequest, fmt.Errorf("invalid expires_at format, use '2006-01-02' or '2006-01-02 15:04:05'"))
				return
			}
			expiresAt = &t
		}

		info, rawKey, err := h.service.CreateAPIKey(c, userID, req.Name, expiresAt)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		resp := &CreateAPIKeyResponse{
			ID:        info.ID,
			Name:      info.Name,
			Key:       rawKey,
			CreatedAt: info.CreatedAt,
		}
		if info.ExpiresAt != "" {
			resp.ExpiresAt = info.ExpiresAt
		}

		response.Success(c, resp)
	}
}
