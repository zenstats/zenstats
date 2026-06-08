package types

// StatsRequest 统计查询请求参数，支持多种时间周期和过滤条件。
type StatsRequest struct {
	Period          string `form:"period" binding:"required"`
	Date            string `form:"date" binding:"omitempty"`
	From            string `form:"from" binding:"omitempty"`
	To              string `form:"to" binding:"omitempty"`
	Interval        string `form:"interval" binding:"omitempty"`
	Metrics         string `form:"metrics" binding:"omitempty"`
	Limit           int    `form:"limit" binding:"omitempty"`
	Page            int    `form:"page" binding:"omitempty"`
	Filters         string `form:"filters" binding:"omitempty"`
	SampleThreshold int64  `form:"sample_threshold" binding:"omitempty"` // 采样阈值，0 表示不采样
}

// [["is", "visit:country_name", ["Germany", "Poland"]]]
// ["and", [["is", "visit:country_name", ["Germany"]], ["is", "visit:city_name", ["Berlin"]]]]
