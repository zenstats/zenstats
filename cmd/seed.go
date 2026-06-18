package cmd

import (
	"context"
	"fmt"
	"math/rand"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/zenstats/zenstats/internal/common"
	"github.com/zenstats/zenstats/internal/event"
	cl "github.com/zenstats/zenstats/internal/store/clickhouse"
	"github.com/zenstats/zenstats/internal/store/postgresql"
	"github.com/zenstats/zenstats/pkg/generic"
	"github.com/zenstats/zenstats/pkg/globals"
)

var (
	seedDays  int
	seedClean bool
	seedTest  bool
)

var seedCmd = &cobra.Command{
	Use:   "seed",
	Short: "生成测试数据，通过事件管道写入 ClickHouse",
	Long: `生成仿真的多维度测试数据。

测试模式 (--test):
  使用固定随机种子生成确定性数据，适合集成测试验证。
  数据规格: 3 天、每天 30 个会话、每会话 1-3 个 pageview
  预期产出约 150-200 个 pageview 事件、30-60 个自定义事件`,
	Run: func(cmd *cobra.Command, args []string) {
		Init()
		if seedTest {
			seedDays = 3
			rand.Seed(42) // 固定随机种子 → 确定性数据
		} else {
			rand.Seed(time.Now().UnixNano())
		}
		if err := runSeed(); err != nil {
			fmt.Printf("生成数据失败: %v\n", err)
		}
	},
}

func init() {
	seedCmd.Flags().IntVarP(&seedDays, "days", "d", 30, "生成多少天的历史数据")
	seedCmd.Flags().BoolVarP(&seedClean, "clean", "c", false, "生成前清空已有数据")
	seedCmd.Flags().BoolVar(&seedTest, "test", false, "测试模式：固定随机种子，生成确定性小数据集")
	RootCmd.AddCommand(seedCmd)
}

// ---------------------------------------------------------------------------
// Seed data profiles
// ---------------------------------------------------------------------------

// Device profile: user agent template + typical screen sizes
type deviceProfile struct {
	Name        string
	Browser     string
	BrowserVer  []string
	OS          string
	OSVer       []string
	UATemplate  string // %s placeholders: browser, os, osVer, browserVer
	ScreenSizes []string
	Weight      int // relative probability
	IsMobile    bool
}

