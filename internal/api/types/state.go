package types

type TopStatsRequest struct {
	Period    string `form:"period" binding:"required"`
	Date      string `form:"date" binding:"omitempty"`
	StartDate string `form:"start_date" binding:"omitempty"`
	EndDate   string `form:"end_date" binding:"omitempty"`
}
