package cmd

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/spf13/cobra"
	cl "github.com/zenstats/zenstats/internal/store/clickhouse"
	"github.com/zenstats/zenstats/internal/store/clickhouse/models"
	"github.com/zenstats/zenstats/internal/store/postgresql"
	"github.com/zenstats/zenstats/pkg/geoip"
)

var seedCmd = &cobra.Command{
	Use:   "seed",
	Short: "生成伪造测试数据写入 ClickHouse",
	Run: func(cmd *cobra.Command, args []string) {
		Init()
		if err := runSeed(); err != nil {
			fmt.Printf("生成数据失败: %v\n", err)
		}
	},
}

func init() {
	RootCmd.AddCommand(seedCmd)
}

func runSeed() error {
	ctx := context.Background()

	// 读取所有站点
	client := postgresql.NewClient()
	sites, err := client.Client.Site.Query().All(ctx)
	if err != nil {
		return fmt.Errorf("读取站点失败: %w", err)
	}
	if len(sites) == 0 {
		return fmt.Errorf("没有找到站点，请先创建站点")
	}

	conn := cl.GetConnection()
	if conn == nil {
		return fmt.Errorf("ClickHouse 连接失败")
	}

	now := time.Now()
	startDate := now.AddDate(0, -1, 0) // 1个月前

	for _, site := range sites {
		fmt.Printf("为站点 %s (ID: %d) 生成数据...\n", site.Domain, site.ID)
		if err := generateSiteData(ctx, conn, site.ID, site.Domain, startDate, now); err != nil {
			fmt.Printf("站点 %s 生成数据失败: %v\n", site.Domain, err)
			continue
		}
		fmt.Printf("站点 %s 数据生成完成\n", site.Domain)
	}

	return nil
}

