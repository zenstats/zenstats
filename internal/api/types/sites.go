package types

import "time"

type CreateSiteRequest struct {
	Domain                      string `json:"domain" binding:"required" maxLength:"255"`
	Timezone                    string `json:"timezone" maxLength:"255"`
	Remark                      string `json:"remark" maxLength:"255"`
	IngestRateLimitScaleSeconds int    `json:"rate_seconds" binding:"omitempty,min=1,max=3600"`
	IngestLimitPerMinute        int    `json:"limit_minute" binding:"omitempty,min=1,max=10000000"`
}

type UpdateSiteRequest struct {
	Timezone                    *string   `json:"timezone" binding:"omitempty,timezone"`
	Public                      *bool     `json:"public" binding:"omitempty,boolean"`
	Remark                      string    `json:"remark" binding:"omitempty,max=255"`
	StatsStartDate              time.Time `json:"stats_start_date" binding:"omitempty,datetime"`
	IngestRateLimitScaleSeconds int       `json:"rate_seconds" binding:"omitempty,min=1,max=3600"`
	IngestLimitPerMinute        int       `json:"limit_minute" binding:"omitempty,min=1,max=10000000"`
}

type SiteWithRemark struct {
	ID                          int64  `json:"id"`
	Domain                      string `json:"domain"`
	Timezone                    string `json:"timezone"`
	Remark                      string `json:"remark"`
	IngestRateLimitScaleSeconds int    `json:"rate_seconds"`
	IngetLimitPerMinute         int    `json:"limit_minute"`
}

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

type AddShieldRuleHostnameRequest struct {
	Hostname        string `json:"hostname" binding:"required"`
	HostnamePattern string `json:"hostname_pattern" binding:"required"`
	Action          string `json:"action" binding:"required,oneof=allow deny"`
}

type AddShieldRuleCountryRequest struct {
	CountryCode string `json:"country_code" binding:"required"`
	Action      string `json:"action" binding:"required,oneof=allow deny"`
}

type AddShieldRuleIPRequest struct {
	IP          string `json:"ip" binding:"required"`
	Action      string `json:"action" binding:"required,oneof=deny allow"`
	Description string `json:"description"`
}
