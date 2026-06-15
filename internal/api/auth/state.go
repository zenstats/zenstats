package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/service"
	"github.com/zenstats/zenstats/pkg/response"
)

// AuthState 系统认证状态
type AuthState struct {
	Initialized         bool `json:"initialized"`
	RegistrationEnabled bool `json:"registration_enabled"`
}

// @Summary		检查系统初始化状态
// @Description	查询系统是否已初始化及注册是否开启。
// @Tags			认证
// @Accept			json
// @Produce		json
// @Success		200	{object}	response.SuccessResponse{data=AuthState}	"成功响应"
// @Failure		500	{object}	response.ErrorResponse	"服务器内部错误"
// @Router			/auth/state [get]
func (h *AuthHandler) State() gin.HandlerFunc {
	return func(c *gin.Context) {
		count, err := h.service.GetUserCount(c)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		configService := service.GetSystemConfigService()
		registrationEnabled := configService.IsRegistrationEnabled(c.Request.Context())

		response.Success(c, &AuthState{
			Initialized:         count > 0,
			RegistrationEnabled: registrationEnabled,
		})
	}
}
