package sites

import "time"

type CreateSiteRequest struct {
	Domain                      string `json:"domain" binding:"required" maxLength:"255"`
	Timezone                    string `json:"timezone" maxLength:"255"`
	Remark                      string `json:"remark" maxLength:"255"`
	IngestRateLimitScaleSeconds int    `json:"rate_seconds"`
	IngestLimitPerMinute        int    `json:"limit_minute"`
}

type UpdateSiteRequest struct {
	Domain                      *string    `json:"domain"`
	Timezone                    *string    `json:"timezone"`
	Public                      *bool      `json:"public"`
	StatsStartDate              *time.Time `json:"stats_start_date"`
	IngestRateLimitScaleSeconds *int       `json:"ingest_rate_limit_scale_seconds"`
	IngestLimitPerMinute        *int       `json:"ingest_limit_per_minute"`
}

type SiteResponse struct {
	ID                          int64     `json:"id"`
	Domain                      string    `json:"domain"`
	Timezone                    string    `json:"timezone"`
	Public                      bool      `json:"public"`
	StatsStartDate              time.Time `json:"stats_start_date"`
	IngestRateLimitScaleSeconds int       `json:"ingest_rate_limit_scale_seconds"`
	IngestLimitPerMinute        int       `json:"ingest_limit_per_minute"`
	CreatedAt                   time.Time `json:"created_at"`
	UpdatedAt                   time.Time `json:"updated_at"`
}