var deviceProfiles = []deviceProfile{
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

// weighted random device selection
func pickDevice() deviceProfile {
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

func buildUserAgent(d deviceProfile) string {
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
type countryIP struct {
	Code      string
	Name      string
	Continent string
	Prefix    string // first two octets
	Weight    int
}

var countries = []countryIP{
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

func pickCountry() countryIP {
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

func generateGeoIP(c countryIP) string {
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
	{URL: "https://www.google.com", Weight: 6}, // social / organic
	{URL: "https://github.com", Weight: 4},
	{URL: "https://twitter.com", Weight: 4},
	{URL: "https://www.reddit.com", Weight: 3},
	{URL: "https://t.co", Weight: 3},
	{URL: "https://www.zhihu.com", Weight: 3},
	{URL: "https://weibo.com", Weight: 2},
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

func buildReferrer() string {
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

// Page weight: higher = more frequent pageviews
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

func pickPage() string {
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

func pickWeightedHour() int {
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

// ---------------------------------------------------------------------------
// Main seed logic
// ---------------------------------------------------------------------------

func runSeed() error {
	ctx := context.Background()

	client := postgresql.NewClient()
	sites, err := client.Client.Site.Query().All(ctx)
	if err != nil {
		return fmt.Errorf("读取站点失败: %w", err)
	}
	if len(sites) == 0 {
		return fmt.Errorf("没有找到站点，请先创建站点")
	}

	// 清空已有数据
	if seedClean {
		fmt.Println("清空已有 ClickHouse 数据...")
		conn := cl.GetConnection()
		if conn != nil {
			for _, table := range []string{"events", "sessions", "location_data"} {
				if err := conn.Exec(ctx, fmt.Sprintf("TRUNCATE TABLE zenstats_events_db.%s", table)); err != nil {
					fmt.Printf("  清空 %s 失败（可忽略）: %v\n", table, err)
				}
			}
		}
		fmt.Println("数据已清空")
	}

	queue := globals.GetQueue()
	if queue == nil {
		return fmt.Errorf("队列未初始化")
	}

	eventWork, err := event.NewEventWork(queue, 1024, 24*time.Hour)
	if err != nil {
		return fmt.Errorf("创建事件处理器失败: %w", err)
	}
	eventWork.Run()

	now := time.Now()
	startDate := now.AddDate(0, 0, -seedDays)

	fmt.Printf("为 %d 个站点生成 %d 天数据 (%s → %s)\n\n",
		len(sites), seedDays,
		startDate.Format("2006-01-02"), now.Format("2006-01-02"))

	totalEvents := 0
	for _, site := range sites {
		fmt.Printf("▸ 站点 %s (ID: %d)\n", site.Domain, site.ID)
		count, err := generateSiteData(queue, site.Domain, startDate, now)
		if err != nil {
			fmt.Printf("  ✗ 失败: %v\n", err)
			continue
		}
		totalEvents += count
		fmt.Printf("  ✓ 入队 %d 个事件\n\n", count)
	}

	fmt.Printf("共入队 %d 个事件，等待处理...\n", totalEvents)
	timeout := time.After(10 * time.Minute)
	for !queue.IsEmpty() {
		select {
		case <-timeout:
			fmt.Println("等待超时，强制关闭")
			goto shutdown
		default:
			time.Sleep(500 * time.Millisecond)
			remaining := queue.Size()
			if remaining > 0 && remaining%1000 < 500 {
				fmt.Printf("  队列剩余: %d\n", remaining)
			}
		}
	}
	fmt.Println("队列已清空，等待缓冲区刷新...")
	time.Sleep(5 * time.Second)

shutdown:
	eventWork.Shutdown()
	fmt.Println("✅ 数据生成完成")
	return nil
}

func generateSiteData(queue *generic.DynamicQueue[*common.EventRequest], domain string, start, end time.Time) (int, error) {
	totalEvents := 0
	current := start

	for current.Before(end) || current.Equal(end) {
		dayOfWeek := current.Weekday()

		// Base sessions per day
		baseSessions := 80 + rand.Intn(40)
		if seedTest {
			baseSessions = 30 // 测试模式：每天固定 30 个会话
		}
		switch dayOfWeek {
		case time.Saturday:
			baseSessions = int(float64(baseSessions) * 0.7)
		case time.Sunday:
			baseSessions = int(float64(baseSessions) * 0.6)
		}

		for s := 0; s < baseSessions; s++ {
			hour := pickWeightedHour()
			minute := rand.Intn(60)
			second := rand.Intn(60)

			sessionStart := time.Date(
				current.Year(), current.Month(), current.Day(),
				hour, minute, second, 0, time.UTC,
			)

			device := pickDevice()
			ua := buildUserAgent(device)
			country := pickCountry()
			ip := generateGeoIP(country)
			referrer := buildReferrer()
			screenSize := device.ScreenSizes[rand.Intn(len(device.ScreenSizes))]

			// Session-level props
			hasUTM := rand.Float64() < 0.25
			source, medium, campaign, content, utmTerm := "", "", "", "", ""
			if hasUTM {
				medium = utmMediums[rand.Intn(len(utmMediums))]
				source = utmSources[rand.Intn(len(utmSources))]
				if medium != "" || source != "" {
					campaign = utmCampaigns[rand.Intn(len(utmCampaigns))]
				}
				if medium == "cpc" || medium == "social" {
					content = utmContents[rand.Intn(len(utmContents))]
				}
				if medium == "cpc" {
					utmTerm = utmTerms[rand.Intn(len(utmTerms))]
				}
			}

			// Page view count per session (power-law: most sessions have 1-3 pageviews)
			pageCount := 1
			r := rand.Float64()
			if seedTest {
				// 测试模式：固定分布 — 1 页(50%)、2 页(30%)、3 页(20%)
				switch {
				case r < 0.50:
					pageCount = 1
				case r < 0.80:
					pageCount = 2
				default:
					pageCount = 3
				}
			} else {
				switch {
				case r < 0.40:
					pageCount = 1
				case r < 0.65:
					pageCount = 2
				case r < 0.80:
					pageCount = 3
				case r < 0.90:
					pageCount = 4
				case r < 0.95:
					pageCount = 5
				case r < 0.98:
					pageCount = 6 + rand.Intn(3)
				default:
					pageCount = 10 + rand.Intn(15)
				}
			}

			// Bounce: single page with no engagement
			isBounce := pageCount == 1 && rand.Float64() < 0.6

			sessionEvents := 0
			for p := 0; p < pageCount; p++ {
				if p > 0 {
					// Time between pageviews
					gap := 5 + rand.Intn(120)
					sessionStart = sessionStart.Add(time.Duration(gap) * time.Second)
				}

				page := pickPage()
				eventURL := fmt.Sprintf("https://%s%s", domain, page)

				// Add UTM to first pageview only
				if p == 0 && hasUTM {
					params := url.Values{}
					if source != "" {
						params.Set("utm_source", source)
					}
					if medium != "" {
						params.Set("utm_medium", medium)
					}
					if campaign != "" {
						params.Set("utm_campaign", campaign)
					}
					if content != "" {
						params.Set("utm_content", content)
					}
					if utmTerm != "" {
						params.Set("utm_term", utmTerm)
					}
					if len(params) > 0 {
						eventURL = eventURL + "?" + params.Encode()
					}
				}

				// Hash-based routing (SPA) — 5% of pageviews
				if page != "/" && rand.Float64() < 0.05 {
					eventURL = fmt.Sprintf("https://%s/#%s", domain, page)
				}

				req := &common.EventRequest{
					Timestamp:      sessionStart,
					EventName:      "pageview",
					URL:            eventURL,
					Domain:         domain,
					Referrer:       referrer,
					Props:          map[string]any{"screen_size": screenSize},
					EngagementTime: 0,
					ScrollDepth:    0,
					Interactive:    false,
					UserAgent:      ua,
					Ip:             ip,
				}

				if err := enqueueWithRetry(queue, req); err != nil {
					return totalEvents, fmt.Errorf("入队失败: %w", err)
				}
				totalEvents++
				sessionEvents++

				// After first pageview, referrer is the site itself (simulating internal nav)
				if p == 0 {
					referrer = fmt.Sprintf("https://%s%s", domain, pages[0])
				}
			}

			// Engagement event (after session, if not bounce & has multiple pages)
			if !isBounce && sessionEvents >= 2 && rand.Float64() < 0.5 {
				engagementTime := 5000 + rand.Intn(45000)
				scrollDepth := uint8(20 + rand.Intn(80))

				lastPage := pages[rand.Intn(len(pages))]
				lastURL := fmt.Sprintf("https://%s%s", domain, lastPage)

				req := &common.EventRequest{
					Timestamp:      sessionStart.Add(time.Duration(10+rand.Intn(180)) * time.Second),
					EventName:      "engagement",
					URL:            lastURL,
					Domain:         domain,
					Referrer:       referrer,
					Props:          map[string]any{"screen_size": screenSize},
					EngagementTime: engagementTime,
					ScrollDepth:    scrollDepth,
					Interactive:    true,
					UserAgent:      ua,
					Ip:             ip,
				}
				if err := enqueueWithRetry(queue, req); err != nil {
					return totalEvents, fmt.Errorf("入队失败: %w", err)
				}
				totalEvents++
			}

			// Extra tracked events (outbound links, file downloads, forms, custom)
			extraEventCount := weightedPick([]int{70, 17, 8, 4, 1}, []int{0, 1, 2, 3, 4})
			for e := 0; e < extraEventCount; e++ {
				eventTime := sessionStart.Add(time.Duration(5+rand.Intn(180)) * time.Second)
				req := generateExtraEvent(domain, referrer, ua, ip, eventTime, screenSize)
				if req != nil {
					if err := enqueueWithRetry(queue, req); err != nil {
						return totalEvents, fmt.Errorf("入队失败: %w", err)
					}
					totalEvents++
				}
			}
		}

		current = current.AddDate(0, 0, 1)
	}

	// Real-time events (last 5 minutes)
	now := time.Now()
	realtimeSessions := 3 + rand.Intn(8)
	for s := 0; s < realtimeSessions; s++ {
		device := pickDevice()
		ua := buildUserAgent(device)
		country := pickCountry()
		ip := generateGeoIP(country)
		screenSize := device.ScreenSizes[rand.Intn(len(device.ScreenSizes))]
		eventTime := now.Add(-time.Duration(rand.Intn(300)) * time.Second)

		page := pickPage()
		eventURL := fmt.Sprintf("https://%s%s", domain, page)

		req := &common.EventRequest{
			Timestamp:   eventTime,
			EventName:   "pageview",
			URL:         eventURL,
			Domain:      domain,
			Referrer:    buildReferrer(),
			Props:       map[string]any{"screen_size": screenSize},
			Interactive: false,
			UserAgent:   ua,
			Ip:          ip,
		}
		if err := enqueueWithRetry(queue, req); err != nil {
			return totalEvents, fmt.Errorf("入队失败: %w", err)
		}
		totalEvents++
	}

	return totalEvents, nil
}

// ---------------------------------------------------------------------------
// Extra event generation
// ---------------------------------------------------------------------------
func generateExtraEvent(domain, referrer, ua, ip string, eventTime time.Time, screenSize string) *common.EventRequest {
	r := rand.Float64()
	switch {
	case r < 0.25:
		// Outbound Link: Click
		link := outboundLinks[rand.Intn(len(outboundLinks))]
		return &common.EventRequest{
			Timestamp: eventTime,
			EventName: "Outbound Link: Click",
			URL:       fmt.Sprintf("https://%s%s", domain, pickPage()),
			Domain:    domain,
			Referrer:  referrer,
			Props:     map[string]any{"url": link, "screen_size": screenSize},
			UserAgent: ua,
			Ip:        ip,
		}
	case r < 0.45:
		// File Download
		file := fileDownloads[rand.Intn(len(fileDownloads))]
		return &common.EventRequest{
			Timestamp: eventTime,
			EventName: "File Download",
			URL:       fmt.Sprintf("https://%s%s", domain, pickPage()),
			Domain:    domain,
			Referrer:  referrer,
			Props:     map[string]any{"url": file, "screen_size": screenSize},
			UserAgent: ua,
			Ip:        ip,
		}
	case r < 0.60:
		// Form: Submission
		return &common.EventRequest{
			Timestamp: eventTime,
			EventName: "Form: Submission",
			URL:       fmt.Sprintf("https://%s%s", domain, pickPage()),
			Domain:    domain,
			Referrer:  referrer,
			Props:     map[string]any{"screen_size": screenSize},
			UserAgent: ua,
			Ip:        ip,
		}
	default:
		// Custom event
		ce := customEvents[rand.Intn(len(customEvents))]
		props := map[string]any{"screen_size": screenSize}
		for k, v := range ce.Props {
			props[k] = v
		}
		return &common.EventRequest{
			Timestamp: eventTime,
			EventName: ce.Name,
			URL:       fmt.Sprintf("https://%s%s", domain, pickPage()),
			Domain:    domain,
			Referrer:  referrer,
			Props:     props,
			UserAgent: ua,
			Ip:        ip,
		}
	}
}

// weightedPick returns n where weights[n] is selected by probability.
func weightedPick(weights []int, values []int) int {
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

// ---------------------------------------------------------------------------
// Queue helpers
// ---------------------------------------------------------------------------
func enqueueWithRetry(queue *generic.DynamicQueue[*common.EventRequest], req *common.EventRequest) error {
	maxRetries := 30
	for i := 0; i < maxRetries; i++ {
		err := queue.Enqueue(req)
		if err == nil {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("队列持续满，重试 %d 次后放弃", maxRetries)
}