func generateSiteData(ctx context.Context, conn clickhouse.Conn, siteId int64, domain string, start, end time.Time) error {
	// 预定义的页面路径（稳定范围内随机）
	pages := []string{
		"/",
		"/about",
		"/contact",
		"/pricing",
		"/help",
		"/search",
		"/sitemap",
		"/news",
		"/events",
		"/careers",

		// 产品相关 (10个)
		"/products",
		"/products/item-a",
		"/products/item-b",
		"/products/item-c",
		"/products/item-d",
		"/products/item-e",
		"/products/category/electronics",
		"/products/category/books",
		"/products/category/clothing",
		"/products/category/furniture",

		// 博客相关 (10个)
		"/blog",
		"/blog/post-1",
		"/blog/post-2",
		"/blog/post-3",
		"/blog/post-4",
		"/blog/post-5",
		"/blog/category/tech",
		"/blog/category/lifestyle",
		"/blog/category/business",
		"/blog/archive/2024",

		// 文档相关 (8个)
		"/docs",
		"/docs/getting-started",
		"/docs/api",
		"/docs/api/authentication",
		"/docs/api/endpoints",
		"/docs/api/rate-limits",
		"/docs/faq",
		"/docs/changelog",

		// 用户中心与账户 (8个)
		"/login",
		"/register",
		"/dashboard",
		"/settings",
		"/settings/profile",
		"/settings/security",
		"/settings/notifications",
		"/reset-password",

		// 功能与应用页面 (8个)
		"/tools",
		"/tools/seo-analyzer",
		"/tools/keyword-planner",
		"/tools/backlink-checker",
		"/downloads",
		"/downloads/windows",
		"/downloads/macos",
		"/downloads/linux",

		// 社区与支持 (6个)
		"/community",
		"/community/forums",
		"/community/guidelines",
		"/support",
		"/support/tickets",
		"/support/knowledge-base",
	}

	// 预定义的 referrer 来源
	referrers := []string{
		"",
		"",
		"",
		"https://www.google.com",
		"https://www.baidu.com",
		"https://www.bing.com",
		"https://github.com",
		"https://twitter.com",
		"https://weibo.com",
		"https://zhihu.com",
	}

	referrerSources := []string{
		"",
		"",
		"",
		"google",
		"baidu",
		"bing",
		"github",
		"twitter",
		"weibo",
		"zhihu",
	}

	// 预定义的 UA 信息
	browsers := []string{"Chrome", "Firefox", "Safari", "Edge", "Opera"}
	browserVersions := []string{"120.0.0.0", "121.0.0.0", "122.0.0.0", "119.0.0.0", "118.0.0.0"}
	oses := []string{"Windows", "macOS", "Linux", "Android", "iOS"}
	osVersions := []string{"10", "11", "14", "15", "16", "13", "12"}
	screenSizes := []string{"1920x1080", "1366x768", "1536x864", "1440x900", "1280x720", "375x812", "390x844"}

	// 预定义的国家/城市
	countries := []string{"CN", "US", "JP", "KR", "DE", "GB", "FR", "CA", "AU", "BR"}
	cities := []string{"Beijing", "Shanghai", "Tokyo", "Seoul", "Berlin", "London", "Paris", "Toronto", "Sydney", "Sao Paulo"}
	continents := []string{"AS", "NA", "AS", "AS", "EU", "EU", "EU", "NA", "OC", "SA"}

	// 预定义的 UTM 参数
	utmSources := []string{"google", "facebook", "twitter", "weibo", "zhihu", "newsletter", "partner"}
	utmMediums := []string{"cpc", "social", "email", "referral", "organic", ""}
	utmCampaigns := []string{"spring_sale", "product_launch", "brand_awareness", "retargeting", "holiday_promo", ""}

	// 预定义的频道
	_ = []string{"Direct", "Organic Search", "Social", "Referral", "Email", "Paid Search"}

	// 生成用户池（模拟约 500 个独立用户）
	userCount := 1000
	users := make([]uint64, userCount)
	for i := 0; i < userCount; i++ {
		users[i] = generateUserID()
	}

	// 按天生成数据
	current := start
	for current.Before(end) || current.Equal(end) {
		// 每天生成的会话数（模拟真实流量模式）
		dayOfWeek := current.Weekday()
		sessionsPerDay := 80 + rand.Intn(40) // 基础 80-120 个会话
		if dayOfWeek == time.Saturday || dayOfWeek == time.Sunday {
			sessionsPerDay = sessionsPerDay / 2 // 周末流量减半
		}

		// 模拟小时级别的流量分布
		hourlyDistribution := []float64{
			0.01, 0.01, 0.01, 0.01, 0.01, 0.02, // 0-5点
			0.03, 0.05, 0.08, 0.10, 0.12, 0.11, // 6-11点
			0.09, 0.08, 0.09, 0.10, 0.11, 0.10, // 12-17点
			0.08, 0.07, 0.05, 0.04, 0.03, 0.02, // 18-23点
		}

		var events []*models.Events
		var sessions []*models.Sessions

		for i := 0; i < sessionsPerDay; i++ {
			// 根据小时分布选择随机小时
			hour := weightedRandom(hourlyDistribution)
			minute := rand.Intn(60)
			second := rand.Intn(60)

			sessionStart := time.Date(
				current.Year(), current.Month(), current.Day(),
				hour, minute, second, 0, time.UTC,
			)

			// 随机选择用户
			userId := users[rand.Intn(len(users))]

			// 随机选择设备信息
			browserIdx := rand.Intn(len(browsers))
			osIdx := rand.Intn(len(oses))
			countryIdx := rand.Intn(len(countries))
			refIdx := rand.Intn(len(referrers))

			// 生成 session ID
			sessionId := generateSessionId(sessionStart)

			// 模拟 1-8 个页面的会话
			pageCount := 1 + rand.Intn(8)
			if rand.Float64() < 0.3 {
				pageCount = 1 // 30% 的跳出率
			}

			// 生成 IP 地址
			ip := generateRandomIP()
			userAgent := fmt.Sprintf("Mozilla/5.0 (%s; %s) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s Safari/537.36",
				oses[osIdx], osVersions[rand.Intn(len(osVersions))], browserVersions[browserIdx])

			var sessionEvents []*models.Events
			entryPage := ""
			exitPage := ""
			var sessionTimestamp time.Time

			for p := 0; p < pageCount; p++ {
				// 页面间随机间隔 1-60 秒
				if p > 0 {
					sessionStart = sessionStart.Add(time.Duration(1+rand.Intn(60)) * time.Second)
				}

				pageIdx := rand.Intn(len(pages))
				pathname := pages[pageIdx]
				hostname := domain

				if entryPage == "" {
					entryPage = pathname
				}
				exitPage = pathname
				sessionTimestamp = sessionStart

				// 随机决定是否为 engagement 事件
				eventName := "pageview"
				engagementTime := 0
				scrollDepth := uint8(0)
				if rand.Float64() < 0.2 && p > 0 {
					eventName = "engagement"
					engagementTime = 1000 + rand.Intn(30000) // 1-30秒
					scrollDepth = uint8(20 + rand.Intn(80))  // 20-100%
				}

				// 生成 UTM 参数（只有部分请求有）
				utmMedium := ""
				utmSource := ""
				utmCampaign := ""
				if rand.Float64() < 0.2 {
					utmMedium = utmMediums[rand.Intn(len(utmMediums))]
					utmSource = utmSources[rand.Intn(len(utmSources))]
					utmCampaign = utmCampaigns[rand.Intn(len(utmCampaigns))]
				}

				// 计算频道
				channel := "Direct"
				if referrers[refIdx] != "" {
					channel = "Referral"
				}
				if utmMedium == "cpc" {
					channel = "Paid Search"
				}
				if utmMedium == "social" {
					channel = "Social"
				}

				event := &models.Events{
					SessionId:              sessionId,
					Timestamp:              sessionStart,
					Name:                   eventName,
					SiteId:                 uint64(siteId),
					UserId:                 userId,
					HostName:               hostname,
					PathName:               pathname,
					Referrer:               referrers[refIdx],
					ReferrerSource:         referrerSources[refIdx],
					OperatingSystem:        oses[osIdx],
					OperatingSystemVersion: osVersions[rand.Intn(len(osVersions))],
					ScreenSize:             screenSizes[rand.Intn(len(screenSizes))],
					MetaKey:                []string{},
					MetaValue:              []string{},
					Browser:                browsers[browserIdx],
					BrowserVersion:         browserVersions[browserIdx],
					IP:                     net.ParseIP(ip),
					IPv6:                   net.IPv4zero,
					CountryCode:            countries[countryIdx],
					ContinentGeonameId:     continents[countryIdx],
					CityGeonameId:          cities[countryIdx],
					Coordinates:            geoip.Coordinates{Latitude: rand.Float64()*180 - 90, Longitude: rand.Float64()*360 - 180},
					URL:                    fmt.Sprintf("https://%s%s", hostname, pathname),
					EngagementTime:         engagementTime,
					ScrollDepth:            scrollDepth,
					UserAgent:              userAgent,
					Props:                  map[string]any{},
					UtmMedium:              utmMedium,
					UtmSource:              utmSource,
					UtmContent:             "",
					UtmTerm:                "",
					UtmCampaign:            utmCampaign,
					Channel:                channel,
					Interactive:            eventName == "engagement",
				}
				sessionEvents = append(sessionEvents, event)
				events = append(events, event)
			}

			// 生成 session
			isBounce := uint8(0)
			if pageCount == 1 {
				isBounce = 1
			}

			session := &models.Sessions{
				SessionId:              sessionId,
				Version:                1,
				Sign:                   1,
				IsBounce:               isBounce,
				Start:                  sessionEvents[0].Timestamp,
				Timestamp:              sessionTimestamp,
				EntryPage:              entryPage,
				ExitPage:               exitPage,
				PageViews:              int32(pageCount),
				Events:                 int32(len(sessionEvents)),
				Duration:               uint32(sessionTimestamp.Sub(sessionEvents[0].Timestamp).Seconds()),
				SiteId:                 uint64(siteId),
				UserId:                 userId,
				HostName:               domain,
				PathName:               exitPage,
				EntryMetaKey:           []string{},
				EntryMetaValue:         []string{},
				IP:                     net.ParseIP(ip),
				IPv6:                   net.IPv4zero,
				URL:                    fmt.Sprintf("https://%s%s", domain, exitPage),
				UserAgent:              userAgent,
				UtmMedium:              sessionEvents[0].UtmMedium,
				UtmSource:              sessionEvents[0].UtmSource,
				UtmContent:             "",
				UtmTerm:                "",
				UtmCampaign:            sessionEvents[0].UtmCampaign,
				Channel:                sessionEvents[0].Channel,
				ScreenSize:             sessionEvents[0].ScreenSize,
				OperatingSystem:        sessionEvents[0].OperatingSystem,
				OperatingSystemVersion: sessionEvents[0].OperatingSystemVersion,
				Browser:                sessionEvents[0].Browser,
				BrowserVersion:         sessionEvents[0].BrowserVersion,
				CityGeonameId:          sessionEvents[0].CityGeonameId,
				CountryCode:            sessionEvents[0].CountryCode,
				ContinentGeonameId:     sessionEvents[0].ContinentGeonameId,
				Coordinates:            sessionEvents[0].Coordinates,
				Referrer:               sessionEvents[0].Referrer,
				ReferrerSource:         sessionEvents[0].ReferrerSource,
			}
			sessions = append(sessions, session)
		}

		// 批量写入 events
		if len(events) > 0 {
			if err := batchInsertEvents(ctx, conn, events); err != nil {
				return fmt.Errorf("写入 events 失败: %w", err)
			}
		}

		// 批量写入 sessions
		if len(sessions) > 0 {
			if err := batchInsertSessions(ctx, conn, sessions); err != nil {
				return fmt.Errorf("写入 sessions 失败: %w", err)
			}
		}

		fmt.Printf("  %s: 生成 %d 个会话, %d 个事件\n",
			current.Format("2006-01-02"), len(sessions), len(events))

		current = current.AddDate(0, 0, 1)
	}

	// 生成少量当前时间的事件，确保实时面板有数据
	now := time.Now()
	var realtimeEvents []*models.Events
	var realtimeSessions []*models.Sessions
	realtimeCount := 5 + rand.Intn(6) // 5-10 个会话
	for i := 0; i < realtimeCount; i++ {
		userId := users[rand.Intn(len(users))]
		browserIdx := rand.Intn(len(browsers))
		osIdx := rand.Intn(len(oses))
		countryIdx := rand.Intn(len(countries))
		refIdx := rand.Intn(len(referrers))
		ip := generateRandomIP()
		userAgent := fmt.Sprintf("Mozilla/5.0 (%s; %s) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s Safari/537.36",
			oses[osIdx], osVersions[rand.Intn(len(osVersions))], browserVersions[browserIdx])

		sessionStart := now.Add(-time.Duration(rand.Intn(300)) * time.Second) // 最近5分钟内
		sessionId := generateSessionId(sessionStart)
		pageCount := 1 + rand.Intn(4) // 1-4 页
		var entryPage, exitPage string
		var sessionEvents []*models.Events

		for p := 0; p < pageCount; p++ {
			if p > 0 {
				sessionStart = sessionStart.Add(time.Duration(1+rand.Intn(30)) * time.Second)
			}
			pageIdx := rand.Intn(len(pages))
			pathname := pages[pageIdx]
			if entryPage == "" {
				entryPage = pathname
			}
			exitPage = pathname

			eventName := "pageview"
			engagementTime := 0
			scrollDepth := uint8(0)
			if rand.Float64() < 0.3 && p > 0 {
				eventName = "engagement"
				engagementTime = 1000 + rand.Intn(15000)
				scrollDepth = uint8(30 + rand.Intn(70))
			}

			utmMedium := ""
			utmSource := ""
			utmCampaign := ""
			if rand.Float64() < 0.3 {
				utmMedium = utmMediums[rand.Intn(len(utmMediums))]
				utmSource = utmSources[rand.Intn(len(utmSources))]
				utmCampaign = utmCampaigns[rand.Intn(len(utmCampaigns))]
			}
			channel := "Direct"
			if referrers[refIdx] != "" {
				channel = "Referral"
			}
			if utmMedium == "cpc" {
				channel = "Paid Search"
			}

			event := &models.Events{
				SessionId:              sessionId,
				Timestamp:              sessionStart,
				Name:                   eventName,
				SiteId:                 uint64(siteId),
				UserId:                 userId,
				HostName:               domain,
				PathName:               pathname,
				Referrer:               referrers[refIdx],
				ReferrerSource:         referrerSources[refIdx],
				OperatingSystem:        oses[osIdx],
				OperatingSystemVersion: osVersions[rand.Intn(len(osVersions))],
				ScreenSize:             screenSizes[rand.Intn(len(screenSizes))],
				MetaKey:                []string{},
				MetaValue:              []string{},
				Browser:                browsers[browserIdx],
				BrowserVersion:         browserVersions[browserIdx],
				IP:                     net.ParseIP(ip),
				IPv6:                   net.IPv4zero,
				CountryCode:            countries[countryIdx],
				ContinentGeonameId:     continents[countryIdx],
				CityGeonameId:          cities[countryIdx],
				Coordinates:            geoip.Coordinates{Latitude: rand.Float64()*180 - 90, Longitude: rand.Float64()*360 - 180},
				URL:                    fmt.Sprintf("https://%s%s", domain, pathname),
				EngagementTime:         engagementTime,
				ScrollDepth:            scrollDepth,
				UserAgent:              userAgent,
				Props:                  map[string]any{},
				UtmMedium:              utmMedium,
				UtmSource:              utmSource,
				UtmContent:             "",
				UtmTerm:                "",
				UtmCampaign:            utmCampaign,
				Channel:                channel,
				Interactive:            eventName == "engagement",
			}
			sessionEvents = append(sessionEvents, event)
			realtimeEvents = append(realtimeEvents, event)
		}

		isBounce := uint8(0)
		if pageCount == 1 {
			isBounce = 1
		}
		sessionTimestamp := sessionStart
		realtimeSessions = append(realtimeSessions, &models.Sessions{
			SessionId:              sessionId,
			Version:                1,
			Sign:                   1,
			IsBounce:               isBounce,
			Start:                  sessionEvents[0].Timestamp,
			Timestamp:              sessionTimestamp,
			EntryPage:              entryPage,
			ExitPage:               exitPage,
			PageViews:              int32(pageCount),
			Events:                 int32(len(sessionEvents)),
			Duration:               uint32(sessionTimestamp.Sub(sessionEvents[0].Timestamp).Seconds()),
			SiteId:                 uint64(siteId),
			UserId:                 userId,
			HostName:               domain,
			PathName:               exitPage,
			EntryMetaKey:           []string{},
			EntryMetaValue:         []string{},
			IP:                     net.ParseIP(ip),
			IPv6:                   net.IPv4zero,
			URL:                    fmt.Sprintf("https://%s%s", domain, exitPage),
			UserAgent:              userAgent,
			UtmMedium:              sessionEvents[0].UtmMedium,
			UtmSource:              sessionEvents[0].UtmSource,
			UtmContent:             "",
			UtmTerm:                "",
			UtmCampaign:            sessionEvents[0].UtmCampaign,
			Channel:                sessionEvents[0].Channel,
			ScreenSize:             sessionEvents[0].ScreenSize,
			OperatingSystem:        sessionEvents[0].OperatingSystem,
			OperatingSystemVersion: sessionEvents[0].OperatingSystemVersion,
			Browser:                sessionEvents[0].Browser,
			BrowserVersion:         sessionEvents[0].BrowserVersion,
			CityGeonameId:          sessionEvents[0].CityGeonameId,
			CountryCode:            sessionEvents[0].CountryCode,
			ContinentGeonameId:     sessionEvents[0].ContinentGeonameId,
			Coordinates:            sessionEvents[0].Coordinates,
			Referrer:               sessionEvents[0].Referrer,
			ReferrerSource:         sessionEvents[0].ReferrerSource,
		})
	}

	if len(realtimeEvents) > 0 {
		if err := batchInsertEvents(ctx, conn, realtimeEvents); err != nil {
			return fmt.Errorf("写入实时事件失败: %w", err)
		}
	}
	if len(realtimeSessions) > 0 {
		if err := batchInsertSessions(ctx, conn, realtimeSessions); err != nil {
			return fmt.Errorf("写入实时会话失败: %w", err)
		}
	}
	fmt.Printf("  实时: 生成 %d 个会话, %d 个事件\n", len(realtimeSessions), len(realtimeEvents))

	return nil
}

