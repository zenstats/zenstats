package auth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/api/types"
	"github.com/zenstats/zenstats/internal/service"
	"github.com/zenstats/zenstats/pkg/response"
)

// ForgotPasswordHandler 忘记密码处理器
type ForgotPasswordHandler struct {
	userService    *service.UserService
	emailService   *service.EmailService
}

// NewForgotPasswordHandler 创建 ForgotPasswordHandler 实例
func NewForgotPasswordHandler() *ForgotPasswordHandler {
	return &ForgotPasswordHandler{
		userService:  service.GetUserService(),
		emailService: service.GetEmailService(),
	}
}

// ForgotPasswordRequest 忘记密码请求
type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// ForgotPassword 忘记密码 - 发送重置邮件
//
//	@Summary		忘记密码
//	@Description	输入邮箱发送密码重置链接
//	@Tags			认证
//	@Accept			json
//	@Produce		json
//	@Param			body	body		ForgotPasswordRequest	true	"忘记密码请求"
//	@Success		200		{object}	response.SuccessResponse	"邮件发送成功"
//	@Failure		400		{object}	response.ErrorResponse	"请求参数错误"
//	@Router			/auth/forgot-password [post]
func (h *ForgotPasswordHandler) ForgotPassword() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req ForgotPasswordRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		// 查找用户
		user, err := h.userService.GetUserByEmail(c.Request.Context(), req.Email)
		if err != nil {
			// 无论用户是否存在，都返回成功（防止邮箱枚举）
			response.Success(c, gin.H{"message": "If the email exists, a reset link has been sent"})
			return
		}

		// 生成重置 token 并发送邮件
		baseURL := service.GetBaseURLFromRequest(c)
		err = h.emailService.SendPasswordResetEmail(c.Request.Context(), user.ID, user.Email, baseURL)
		if err != nil {
			// 即使邮件发送失败，也返回成功（防止邮箱枚举）
			response.Success(c, gin.H{"message": "If the email exists, a reset link has been sent"})
			return
		}

		response.Success(c, gin.H{"message": "If the email exists, a reset link has been sent"})
	}
}

// ResetPasswordHandler 重置密码处理器
type ResetPasswordHandler struct {
	userService  *service.UserService
	emailService *service.EmailService
}

// NewResetPasswordHandler 创建 ResetPasswordHandler 实例
func NewResetPasswordHandler() *ResetPasswordHandler {
	return &ResetPasswordHandler{
		userService:  service.GetUserService(),
		emailService: service.GetEmailService(),
	}
}

// ResetPasswordRequest 重置密码请求
type ResetPasswordRequest struct {
	Token       string `json:"token" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

// ResetPassword 重置密码
//
//	@Summary		重置密码
//	@Description	通过重置 token 设置新密码
//	@Tags			认证
//	@Accept			json
//	@Produce		json
//	@Param			body	body		ResetPasswordRequest	true	"重置密码请求"
//	@Success		200		{object}	response.SuccessResponse	"密码重置成功"
//	@Failure		400		{object}	response.ErrorResponse	"请求参数错误或 token 无效"
//	@Router			/auth/reset-password [post]
func (h *ResetPasswordHandler) ResetPassword() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req ResetPasswordRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		// 验证 token
		resetToken, err := h.emailService.VerifyPasswordResetToken(c.Request.Context(), req.Token)
		if err != nil {
			response.Error(c, http.StatusBadRequest, errors.New("invalid or expired reset token"))
			return
		}

		// 更新密码
		err = h.userService.UpdatePassword(c.Request.Context(), resetToken.UserID, req.NewPassword)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		// 标记 token 已使用
		_ = h.emailService.MarkPasswordResetTokenUsed(c.Request.Context(), resetToken.ID)

		response.Success(c, gin.H{"message": "password reset successful"})
	}
}

// ChangePassword 修改密码（已登录用户）
func (h *AuthHandler) ChangePassword() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.ChangePasswordRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		userID := c.GetInt64("user_id")

		// 获取用户
		user, err := h.service.GetUserByID(c.Request.Context(), userID)
		if err != nil {
			response.Error(c, http.StatusUnauthorized, err)
			return
		}

		// 验证旧密码
		if !h.service.CheckPassword(c.Request.Context(), user, req.OldPassword) {
			response.ErrorWithKey(c, http.StatusUnauthorized, "auth.invalid_password")
			return
		}

		// 更新密码
		err = h.service.UpdatePassword(c.Request.Context(), userID, req.NewPassword)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		response.Success(c, gin.H{"message": "password changed successfully"})
	}
}
