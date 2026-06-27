package auth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/zenstats/zenstats/internal/api/types"
	authJwt "github.com/zenstats/zenstats/internal/auth"
	"github.com/zenstats/zenstats/pkg/response"
)

// @Summary		刷新访问令牌
// @Description	使用刷新令牌获取新的访问令牌。refreshToken 通过查询参数传递。
// @Tags			认证
// @Accept			json
// @Produce		json
// @Param			refreshToken	query		string					true	"刷新令牌"
// @Success		200				{object}	response.SuccessResponse{data=string}	"成功响应，data 为新的访问令牌"
// @Failure		400				{object}	response.ErrorResponse	"请求参数错误"
// @Failure		401				{object}	response.ErrorResponse	"无效的刷新令牌"
// @Failure		431				{object}	response.ErrorResponse	"刷新令牌已过期"
// @Failure		500				{object}	response.ErrorResponse	"服务器内部错误"
// @Router			/auth/refresh [get]
func (h *AuthHandler) Refresh() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.RefreshRequest
		if err := c.ShouldBindQuery(&req); err != nil {
			response.Error(c, http.StatusBadRequest, errors.New("invalid refresh token"+err.Error()))
			return
		}

		claims, err := authJwt.ParseToken(req.RefreshToken)

		if err != nil {
			if errors.Is(err, jwt.ErrTokenExpired) {
				response.Error(c, 431, errors.New("the refresh token has expired"))
				return
			}
			response.Error(c, http.StatusUnauthorized, errors.New("invalid refresh token"))
			return
		}

		if claims.Subject != "refresh_token" {
			response.Error(c, http.StatusUnauthorized, errors.New("invalid refresh token"))
			return
		}

		var token string
		if claims.UserType == "sub_account" {
			token, err = authJwt.GenerateSubAccountToken(claims.SubAccountID, claims.UserID, claims.Role, claims.Permissions)
		} else {
			token, err = authJwt.GenerateToken(claims.UserID)
		}
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		response.Success(c, token)
	}
}
