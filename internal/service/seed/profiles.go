// Package seed 提供测试种子数据生成功能。
// 包含设备、地域、引荐来源、页面等仿真数据配置，
// 以及加权随机选择辅助函数。
package seed

import (
	"fmt"
	"math/rand"
	"net/url"
	"strings"
)

// ---------------------------------------------------------------------------
// Device profiles
// ---------------------------------------------------------------------------

// DeviceProfile 描述一种设备/浏览器组合的 UA 模板和屏幕尺寸分布。
type DeviceProfile struct {
	Name        string
	Browser     string
	BrowserVer  []string
	OS          string
	OSVer       []string
	UATemplate  string // %s placeholders: osVer, browserVer
	ScreenSizes []string
	Weight      int // relative probability
	IsMobile    bool
}

var deviceProfiles = []DeviceProfile{
	{
		Name:        "Chrome Desktop",
		Browser:     "Chrome",
		BrowserVer:  []string{"120.0", "121.0", "122.0", "123.0", "124.0"},
		OS:          "Windows",
		OSVer:       []string{"10.0", "11.0"},
		UATemplate:  "Mozilla/5.0 (Windows NT %s; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s Safari/537.36",
		ScreenSizes: []string{"1920x1080", "2560x1440", "1440x900", "1366x768", "1536x864"},
		Weight:      35,
	},
	{
		Name:        "Chrome macOS",
		Browser:     "Chrome",
		BrowserVer:  []string{"120.0", "121.0", "122.0", "123.0", "124.0"},
		OS:          "Mac",
		OSVer:       []string{"14.0", "14.3", "14.5", "15.0"},
		UATemplate:  "Mozilla/5.0 (Macintosh; Intel Mac OS X %s) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s Safari/537.36",
		ScreenSizes: []string{"1680x1050", "2560x1600", "1440x900", "2048x1280"},
		Weight:      18,
	},
	{
		Name:        "Safari macOS",
		Browser:     "Safari",
		BrowserVer:  []string{"17.3", "17.4", "17.5", "18.0"},
		OS:          "Mac",
		OSVer:       []string{"14.0", "14.3", "14.5", "15.0"},
		UATemplate:  "Mozilla/5.0 (Macintosh; Intel Mac OS X %s) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/%s Safari/605.1.15",
		ScreenSizes: []string{"1680x1050", "2560x1600", "1440x900"},
		Weight:      10,
	},
	{
		Name:        "Firefox Desktop",
		Browser:     "Firefox",
		BrowserVer:  []string{"121.0", "122.0", "123.0", "124.0"},
		OS:          "Windows",
		OSVer:       []string{"10.0", "11.0"},
		UATemplate:  "Mozilla/5.0 (Windows NT %s; Win64; x64; rv:109.0) Gecko/20100101 Firefox/%s",
		ScreenSizes: []string{"1920x1080", "2560x1440", "1366x768"},
		Weight:      9,
	},
	{
		Name:        "Edge Desktop",
		Browser:     "Edge",
		BrowserVer:  []string{"120.0", "121.0", "122.0"},
		OS:          "Windows",
		OSVer:       []string{"10.0", "11.0"},
		UATemplate:  "Mozilla/5.0 (Windows NT %s; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s Safari/537.36 Edg/%s",
		ScreenSizes: []string{"1920x1080", "1366x768"},
		Weight:      5,
	},
	{
		Name:        "Chrome Android",
		Browser:     "Chrome Mobile",
		BrowserVer:  []string{"120.0", "121.0", "122.0", "123.0"},
		OS:          "Android",
		OSVer:       []string{"13.0", "14.0", "15.0"},
		UATemplate:  "Mozilla/5.0 (Linux; Android %s; SM-S908B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s Mobile Safari/537.36",
		ScreenSizes: []string{"412x915", "393x851", "390x844", "428x926"},
		Weight:      12,
		IsMobile:    true,
	},
	{
		Name:        "Safari iPhone",
		Browser:     "Safari Mobile",
		BrowserVer:  []string{"17.3", "17.4", "17.5"},
		OS:          "iPhone",
		OSVer:       []string{"17.0", "17.3", "17.5", "18.0"},
		UATemplate:  "Mozilla/5.0 (iPhone; CPU iPhone OS %s like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/%s Mobile/15E148 Safari/604.1",
		ScreenSizes: []string{"390x844", "393x852", "428x926"},
		Weight:      8,
		IsMobile:    true,
	},
	{
		Name:        "Safari iPad",
		Browser:     "Safari",
		BrowserVer:  []string{"17.3", "17.4", "17.5"},
		OS:          "iPad",
		OSVer:       []string{"17.0", "17.3", "17.5"},
		UATemplate:  "Mozilla/5.0 (iPad; CPU OS %s like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/%s Safari/605.1.15",
		ScreenSizes: []string{"1024x1366", "820x1180", "744x1133"},
		Weight:      3,
	},
}

