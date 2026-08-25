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
// @Description	使用刷新令牌获取新的访问令牌。刷新令牌通过 JSON body 传递（不接受 query 参数，避免落入访问日志）。
// @Tags			认证
// @Accept			json
// @Produce		json
// @Param			request	body		types.RefreshRequest	true	"刷新令牌"
// @Success		200				{object}	response.SuccessResponse{data=string}	"成功响应，data 为新的访问令牌"
// @Failure		400				{object}	response.ErrorResponse	"请求参数错误"
// @Failure		401				{object}	response.ErrorResponse	"无效的刷新令牌"
// @Failure		431				{object}	response.ErrorResponse	"刷新令牌已过期"
// @Failure		500				{object}	response.ErrorResponse	"服务器内部错误"
// @Router			/auth/refresh [post]
func (h *AuthHandler) Refresh() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.RefreshRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, errors.New("invalid refresh token: "+err.Error()))
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

		if claims.Subject != authJwt.SubjectRefreshToken {
			response.Error(c, http.StatusUnauthorized, errors.New("invalid refresh token"))
			return
		}

		var token string
		if claims.UserType == "sub_account" {
			// 刷新时重新校验子账号状态并取最新权限，
			// 防止已停用/删除或权限被撤销的子账号在刷新有效期内继续获得有效令牌。
			subAccount, saErr := h.subAccountService.GetSubAccountByID(c.Request.Context(), claims.SubAccountID)
			if saErr != nil || subAccount == nil {
				response.Error(c, http.StatusUnauthorized, errors.New("invalid refresh token"))
				return
			}
			if subAccount.Status != "active" {
				response.ErrorWithKey(c, http.StatusForbidden, "auth.sub_account_suspended")
				return
			}
			token, err = authJwt.GenerateSubAccountToken(subAccount.ID, subAccount.ParentUserID, subAccount.Role, subAccount.Permissions)
		} else {
			// 刷新时同样校验父账号状态，防止已停用账号靠刷新令牌续命
			if h.service.GetUserStatus(c.Request.Context(), claims.UserID) == "suspended" {
				response.Error(c, http.StatusForbidden, errors.New("account suspended"))
				return
			}
			token, err = authJwt.GenerateToken(claims.UserID)
		}
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		response.Success(c, token)
	}
}
