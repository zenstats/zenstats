package types

import (
	"time"
)

// CreateSiteRequest 创建站点请求参数。
type CreateSiteRequest struct {
	Domain                      string `json:"domain" binding:"required" maxLength:"255" example:"example.com"`
	Timezone                    string `json:"timezone" maxLength:"255" example:"Asia/Shanghai"`
	Remark                      string `json:"remark" maxLength:"255" example:"官网"`
	IngestRateLimitScaleSeconds int    `json:"rate_seconds" binding:"omitempty,min=1,max=3600" example:"60"`
	IngestLimitPerMinute        int    `json:"limit_minute" binding:"omitempty,min=1,max=10000000" example:"600"`
}

// UpdateSiteRequest 更新站点请求参数，支持部分更新（可选字段使用指针类型）。
type UpdateSiteRequest struct {
	Timezone                    *string   `json:"timezone" binding:"omitempty,timezone"`
	Public                      *bool     `json:"public" binding:"omitempty,boolean"`
	Remark                      string    `json:"remark" binding:"omitempty,max=255"`
	StatsStartDate              time.Time `json:"stats_start_date" binding:"omitempty,datetime"`
	IngestRateLimitScaleSeconds *int      `json:"rate_seconds" binding:"omitempty,min=1,max=3600"`
	IngestLimitPerMinute        *int      `json:"limit_minute" binding:"omitempty,min=1,max=10000000"`
}

// SiteWithRemark 包含备注信息的站点响应结构。
type SiteWithRemark struct {
	ID                          int64  `json:"id"`
	Domain                      string `json:"domain"`
	Timezone                    string `json:"timezone"`
	Remark                      string `json:"remark"`
	IngestRateLimitScaleSeconds int    `json:"rate_seconds"`
	IngetLimitPerMinute         int    `json:"limit_minute"`
}

// SiteResponse 站点详细信息响应结构。
type SiteResponse struct {
	ID                          int64     `json:"id"`
	Domain                      string    `json:"domain"`
	Timezone                    string    `json:"timezone"`
	Public                      bool      `json:"public"`
	StatsStartDate              time.Time `json:"stats_start_date"`
	IngestRateLimitScaleSeconds int       `json:"rate_seconds"`
	IngestLimitPerMinute        int       `json:"limit_minute"`
	CreatedAt                   time.Time `json:"created_at"`
	UpdatedAt                   time.Time `json:"updated_at"`
}

// AddShieldRuleHostnameRequest 添加 Hostname 屏蔽规则请求参数。
type AddShieldRuleHostnameRequest struct {
	Hostname        string `json:"hostname" binding:"required"`
	HostnamePattern string `json:"hostname_pattern" binding:"required"`
	Action          string `json:"action" binding:"required,oneof=allow deny"`
}

// AddShieldRuleCountryRequest 添加国家屏蔽规则请求参数。
type AddShieldRuleCountryRequest struct {
	CountryCode string `json:"country_code" binding:"required" example:"CN"`
	Action      string `json:"action" binding:"required,oneof=allow deny" example:"deny"`
}

// AddShieldRuleIPRequest 添加 IP 屏蔽规则请求参数。
type AddShieldRuleIPRequest struct {
	IP          string `json:"ip" binding:"required" example:"203.0.113.10"`
	Action      string `json:"action" binding:"required,oneof=deny allow" example:"deny"`
	Description string `json:"description" example:"测试流量"`
}

// ShieldRuleIPResponse IP 屏蔽规则响应结构。
type ShieldRuleIPResponse struct {
	ID          int64     `json:"id"`
	SiteID      int64     `json:"site_id"`
	IP          string    `json:"ip"`
	Action      string    `json:"action"`
	Description string    `json:"description"`
	AddedBy     string    `json:"added_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