func batchInsertEvents(ctx context.Context, conn clickhouse.Conn, events []*models.Events) error {
	batch, err := conn.PrepareBatch(ctx, `INSERT INTO events (
		timestamp, name, site_id, user_id, session_id,
		url, hostname, pathname, referrer, referrer_source,
		operating_system, utm_medium, utm_source, utm_content, utm_term, utm_campaign,
		"meta.key", "meta.value", screen_size, browser, browser_version,
		user_agent, operating_system_version, engagement_time, scroll_depth,
		ipv4, country_code, continent_geoname_id, city_geoname_id, coordinates,
		ipv6, channel
	)`)
	if err != nil {
		return err
	}

	for _, e := range events {
		coordinates := []float64{e.Coordinates.Latitude, e.Coordinates.Longitude}
		err = batch.Append(
			e.Timestamp,
			e.Name,
			e.SiteId,
			e.UserId,
			e.SessionId,
			e.URL,
			e.HostName,
			e.PathName,
			e.Referrer,
			e.ReferrerSource,
			e.OperatingSystem,
			e.UtmMedium,
			e.UtmSource,
			e.UtmContent,
			e.UtmTerm,
			e.UtmCampaign,
			e.MetaKey,
			e.MetaValue,
			e.ScreenSize,
			e.Browser,
			e.BrowserVersion,
			e.UserAgent,
			e.OperatingSystemVersion,
			e.EngagementTime,
			e.ScrollDepth,
			e.IP,
			e.CountryCode,
			e.ContinentGeonameId,
			e.CityGeonameId,
			coordinates,
			e.IPv6,
			e.Channel,
		)
		if err != nil {
			return err
		}
	}

	return batch.Send()
}

