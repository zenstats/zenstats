package apikeys

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/pkg/response"
)

// Delete 删除指定 API Key。
//
//	@Summary		删除 API Key
//	@Description	删除当前用户的指定 API Key
//	@Tags			API Key 管理
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			id	path		int									true	"API Key ID"
//	@Success		200	{object}	response.SuccessResponse{data=nil}	"删除成功"
//	@Failure		400	{object}	response.ErrorResponse				"请求参数错误"
//	@Failure		500	{object}	response.ErrorResponse				"服务器内部错误"
//	@Router			/apikeys/{id} [delete]
func (h *APIKeyHandler) Delete() gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")
		keyID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		userID := c.GetInt64("user_id")
		if err := h.service.DeleteAPIKey(c, userID, keyID); err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		response.Success(c, nil)
	}
}
