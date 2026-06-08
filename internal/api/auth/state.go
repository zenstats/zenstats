package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/pkg/response"
)

// @Summary		检查系统初始化状态
// @Description	查询系统是否已初始化。响应 data 返回 "initialized" 表示已初始化，"not_initialized" 表示未初始化。
// @Tags			认证
// @Accept			json
// @Produce		json
// @Success		200	{object}	response.SuccessResponse{data=string}	"成功响应，data 为系统状态"
// @Failure		500	{object}	response.ErrorResponse	"服务器内部错误"
// @Router			/auth/state [get]
func (h *AuthHandler) State() gin.HandlerFunc {
	return func(c *gin.Context) {
		count, err := h.service.GetUserCount(c)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}
		if count == 0 {
			response.Success(c, "not_initialized")
			return
		}
		response.Success(c, "initialized")
	}
}
