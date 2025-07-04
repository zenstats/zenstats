package state

type TopStatsRequest struct {
	Period string `from:"period" binding:"required"`
	Date   string `from:"date" binding:"required"`
}
