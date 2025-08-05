package types

type StatsRequest struct {
	Period   string `form:"period" binding:"required"`
	Date     string `form:"date" binding:"omitempty"`
	From     string `form:"from" binding:"omitempty"`
	To       string `form:"to" binding:"omitempty"`
	Interval string `form:"interval" binding:"omitempty"`
	Limit    int    `form:"limit" binding:"omitempty"`
	Page     int    `form:"page" binding:"omitempty"`
	Filters  []any  `json:"filters" binding:"omitempty"`
}

// [["is", "visit:country_name", ["Germany", "Poland"]]]
// ["and", [["is", "visit:country_name", ["Germany"]], ["is", "visit:city_name", ["Berlin"]]]]