// PickDevice 加权随机选择设备配置。
func PickDevice() DeviceProfile {
	total := 0
	for _, d := range deviceProfiles {
		total += d.Weight
	}
	r := rand.Intn(total)
	cum := 0
	for _, d := range deviceProfiles {
		cum += d.Weight
		if r < cum {
			return d
		}
	}
	return deviceProfiles[0]
}

// BuildUserAgent 根据设备配置生成 User-Agent 字符串。
func BuildUserAgent(d DeviceProfile) string {
	osVer := d.OSVer[rand.Intn(len(d.OSVer))]
	browserVer := d.BrowserVer[rand.Intn(len(d.BrowserVer))]
	switch d.Browser {
	case "Edge":
		return fmt.Sprintf(d.UATemplate, osVer, browserVer, browserVer)
	default:
		return fmt.Sprintf(d.UATemplate, osVer, browserVer)
	}
}

// ---------------------------------------------------------------------------
// Geo-distributed IPs (country → sample CIDR prefix)
// ---------------------------------------------------------------------------

// CountryIP 国家/地区 IP 段信息。
type CountryIP struct {
	Code      string
	Name      string
	Continent string
	Prefix    string // first two octets
	Weight    int
}

var countries = []CountryIP{
	{Code: "US", Name: "United States", Continent: "NA", Prefix: "104.", Weight: 25},
	{Code: "CN", Name: "China", Continent: "AS", Prefix: "42.", Weight: 18},
	{Code: "GB", Name: "United Kingdom", Continent: "EU", Prefix: "81.", Weight: 8},
	{Code: "DE", Name: "Germany", Continent: "EU", Prefix: "87.", Weight: 7},
	{Code: "JP", Name: "Japan", Continent: "AS", Prefix: "126.", Weight: 6},
	{Code: "FR", Name: "France", Continent: "EU", Prefix: "90.", Weight: 5},
	{Code: "BR", Name: "Brazil", Continent: "SA", Prefix: "177.", Weight: 5},
	{Code: "IN", Name: "India", Continent: "AS", Prefix: "103.", Weight: 5},
	{Code: "CA", Name: "Canada", Continent: "NA", Prefix: "142.", Weight: 4},
	{Code: "AU", Name: "Australia", Continent: "OC", Prefix: "1.", Weight: 3},
	{Code: "KR", Name: "South Korea", Continent: "AS", Prefix: "211.", Weight: 3},
	{Code: "NL", Name: "Netherlands", Continent: "EU", Prefix: "82.", Weight: 2},
	{Code: "SG", Name: "Singapore", Continent: "AS", Prefix: "116.", Weight: 2},
	{Code: "RU", Name: "Russia", Continent: "EU", Prefix: "95.", Weight: 2},
	{Code: "ES", Name: "Spain", Continent: "EU", Prefix: "88.", Weight: 2},
	{Code: "IT", Name: "Italy", Continent: "EU", Prefix: "93.", Weight: 2},
	{Code: "MX", Name: "Mexico", Continent: "NA", Prefix: "189.", Weight: 1},
}

// PickCountry 加权随机选择国家/地区。
func PickCountry() CountryIP {
	total := 0
	for _, c := range countries {
		total += c.Weight
	}
	r := rand.Intn(total)
	cum := 0
	for _, c := range countries {
		cum += c.Weight
		if r < cum {
			return c
		}
	}
	return countries[0]
}

// GenerateGeoIP 根据国家信息生成模拟 IP 地址。
func GenerateGeoIP(c CountryIP) string {
	third := rand.Intn(256)
	fourth := 1 + rand.Intn(254)
	return fmt.Sprintf("%s%d.%d", c.Prefix, third, fourth)
}

