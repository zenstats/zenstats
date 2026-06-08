package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/api/types"
	"github.com/zenstats/zenstats/internal/auth"
	"github.com/zenstats/zenstats/pkg/response"
)

// Initialize 初始化系统，创建初始管理员用户并颁发访问令牌。
//
//	@Summary		初始化系统
//	@Description	创建初始管理员用户并完成系统初始化。该接口仅在系统未初始化时可用。
//	@Tags			认证
//	@Accept			json
//	@Produce		json
//	@Param			request	body		types.InitializeRequest					true	"初始化请求参数"
//	@Success		200		{object}	response.SuccessResponse{data=types.InitializeResponse}	"初始化成功返回令牌和用户信息"
//	@Failure		400		{object}	response.ErrorResponse		"请求参数错误"
//	@Failure		500		{object}	response.ErrorResponse		"服务器内部错误"
//	@Router			/auth/init [post]
func (h *AuthHandler) Initialize() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.InitializeRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		user, err := h.service.CreateUser(c.Request.Context(), req.Name, req.Email, req.Password)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}
		// 颁发jwt token
		token, err := auth.GenerateToken(user.ID)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}
		refreshToken, err := auth.GenerateRefreshToken(user.ID)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}
		data := &types.InitializeResponse{
			Token:        token,
			RefreshToken: refreshToken,
			User: &types.User{
				Email: user.Email,
				Name:  user.Name,
			},
		}

		response.Success(c, data)
	}
}
