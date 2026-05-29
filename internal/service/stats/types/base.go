package types

import (
	"time"
)

// TimeRange 定义统计时间范围
type TimeRange struct {
	Start time.Time
	End   time.Time
}

// NewTimeRange 创建时间范围
func NewTimeRange(start, end time.Time) TimeRange {
	return TimeRange{
		Start: start,
		End:   end,
	}
}

// VisitProps 访问属性，包含来源、设备、浏览器、地理位置等所有可过滤的访问维度。
type VisitProps struct {
	Source            string `json:"source"`
	Channel           string `json:"channel"`
	Referrer          string `json:"referrer"`
	UTMMedium         string `json:"utm_medium"`
	UTMSource         string `json:"utm_source"`
	UTMCampaign       string `json:"utm_campaign"`
	UTMContent        string `json:"utm_content"`
	UTMTerm           string `json:"utm_term"`
	Screen            string `json:"screen"`
	Device            string `json:"device"`
	Browser           string `json:"browser"`
	BrowserVersion    string `json:"browser_version"`
	OS                string `json:"os"`
	OSVersion         string `json:"os_version"`
	Country           string `json:"country"`
	Region            string `json:"region"`
	City              string `json:"city"`
	CountryName       string `json:"country_name"`
	RegionName        string `json:"region_name"`
	CityName          string `json:"city_name"`
	EntryPage         string `json:"entry_page"`
	ExitPage          string `json:"exit_page"`
	EntryPageHostname string `json:"entry_page_hostname"`
	ExitPageHostname  string `json:"exit_page_hostname"`
}

// EventProps 事件属性，包含事件名称、页面、目标等可过滤的事件维度。
type EventProps struct {
	Name     string `json:"name"`
	Page     string `json:"page"`
	Goal     string `json:"goal"`
	Hostname string `json:"hostname"`
}

// FilterOperators 定义所有支持的过滤操作符常量。
var FilterOperators = struct {
	Is                 string
	IsNot              string
	Matches            string
	MatchesNot         string
	MatchesWildcard    string
	MatchesWildcardNot string
	Contains           string
	ContainsNot        string
	And                string
	Or                 string
	Not                string
	HasDone            string
	HasNotDone         string
}{
	Is:                 "is",
	IsNot:              "is_not",
	Matches:            "matches",
	MatchesNot:         "matches_not",
	MatchesWildcard:    "matches_wildcard",
	MatchesWildcardNot: "matches_wildcard_not",
	Contains:           "contains",
	ContainsNot:        "contains_not",
	And:                "and",
	Or:                 "or",
	Not:                "not",
	HasDone:            "has_done",
	HasNotDone:         "has_not_done",
}

// DimensionPrefix 定义维度前缀常量（visit: 和 event:）。
var DimensionPrefix = struct {
	Visit string
	Event string
}{
	Visit: "visit:",
	Event: "event:",
}