func batchInsertSessions(ctx context.Context, conn clickhouse.Conn, sessions []*models.Sessions) error {
	batch, err := conn.PrepareBatch(ctx, `INSERT INTO sessions (
		"start", timestamp, session_id, version, sign, is_bounce,
		entry_page, exit_page, pageviews, events, duration,
		site_id, user_id, url, hostname, pathname,
		referrer, referrer_source, operating_system, utm_medium, utm_source,
		utm_content, utm_term, utm_campaign, "entry_meta.key", "entry_meta.value",
		screen_size, browser, browser_version, user_agent, operating_system_version,
		ipv4, country_code, continent_geoname_id, city_geoname_id, coordinates,
		ipv6, channel
	)`)
	if err != nil {
		return err
	}

	for _, s := range sessions {
		coordinates := []float64{s.Coordinates.Latitude, s.Coordinates.Longitude}
		err = batch.Append(
			s.Start,
			s.Timestamp,
			s.SessionId,
			s.Version,
			s.Sign,
			s.IsBounce,
			s.EntryPage,
			s.ExitPage,
			s.PageViews,
			s.Events,
			s.Duration,
			s.SiteId,
			s.UserId,
			s.URL,
			s.HostName,
			s.PathName,
			s.Referrer,
			s.ReferrerSource,
			s.OperatingSystem,
			s.UtmMedium,
			s.UtmSource,
			s.UtmContent,
			s.UtmTerm,
			s.UtmCampaign,
			s.EntryMetaKey,
			s.EntryMetaValue,
			s.ScreenSize,
			s.Browser,
			s.BrowserVersion,
			s.UserAgent,
			s.OperatingSystemVersion,
			s.IP,
			s.CountryCode,
			s.ContinentGeonameId,
			s.CityGeonameId,
			coordinates,
			s.IPv6,
			s.Channel,
		)
		if err != nil {
			return err
		}
	}

	return batch.Send()
}

func generateUserID() uint64 {
	return uint64(rand.Int63())
}

func generateSessionId(t time.Time) uint64 {
	return (uint64(t.UnixNano()) << 24) | (uint64(rand.Intn(256)) << 16) | uint64(rand.Intn(65536))
}

func generateRandomIP() string {
	// 避免生成 0.x.x.x 或 127.x.x.x 等特殊 IP
	first := 1 + rand.Intn(223)
	second := rand.Intn(256)
	third := rand.Intn(256)
	fourth := 1 + rand.Intn(254)
	return fmt.Sprintf("%d.%d.%d.%d", first, second, third, fourth)
}

func weightedRandom(weights []float64) int {
	total := 0.0
	for _, w := range weights {
		total += w
	}

	r := rand.Float64() * total
	cumulative := 0.0
	for i, w := range weights {
		cumulative += w
		if r <= cumulative {
			return i
		}
	}

	return len(weights) - 1
}

func init() {
	// 设置随机种子
	rand.Seed(time.Now().UnixNano())
}
