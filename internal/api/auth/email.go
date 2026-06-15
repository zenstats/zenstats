package auth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/api/types"
	"github.com/zenstats/zenstats/internal/service"
	"github.com/zenstats/zenstats/pkg/response"
)

type EmailHandler struct {
	service *service.EmailService
}

func NewEmailHandler() *EmailHandler {
	return &EmailHandler{
		service: service.GetEmailService(),
	}
}

// SendVerification 发送验证邮件
// @Summary      发送验证邮件
// @Description  向当前用户发送邮箱验证邮件
// @Tags         认证
// @Produce      json
// @Success      200  {object}  response.SuccessResponse  "发送成功"
// @Failure      401  {object}  response.ErrorResponse    "未授权"
// @Failure      500  {object}  response.ErrorResponse    "服务器错误"
// @Router       /auth/send-verification [post]
func (h *EmailHandler) SendVerification() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			response.Error(c, http.StatusUnauthorized, errors.New("unauthorized"))
			return
		}

		uid, ok := userID.(int64)
		if !ok {
			response.Error(c, http.StatusInternalServerError, errors.New("invalid user id"))
			return
		}

		// 检查是否是管理员（管理员不需要验证）
		isAdmin, err := h.service.IsAdmin(c.Request.Context(), uid)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}
		if isAdmin {
			response.Error(c, http.StatusBadRequest, errors.New("admin does not need email verification"))
			return
		}

		// 检查是否已验证
		isVerified, err := h.service.IsEmailVerified(c.Request.Context(), uid)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}
		if isVerified {
			response.Error(c, http.StatusBadRequest, errors.New("email already verified"))
			return
		}

		// 发送验证邮件
		baseURL := service.GetBaseURLFromRequest(c)
		if err := h.service.SendVerificationEmail(c.Request.Context(), uid, "", baseURL); err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		response.Success(c, gin.H{"message": "verification email sent"})
	}
}

// VerifyEmail 验证邮箱
// @Summary      验证邮箱
// @Description  通过验证链接中的 token 验证邮箱
// @Tags         认证
// @Produce      json
// @Param        token  query     string  true  "验证 token"
// @Success      200    {object}  response.SuccessResponse  "验证成功"
// @Failure      400    {object}  response.ErrorResponse    "token 无效或过期"
// @Router       /auth/verify-email [get]
func (h *EmailHandler) VerifyEmail() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Query("token")
		if token == "" {
			response.Error(c, http.StatusBadRequest, errors.New("token is required"))
			return
		}

		if err := h.service.VerifyEmail(c.Request.Context(), token); err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		// 重定向到前端验证成功页面
		c.Redirect(http.StatusMovedPermanently, "/verify-email-success")
	}
}

// GetVerificationStatus 获取验证状态
// @Summary      获取邮箱验证状态
// @Description  获取当前用户的邮箱验证状态
// @Tags         认证
// @Produce      json
// @Success      200  {object}  response.SuccessResponse{data=types.VerificationStatus}  "验证状态"
// @Failure      401  {object}  response.ErrorResponse  "未授权"
// @Router       /auth/verification-status [get]
func (h *EmailHandler) GetVerificationStatus() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			response.Error(c, http.StatusUnauthorized, errors.New("unauthorized"))
			return
		}

		uid, ok := userID.(int64)
		if !ok {
			response.Error(c, http.StatusInternalServerError, errors.New("invalid user id"))
			return
		}

		// 检查是否是管理员
		isAdmin, err := h.service.IsAdmin(c.Request.Context(), uid)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		// 检查是否已验证
		isVerified, err := h.service.IsEmailVerified(c.Request.Context(), uid)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		// 管理员默认已验证
		if isAdmin {
			isVerified = true
		}

		response.Success(c, &types.VerificationStatus{
			EmailVerified: isVerified,
			IsAdmin:       isAdmin,
		})
	}
}