// ---------------------------------------------------------------------------
// Referrer profiles
// ---------------------------------------------------------------------------

type referrerProfile struct {
	URL    string
	Weight int
}

var referrers = []referrerProfile{
	{URL: "", Weight: 35}, // direct / no referrer
	{URL: "https://www.google.com/search?q={{query}}", Weight: 22},
	{URL: "https://www.baidu.com/s?wd={{query}}", Weight: 10},
	{URL: "https://www.bing.com/search?q={{query}}", Weight: 5},
	{URL: "https://www.google.com", Weight: 6},
	{URL: "https://github.com", Weight: 4},
	{URL: "https://twitter.com", Weight: 4},
	{URL: "https://www.reddit.com", Weight: 3},
	{URL: "https://t.co", Weight: 3},
	{URL: "https://www.zhihu.com", Weight: 3},
	{URL: "https://weibo.com", Weight: 2},
	{URL: "https://www.weibo.com", Weight: 2},
	{URL: "https://m.weibo.com", Weight: 2},
	{URL: "https://www.linkedin.com", Weight: 2},
	{URL: "https://news.ycombinator.com", Weight: 1},
}

var searchQueries = []string{
	"analytics platform", "web statistics tool", "site analytics open source",
	"javascript tracking library", "privacy friendly analytics", "self hosted analytics",
	"website visitor tracking", "real time analytics dashboard", "page view counter",
	"session recording tool", "heatmap analytics", "conversion tracking",
	"SEO analysis tool", "traffic source report", "bounce rate reduction",
	"user engagement metrics", "event tracking setup", "custom event analytics",
	"API documentation", "developer tools", "开源网站统计", "网站数据分析工具",
	"隐私友好的统计", "网站流量监控", "nginx access log", "docker-compose monitoring",
}

// BuildReferrer 生成一个随机 Referrer URL。
func BuildReferrer() string {
	r := pickWeightedReferrer()
	if r.URL == "" || !strings.Contains(r.URL, "{{query}}") {
		return r.URL
	}
	query := searchQueries[rand.Intn(len(searchQueries))]
	return strings.Replace(r.URL, "{{query}}", url.QueryEscape(query), 1)
}

func pickWeightedReferrer() referrerProfile {
	total := 0
	for _, r := range referrers {
		total += r.Weight
	}
	r := rand.Intn(total)
	cum := 0
	for _, ref := range referrers {
		cum += ref.Weight
		if r < cum {
			return ref
		}
	}
	return referrers[0]
}

// ---------------------------------------------------------------------------
// Pages & paths for realistic navigation
// ---------------------------------------------------------------------------

var pages = []string{
	"/",
	"/about", "/contact", "/pricing", "/help",
	"/blog", "/blog/announcing-v2", "/blog/migrating-from-ga4",
	"/blog/privacy-first-analytics", "/blog/self-hosted-guide",
	"/docs", "/docs/getting-started", "/docs/api",
	"/docs/api/authentication", "/docs/api/events", "/docs/api/sessions",
	"/docs/changelog", "/docs/faq",
	"/features", "/features/realtime", "/features/event-tracking",
	"/features/funnels", "/features/goals",
	"/pricing", "/pricing/enterprise",
	"/login", "/register", "/dashboard",
	"/settings", "/settings/profile", "/settings/sites",
	"/search", "/sitemap",
	"/community", "/community/showcase",
	"/downloads", "/downloads/desktop", "/downloads/mobile",
}

var pageWeights = []int{
	30,         // /
	5, 3, 3, 2, // about/contact/pricing/help
	8, 4, 3, 2, // blog/*
	6, 4, 3, 2, 2, 1, // docs/*
	3, 2, 2, // features/*
	3, 1, // pricing/*
	4, 3, 5, // login/register/dashboard
	3, 2, 1, // settings/*
	2, 1, // search/sitemap
	2, 1, // community/*
	2, 1, 1, // downloads/*
}

// PickPage 加权随机选择一个页面路径。
func PickPage() string {
	total := 0
	for _, w := range pageWeights {
		total += w
	}
	r := rand.Intn(total)
	cum := 0
	for i, w := range pageWeights {
		cum += w
		if r < cum {
			return pages[i]
		}
	}
	return pages[0]
}

