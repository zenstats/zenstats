package user

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/api/types"
	"github.com/zenstats/zenstats/pkg/response"
)

// @Summary		更新用户资料
// @Description	更新当前用户的显示名称。
// @Tags			用户
// @Accept			json
// @Produce		json
// @Param			body	body		types.UpdateProfileRequest	true	"更新资料参数"
// @Success		200		{object}	response.SuccessResponse	"更新成功"
// @Failure		400		{object}	response.ErrorResponse	"请求参数错误"
// @Router			/user/profile [put]
func (h *UserHandler) UpdateProfile() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.UpdateProfileRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		userID := c.GetInt64("user_id")

		err := h.userService.UpdateUserName(c.Request.Context(), userID, req.Name)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		response.Success(c, gin.H{"message": "profile updated successfully"})
	}
}
