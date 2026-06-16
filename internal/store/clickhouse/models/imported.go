package models

import "time"

type ImportedVisitors struct {
	SiteID        uint64    `json:"site_id"`
	Date          time.Time `json:"date"`
	Visitors      uint64    `json:"visitors"`
	Pageviews     uint64    `json:"pageviews"`
	Bounces       uint64    `json:"bounces"`
	Visits        uint64    `json:"visits"`
	VisitDuration uint64    `json:"visit_duration"`
	ImportID      uint64    `json:"import_id"`
}

type ImportedSource struct {
	SiteID        uint64    `json:"site_id"`
	Date          time.Time `json:"date"`
	Source        string    `json:"source"`
	UtmMedium     string    `json:"utm_medium"`
	UtmCampaign   string    `json:"utm_campaign"`
	UtmContent    string    `json:"utm_content"`
	UtmTerm       string    `json:"utm_term"`
	UtmSource     string    `json:"utm_source"`
	Referrer      string    `json:"referrer"`
	Visitors      uint64    `json:"visitors"`
	Visits        uint64    `json:"visits"`
	VisitDuration uint64    `json:"visit_duration"`
	Bounces       uint32    `json:"bounces"`
	Pageviews     uint64    `json:"pageviews"`
	ImportID      uint64    `json:"import_id"`
}

type ImportedPage struct {
	SiteID        uint64    `json:"site_id"`
	Date          time.Time `json:"date"`
	Hostname      string    `json:"hostname"`
	Page          string    `json:"page"`
	Visitors      uint64    `json:"visitors"`
	Pageviews     uint64    `json:"pageviews"`
	Exits         uint64    `json:"exits"`
	TimeOnPage    uint64    `json:"time_on_page"`
	ImportID      uint64    `json:"import_id"`
	Visits        uint64    `json:"visits"`
	ActiveVisitors uint64   `json:"active_visitors"`
}

type ImportedOS struct {
	SiteID                 uint64    `json:"site_id"`
	Date                   time.Time `json:"date"`
	OperatingSystem        string    `json:"operating_system"`
	Visitors               uint64    `json:"visitors"`
	Visits                 uint64    `json:"visits"`
	VisitDuration          uint64    `json:"visit_duration"`
	Bounces                uint32    `json:"bounces"`
	ImportID               uint64    `json:"import_id"`
	Pageviews              uint64    `json:"pageviews"`
	OperatingSystemVersion string    `json:"operating_system_version"`
}

type ImportedLocation struct {
	SiteID        uint64    `json:"site_id"`
	Date          time.Time `json:"date"`
	Country       string    `json:"country"`
	Region        string    `json:"region"`
	City          uint64    `json:"city"`
	Visitors      uint64    `json:"visitors"`
	Visits        uint64    `json:"visits"`
	VisitDuration uint64    `json:"visit_duration"`
	Bounces       uint32    `json:"bounces"`
	ImportID      uint64    `json:"import_id"`
	Pageviews     uint64    `json:"pageviews"`
}

type ImportedExitPage struct {
	SiteID        uint64    `json:"site_id"`
	Date          time.Time `json:"date"`
	ExitPage      string    `json:"exit_page"`
	Visitors      uint64    `json:"visitors"`
	Exits         uint64    `json:"exits"`
	ImportID      uint64    `json:"import_id"`
	Pageviews     uint64    `json:"pageviews"`
	Bounces       uint32    `json:"bounces"`
	VisitDuration uint64    `json:"visit_duration"`
}

type ImportedEntryPage struct {
	SiteID        uint64    `json:"site_id"`
	Date          time.Time `json:"date"`
	EntryPage     string    `json:"entry_page"`
	Visitors      uint64    `json:"visitors"`
	Entrances     uint64    `json:"entrances"`
	VisitDuration uint64    `json:"visit_duration"`
	Bounces       uint32    `json:"bounces"`
	ImportID      uint64    `json:"import_id"`
	Pageviews     uint64    `json:"pageviews"`
}

type ImportedDevice struct {
	SiteID        uint64    `json:"site_id"`
	Date          time.Time `json:"date"`
	Device        string    `json:"device"`
	Visitors      uint64    `json:"visitors"`
	Visits        uint64    `json:"visits"`
	VisitDuration uint64    `json:"visit_duration"`
	Bounces       uint32    `json:"bounces"`
	ImportID      uint64    `json:"import_id"`
	Pageviews     uint64    `json:"pageviews"`
}

type ImportedCustomEvent struct {
	SiteID   uint64    `json:"site_id"`
	ImportID uint64    `json:"import_id"`
	Date     time.Time `json:"date"`
	Name     string    `json:"name"`
	LinkURL  string    `json:"link_url"`
	Path     string    `json:"path"`
	Visitors uint64    `json:"visitors"`
	Events   uint64    `json:"events"`
}

type ImportedBrowser struct {
	SiteID         uint64    `json:"site_id"`
	Date           time.Time `json:"date"`
	Browser        string    `json:"browser"`
	Visitors       uint64    `json:"visitors"`
	Visits         uint64    `json:"visits"`
	VisitDuration  uint64    `json:"visit_duration"`
	Bounces        uint32    `json:"bounces"`
	ImportID       uint64    `json:"import_id"`
	Pageviews      uint64    `json:"pageviews"`
	BrowserVersion string    `json:"browser_version"`
}
