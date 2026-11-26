package router

import (
	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/api/auth"
	"github.com/zenstats/zenstats/internal/middleware"
)

// RegisterAuthRouter 注册认证相关路由。
// 包括用户登录、用户注册、令牌刷新、系统状态检查和系统初始化接口。
func RegisterAuthRouter(router *gin.RouterGroup) {
	authHandle := auth.NewAuthHandler()
	emailHandle := auth.NewEmailHandler()
	forgotPasswordHandle := auth.NewForgotPasswordHandler()
	resetPasswordHandle := auth.NewResetPasswordHandler()

	auth := router.Group("/auth")
	limit := middleware.AuthRateLimit()

	// 系统初始化（仅需 CheckInitialization 中件，不需要 JWTAuth）
	initGroup := auth.Group("")
	initGroup.Use(middleware.CheckInitialization())
	initGroup.POST("/init", authHandle.Initialize())

	// 无需登录的路由
	auth.POST("/login", limit, authHandle.Login())
	auth.POST("/sub-login", limit, authHandle.SubLogin())
	auth.POST("/register", limit, authHandle.Register())
	auth.POST("/refresh", authHandle.Refresh())
	auth.GET("/state", authHandle.State())
	auth.GET("/verify-email", emailHandle.VerifyEmail())

	// 忘记密码 / 重置密码（无需登录）
	auth.POST("/forgot-password", limit, forgotPasswordHandle.ForgotPassword())
	auth.POST("/reset-password", limit, resetPasswordHandle.ResetPassword())

	// 需要登录的路由
	auth.Use(middleware.JWTAuth())
	auth.POST("/send-verification", emailHandle.SendVerification())
	auth.GET("/verification-status", emailHandle.GetVerificationStatus())
	auth.Use(middleware.SubAccountReadOnly()).POST("/change-password", authHandle.ChangePassword())
}
