// Package types 定义 API 层的请求和响应数据结构。
package types

// InitializeRequest 系统初始化请求参数。
type InitializeRequest struct {
	Email    string `json:"email" binding:"required"`
	Name     string `json:"name" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// User 用户基本信息。
type User struct {
	ID            int64  `json:"id"`
	Email         string `json:"email"`
	Name          string `json:"name"`
	IsAdmin       bool   `json:"is_admin"`
	EmailVerified bool   `json:"email_verified"`
}

// InitializeResponse 系统初始化成功响应，包含访问令牌和用户信息。
type InitializeResponse struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
	User         *User  `json:"user"`
}

// LoginRequest 用户登录请求参数。
type LoginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse 用户登录成功响应，包含访问令牌和用户信息。
type LoginResponse struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
	User         *User  `json:"user"`
}

// RefreshRequest 刷新访问令牌请求参数。
type RefreshRequest struct {
	RefreshToken string `form:"refreshToken" binding:"required"`
}

// RegisterRequest 用户注册请求参数。
type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Name     string `json:"name" binding:"required,min=2,max=100"`
	Password string `json:"password" binding:"required,min=8,max=128"`
}

// RegisterResponse 用户注册成功响应，包含访问令牌和用户信息。
type RegisterResponse struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
	User         *User  `json:"user"`
}

// VerificationStatus 邮箱验证状态
type VerificationStatus struct {
	EmailVerified bool `json:"email_verified"`
	IsAdmin       bool `json:"is_admin"`
}

// SubAccountLoginRequest 子账号登录请求参数
type SubAccountLoginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// SubAccountLoginResponse 子账号登录成功响应
type SubAccountLoginResponse struct {
	Token        string          `json:"token"`
	RefreshToken string          `json:"refresh_token"`
	User         *SubAccountUser `json:"user"`
}

// SubAccountUser 子账号用户信息
type SubAccountUser struct {
	ID           int64  `json:"id"`
	Email        string `json:"email"`
	Name         string `json:"name"`
	Role         string `json:"role"`
	ParentUserID int64  `json:"parent_user_id"`
}

// UpdateProfileRequest 更新用户资料请求参数
type UpdateProfileRequest struct {
	Name string `json:"name" binding:"required,min=2,max=100"`
}

// ChangePasswordRequest 修改密码请求参数
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8,max=128"`
}
