package imports

type UploadResponse struct {
	ImportID     uint64 `json:"import_id"`
	RowsImported int    `json:"rows_imported"`
	ReportType   string `json:"report_type"`
	Table        string `json:"table"`
}

type AggregateResponse struct {
	Visitors      uint64  `json:"visitors"`
	Pageviews     uint64  `json:"pageviews"`
	Visits        uint64  `json:"visits"`
	BounceRate    float64 `json:"bounce_rate"`
	VisitDuration uint64  `json:"visit_duration"`
	ViewsPerVisit float64 `json:"views_per_visit"`
}

type BreakdownRow struct {
	Name     string `json:"name"`
	Visitors uint64 `json:"visitors"`
	Pageviews uint64 `json:"pageviews,omitempty"`
}

type BreakdownResponse struct {
	Data []BreakdownRow `json:"data"`
}

type TimeSeriesPoint struct {
	Date      string `json:"date"`
	Visitors  uint64 `json:"visitors"`
	Pageviews uint64 `json:"pageviews"`
}
