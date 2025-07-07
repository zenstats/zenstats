package models

import (
	"net"
	"time"

	"github.com/zenstats/zenstats/pkg/geoip"
)

type Events struct {
	SessionId              uint64            `json:"session_id" ch:"session_id"`
	Timestamp              time.Time         `json:"timestamp" ch:"timestamp"` // 时间戳
	Name                   string            `json:"name" ch:"name"`           // 事件名称
	SiteId                 uint64            `json:"site_id" ch:"site_id"`
	UserId                 uint64            `json:"user_id" ch:"user_id"`
	HostName               string            `json:"hostname" ch:"hostname"`
	PathName               string            `json:"pathname" ch:"pathname"`
	Referrer               string            `json:"referrer" ch:"referrer"`               // 来源
	ReferrerSource         string            `json:"referrer_source" ch:"referrer_source"` // 来源
	OperatingSystem        string            `json:"operating_system" ch:"operating_system"`
	OperatingSystemVersion string            `json:"operating_system_version" ch:"operating_system_version"`
	ScreenSize             string            `json:"screen_size" ch:"screen_size"`
	MetaKey                []string          `json:"meta.key" ch:"meta.key"`
	MetaValue              []string          `json:"meta.value" ch:"meta.value"`
	Browser                string            `json:"browser" ch:"browser"`
	BrowserVersion         string            `json:"browser_version" ch:"browser_version"`
	IP                     net.IP            `json:"ipv4" ch:"ipv4"`
	IPv6                   net.IP            `json:"ipv6" ch:"ipv6"`
	CountryCode            string            `json:"country_code" ch:"country_code"`
	ContinentGeonameId     string            `json:"continent_geoname_id" ch:"continent_geoname_id"`
	CityGeonameId          string            `json:"city_geoname_id" ch:"city_geoname_id"`
	Coordinates            geoip.Coordinates `json:"coordinates" ch:"coordinates"`
	URL                    string            `json:"url" ch:"url"`                         // URL
	EngagementTime         int               `json:"engagement_time" ch:"engagement_time"` // 参与时间
	ScrollDepth            uint8             `json:"scroll_depth" ch:"scroll_depth"`       // 滚动深度
	UserAgent              string            `json:"user_agent" ch:"user_agent"`
	Props                  map[string]any    `json:"props" ch:"props"` // 属性

	UtmMedium   string `json:"utm_medium" ch:"utm_medium"`
	UtmSource   string `json:"utm_source" ch:"utm_source"`
	UtmContent  string `json:"utm_content" ch:"utm_content"`
	UtmTerm     string `json:"utm_term" ch:"utm_term"`
	UtmCampaign string `json:"utm_campaign" ch:"utm_campaign"`
	Channel     string `json:"channel" ch:"channel"`

	Interactive bool `json:"interactive"` // 是否交互
}
