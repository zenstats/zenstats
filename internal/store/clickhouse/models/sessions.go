package models

import (
	"net"
	"time"

	"github.com/zenstats/zenstats/pkg/geoip"
)

type Sessions struct {
	SessionId uint64 `json:"session_id" ch:"session_id"`

	Version   uint64    `json:"version" ch:"version"`
	Sign      int       `json:"sign" ch:"sign"`
	IsBounce  uint8     `json:"is_bounce" ch:"is_bounce"`
	Start     time.Time `json:"start" ch:"start"`
	Timestamp time.Time `json:"timestamp" ch:"timestamp"` // 时间戳
	EntryPage string    `json:"entry_page" ch:"entry_page"`
	ExitPage  string    `json:"exit_page" ch:"exit_page"`
	PageViews int32     `json:"pageviews" ch:"pageviews"`
	Events    int32     `json:"events" ch:"events"`
	Duration  uint32    `json:"duration" ch:"duration"`

	SiteId   uint64 `json:"site_id" ch:"site_id"`
	UserId   uint64 `json:"user_id" ch:"user_id"`
	HostName string `json:"hostname" ch:"hostname"`
	PathName string `json:"pathname" ch:"pathname"`

	EntryMetaKey   []string `json:"entry_meta.key" ch:"entry_meta.key"`
	EntryMetaValue []string `json:"entry_meta.value" ch:"entry_meta.value"`
	IP             net.IP   `json:"ipv4" ch:"ipv4"`
	IPv6           net.IP   `json:"ipv6" ch:"ipv6"`
	URL            string   `json:"url" ch:"url"` // URL
	UserAgent      string   `json:"user_agent" ch:"user_agent"`

	UtmMedium              string            `json:"utm_medium" ch:"utm_medium"`
	UtmSource              string            `json:"utm_source" ch:"utm_source"`
	UtmContent             string            `json:"utm_content" ch:"utm_content"`
	UtmTerm                string            `json:"utm_term" ch:"utm_term"`
	UtmCampaign            string            `json:"utm_campaign" ch:"utm_campaign"`
	Channel                string            `json:"channel" ch:"channel"`
	ScreenSize             string            `json:"screen_size" ch:"screen_size"`
	OperatingSystem        string            `json:"operating_system" ch:"operating_system"`
	OperatingSystemVersion string            `json:"operating_system_version" ch:"operating_system_version"`
	ExitPageHostname       string            `json:"exit_page_hostname" ch:"exit_page_hostname"`
	Browser                string            `json:"browser" ch:"browser"`
	BrowserVersion         string            `json:"browser_version" ch:"browser_version"`
	CityGeonameId          string            `json:"city_geoname_id" ch:"city_geoname_id"`
	CountryCode            string            `json:"country_code" ch:"country_code"`
	ContinentGeonameId     string            `json:"continent_geoname_id" ch:"continent_geoname_id"`
	Coordinates            geoip.Coordinates `json:"coordinates" ch:"coordinates"`
	Referrer               string            `json:"referrer" ch:"referrer"`               // 来源
	ReferrerSource         string            `json:"referrer_source" ch:"referrer_source"` // 来源
}

// type SessionAttrs struct {
// 	UtmMedium              string
// 	UtmSource              string
// 	UtmContent             string
// 	UtmTerm                string
// 	UtmCampaign            string
// 	Channel                string
// 	Device                 string
// 	OperatingSystem        string
// 	OperatingSystemVersion string
// 	Browser                string
// 	BrowserVersion         string
// 	CityGeonameId          string
// 	CountryCode            string
// 	ContinentGeonameId     string
// 	Coordinates            geoip.Coordinates
// 	Referrer               string
// 	ReferrerSource         string
// }
