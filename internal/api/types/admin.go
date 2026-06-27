package types

import "time"

// AdminUser 管理员用户信息
type AdminUser struct {
	ID        int64     `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	IsAdmin   bool      `json:"is_admin"`
	Status    string    `json:"status"`
	GroupID   int64     `json:"group_id"`
	GroupName string    `json:"group_name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AdminUserDetail 管理员用户详情
type AdminUserDetail struct {
	ID                  int64     `json:"id"`
	Email               string    `json:"email"`
	Name                string    `json:"name"`
	IsAdmin             bool      `json:"is_admin"`
	Status              string    `json:"status"`
	GroupID             int64     `json:"group_id"`
	GroupName           string    `json:"group_name"`
	MaxSites            int       `json:"max_sites"`
	MaxMonthlyEvents    int       `json:"max_monthly_events"`
	MaxAPIKeys          int       `json:"max_api_keys"`
	MaxSubAccounts      int       `json:"max_sub_accounts"`
	CustomSearchEngines bool      `json:"custom_search_engines"`
	SiteCount           int       `json:"site_count"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// AdminUserListResponse 管理员用户列表响应
type AdminUserListResponse struct {
	Users      []*AdminUser `json:"users"`
	TotalCount int          `json:"total_count"`
	Page       int          `json:"page"`
	PageSize   int          `json:"page_size"`
}

// UpdateUserGroupRequest 更新用户套餐请求
type UpdateUserGroupRequest struct {
	GroupID int64 `json:"group_id" binding:"required"`
}

// UpdateUserStatusRequest 更新用户状态请求
type UpdateUserStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=active suspended expired"`
}

// AdminGroup 管理员套餐信息
type AdminGroup struct {
	ID                  int64     `json:"id"`
	Name                string    `json:"name"`
	Description         string    `json:"description"`
	MaxSites            int       `json:"max_sites"`
	MaxMonthlyEvents    int       `json:"max_monthly_events"`
	MaxAPIKeys          int       `json:"max_api_keys"`
	MaxSubAccounts      int       `json:"max_sub_accounts"`
	CustomSearchEngines bool      `json:"custom_search_engines"`
	IsDefault           bool      `json:"is_default"`
	Price               float64   `json:"price"`
	UserCount           int       `json:"user_count"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// AdminGroupListResponse 管理员套餐列表响应
type AdminGroupListResponse struct {
	Groups []*AdminGroup `json:"groups"`
}

// CreateGroupRequest 创建套餐请求
type CreateGroupRequest struct {
	Name                string  `json:"name" binding:"required,min=1,max=100"`
	Description         string  `json:"description"`
	MaxSites            int     `json:"max_sites" binding:"omitempty,gte=-1"`
	MaxMonthlyEvents    int     `json:"max_monthly_events" binding:"omitempty,gte=-1"`
	MaxAPIKeys          int     `json:"max_api_keys" binding:"omitempty,gte=-1"`
	MaxSubAccounts      int     `json:"max_sub_accounts" binding:"omitempty,gte=-1"`
	CustomSearchEngines bool    `json:"custom_search_engines"`
	IsDefault           bool    `json:"is_default"`
	Price               float64 `json:"price"`
}

// UpdateGroupRequest 更新套餐请求
type UpdateGroupRequest struct {
	Name                string  `json:"name" binding:"required,min=1,max=100"`
	Description         string  `json:"description"`
	MaxSites            int     `json:"max_sites" binding:"omitempty,gte=-1"`
	MaxMonthlyEvents    int     `json:"max_monthly_events" binding:"omitempty,gte=-1"`
	MaxAPIKeys          int     `json:"max_api_keys" binding:"omitempty,gte=-1"`
	MaxSubAccounts      int     `json:"max_sub_accounts" binding:"omitempty,gte=-1"`
	CustomSearchEngines bool    `json:"custom_search_engines"`
	IsDefault           bool    `json:"is_default"`
	Price               float64 `json:"price"`
}

// SystemStats 系统统计数据
type SystemStats struct {
	UserCount  int          `json:"user_count"`
	GroupStats []*GroupStat `json:"group_stats"`
}

// GroupStat 套餐统计
type GroupStat struct {
	GroupID   int64  `json:"group_id"`
	GroupName string `json:"group_name"`
	UserCount int    `json:"user_count"`
}

// CustomSearchEngine 自定义搜索引擎信息
type CustomSearchEngine struct {
	ID        int64     `json:"id"`
	Domain    string    `json:"domain"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CustomSearchEngineListResponse 自定义搜索引擎列表响应
type CustomSearchEngineListResponse struct {
	Engines       []*CustomSearchEngine `json:"engines"`
	HasPermission bool                  `json:"has_permission"`
}

// CreateSearchEngineRequest 创建自定义搜索引擎请求
type CreateSearchEngineRequest struct {
	Domain string `json:"domain" binding:"required,min=1,max=255"`
	Name   string `json:"name" binding:"required,min=1,max=100"`
}

// UpdateSearchEngineRequest 更新自定义搜索引擎请求
type UpdateSearchEngineRequest struct {
	Domain string `json:"domain" binding:"required,min=1,max=255"`
	Name   string `json:"name" binding:"required,min=1,max=100"`
}

// SubAccount 子账号信息
type SubAccount struct {
	ID          int64      `json:"id"`
	Email       string     `json:"email"`
	Name        string     `json:"name"`
	Role        string     `json:"role"`
	Permissions []string   `json:"permissions"`
	Status      string     `json:"status"`
	LastSeen    *time.Time `json:"last_seen"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// SubAccountListResponse 子账号列表响应
type SubAccountListResponse struct {
	SubAccounts    []*SubAccount `json:"sub_accounts"`
	HasPermission  bool          `json:"has_permission"`
	MaxSubAccounts int           `json:"max_sub_accounts"`
	CurrentCount   int           `json:"current_count"`
}

// CreateSubAccountRequest 创建子账号请求
type CreateSubAccountRequest struct {
	Email       string   `json:"email" binding:"required,email"`
	Name        string   `json:"name" binding:"required,min=1,max=100"`
	Password    string   `json:"password" binding:"required,min=8,max=128"`
	Role        string   `json:"role" binding:"omitempty,oneof=viewer editor admin custom"`
	Permissions []string `json:"permissions" binding:"omitempty"`
}

// UpdateSubAccountRequest 更新子账号请求
type UpdateSubAccountRequest struct {
	Name        string   `json:"name" binding:"required,min=1,max=100"`
	Status      string   `json:"status" binding:"required,oneof=active suspended"`
	Role        string   `json:"role" binding:"omitempty,oneof=viewer editor admin custom"`
	Permissions []string `json:"permissions" binding:"omitempty"`
}

// ResetSubAccountPasswordRequest 重置子账号密码请求
type ResetSubAccountPasswordRequest struct {
	NewPassword string `json:"new_password" binding:"required,min=8,max=128"`
}

// UserQuotaInfo 用户额度信息
type UserQuotaInfo struct {
	GroupName           string `json:"group_name"`
	MaxSites            int    `json:"max_sites"`
	MaxMonthlyEvents    int    `json:"max_monthly_events"`
	MaxAPIKeys          int    `json:"max_api_keys"`
	MaxSubAccounts      int    `json:"max_sub_accounts"`
	CustomSearchEngines bool   `json:"custom_search_engines"`
	CurrentSites        int    `json:"current_sites"`
	CurrentMonthlyEvents int64 `json:"current_monthly_events"`
	CurrentAPIKeys      int    `json:"current_api_keys"`
	CurrentSubAccounts  int    `json:"current_sub_accounts"`
}

// AdminSite 管理员站点信息
type AdminSite struct {
	ID         int64      `json:"id"`
	Domain     string     `json:"domain"`
	Remark     string     `json:"remark"`
	Timezone   string     `json:"timezone"`
	OwnerName  string     `json:"owner_name"`
	IsVerified bool       `json:"is_verified"`
	VerifiedAt *time.Time `json:"verified_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// AdminSiteListResponse 管理员站点列表响应
type AdminSiteListResponse struct {
	Sites      []*AdminSite `json:"sites"`
	TotalCount int          `json:"total_count"`
	Page       int          `json:"page"`
	PageSize   int          `json:"page_size"`
}
