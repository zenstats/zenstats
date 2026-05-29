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
	Email string `json:"email"`
	Name  string `json:"name"`
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
