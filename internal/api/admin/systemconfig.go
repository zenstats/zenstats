package admin

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/service"
	"github.com/zenstats/zenstats/pkg/response"
)

type SystemConfigHandler struct {
	service *service.SystemConfigService
}

func NewSystemConfigHandler() *SystemConfigHandler {
	return &SystemConfigHandler{
		service: service.GetSystemConfigService(),
	}
}

// GetConfigs 获取所有系统配置
// @Summary      获取系统配置
// @Description  按分组获取所有系统配置项
// @Tags         管理员
// @Produce      json
// @Success      200  {object}  response.SuccessResponse{data=[]service.ConfigGroup}
// @Failure      500  {object}  response.ErrorResponse
// @Router       /api/admin/configs [get]
func (h *SystemConfigHandler) GetConfigs() gin.HandlerFunc {
	return func(c *gin.Context) {
		groups, err := h.service.GetAllConfigs(c.Request.Context())
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}
		response.Success(c, groups)
	}
}

// UpdateConfigsRequest 批量更新配置请求
type UpdateConfigsRequest struct {
	Items []service.ConfigItem `json:"items" binding:"required"`
}

// UpdateConfigs 批量更新系统配置
// @Summary      更新系统配置
// @Description  批量更新系统配置项
// @Tags         管理员
// @Accept       json
// @Produce      json
// @Param        request  body      UpdateConfigsRequest  true  "配置项列表"
// @Success      200      {object}  response.SuccessResponse
// @Failure      400      {object}  response.ErrorResponse
// @Failure      500      {object}  response.ErrorResponse
// @Router       /api/admin/configs [put]
func (h *SystemConfigHandler) UpdateConfigs() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req UpdateConfigsRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		if err := h.service.UpdateConfigs(c.Request.Context(), req.Items); err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		response.Success(c, gin.H{"message": "configs updated"})
	}
}
