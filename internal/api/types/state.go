package types

type TopStatsRequest struct {
	Period   string `form:"period" binding:"required"`
	Date     string `form:"date" binding:"omitempty"`
	From     string `form:"from" binding:"omitempty"`
	To       string `form:"to" binding:"omitempty"`
	Interval string `form:"interval" binding:"omitempty"`
}
