package types

type StatsRequest struct {
	Period    string `form:"period" binding:"required"`
	Date      string `form:"date" binding:"omitempty"`
	From      string `form:"from" binding:"omitempty"`
	To        string `form:"to" binding:"omitempty"`
	Interval  string `form:"interval" binding:"omitempty"`
	Limit     int    `form:"limit" binding:"omitempty"`
	Page      int    `form:"page" binding:"omitempty"`
	MetaKey   string `form:"meta_key" binding:"omitempty"`
	MetaValue string `form:"meta_value" binding:"omitempty"`
}