// ---------------------------------------------------------------------------
// UTM dimensions
// ---------------------------------------------------------------------------

var utmSources = []string{"google", "facebook", "twitter", "linkedin",
	"newsletter", "partner-site", "reddit", "tiktok", "weibo", "zhihu"}

var utmMediums = []string{"cpc", "social", "email", "referral", "organic",
	"display", "affiliate", ""}

var utmCampaigns = []string{"spring_sale_2024", "product_launch_q1",
	"brand_awareness", "retargeting", "black_friday", "holiday_promo",
	"summer_discount", "back_to_school", ""}

var utmContents = []string{"banner_top", "sidebar_cta", "footer_link",
	"popup_modal", "email_header", "influencer_A", "influencer_B", ""}

var utmTerms = []string{"analytics", "web+stats", "tracking+tool",
	"self+hosted", "privacy", "open+source", ""}

// ---------------------------------------------------------------------------
// Event types (beyond pageview)
// ---------------------------------------------------------------------------

var outboundLinks = []string{
	"https://github.com/zenstats/zenstats",
	"https://twitter.com/zenstats",
	"https://www.google.com",
	"https://en.wikipedia.org/wiki/Web_analytics",
	"https://aws.amazon.com",
	"https://www.cloudflare.com",
	"https://github.com/gin-gonic/gin",
	"https://clickhouse.com",
	"https://www.postgresql.org",
	"https://grafana.com",
}

var fileDownloads = []string{
	"https://example.com/downloads/report-q4-2024.pdf",
	"https://example.com/downloads/presentation.pptx",
	"https://example.com/downloads/data-export.csv",
	"https://example.com/downloads/whitepaper-analytics.pdf",
	"https://example.com/downloads/ebook-getting-started.pdf",
	"https://example.com/downloads/sample-code.zip",
	"https://example.com/downloads/demo-video.mp4",
	"https://example.com/downloads/installation-guide.docx",
}

var customEvents = []struct {
	Name  string
	Props map[string]any
}{
	{Name: "Signup", Props: map[string]any{"plan": "free", "source": "organic"}},
	{Name: "Signup", Props: map[string]any{"plan": "pro", "source": "google_ads"}},
	{Name: "Button Click", Props: map[string]any{"id": "cta-primary", "page": "/pricing"}},
	{Name: "Button Click", Props: map[string]any{"id": "nav-docs", "page": "/"}},
	{Name: "Search", Props: map[string]any{"term": "analytics", "results": 42}},
	{Name: "Search", Props: map[string]any{"term": "api setup", "results": 7}},
	{Name: "Trial Started", Props: map[string]any{"days": "14"}},
	{Name: "Upgrade", Props: map[string]any{"from": "free", "to": "pro"}},
	{Name: "Share", Props: map[string]any{"method": "twitter"}},
	{Name: "Feedback", Props: map[string]any{"rating": "5"}},
}

// ---------------------------------------------------------------------------
// Hourly traffic distribution (24h)
// ---------------------------------------------------------------------------

var hourlyWeights = []float64{
	0.005, 0.003, 0.002, 0.001, 0.002, 0.008, // 0-5 am
	0.020, 0.040, 0.065, 0.080, 0.085, 0.080, // 6-11 am
	0.075, 0.085, 0.090, 0.085, 0.080, 0.075, // 12-5 pm
	0.060, 0.050, 0.040, 0.030, 0.020, 0.015, // 6-11 pm
}

// PickWeightedHour 按小时流量分布加权随机选择小时（0-23）。
func PickWeightedHour() int {
	total := 0.0
	for _, w := range hourlyWeights {
		total += w
	}
	r := rand.Float64() * total
	cum := 0.0
	for i, w := range hourlyWeights {
		cum += w
		if r <= cum {
			return i
		}
	}
	return len(hourlyWeights) - 1
}

// WeightedPick 按权重数组从 values 中返回对应项。
func WeightedPick(weights []int, values []int) int {
	total := 0
	for _, w := range weights {
		total += w
	}
	r := rand.Intn(total)
	cum := 0
	for i, w := range weights {
		cum += w
		if r < cum {
			return values[i]
		}
	}
	return values[len(values)-1]
}
