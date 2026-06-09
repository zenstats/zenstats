package cmd

import (
	"context"
	"fmt"
	"math/rand"
	"net/url"
	"time"

	"github.com/spf13/cobra"
	"github.com/zenstats/zenstats/internal/common"
	"github.com/zenstats/zenstats/internal/event"
	"github.com/zenstats/zenstats/internal/store/postgresql"
	"github.com/zenstats/zenstats/pkg/generic"
	"github.com/zenstats/zenstats/pkg/globals"
)

var seedCmd = &cobra.Command{
	Use:   "seed",
	Short: "生成伪造测试数据，通过事件处理管道写入 ClickHouse",
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

	client := postgresql.NewClient()
	sites, err := client.Client.Site.Query().All(ctx)
	if err != nil {
		return fmt.Errorf("读取站点失败: %w", err)
	}
	if len(sites) == 0 {
		return fmt.Errorf("没有找到站点，请先创建站点")
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
	startDate := now.AddDate(0, -1, 0)

	for _, site := range sites {
		fmt.Printf("为站点 %s (ID: %d) 生成数据...\n", site.Domain, site.ID)
		count, err := generateSiteData(ctx, queue, site.ID, site.Domain, startDate, now)
		if err != nil {
			fmt.Printf("站点 %s 生成数据失败: %v\n", site.Domain, err)
			continue
		}
		fmt.Printf("站点 %s 已入队 %d 个事件\n", site.Domain, count)
	}

	fmt.Println("等待事件处理完成...")
	timeout := time.After(10 * time.Minute)
	for !queue.IsEmpty() {
		select {
		case <-timeout:
			fmt.Println("等待超时，强制关闭")
		 goto shutdown
		default:
			time.Sleep(500 * time.Millisecond)
			fmt.Printf("  队列剩余: %d\n", queue.Size())
		}
	}
	fmt.Println("队列已清空，等待缓冲区刷新...")
	time.Sleep(5 * time.Second)

shutdown:
	eventWork.Shutdown()
	fmt.Println("数据生成完成")
	return nil
}

func generateSiteData(ctx context.Context, queue *generic.DynamicQueue[*common.EventRequest], siteId int64, domain string, start, end time.Time) (int, error) {
	pages := []string{
		"/", "/about", "/contact", "/pricing", "/help", "/search", "/sitemap",
		"/news", "/events", "/careers",
		"/products", "/products/item-a", "/products/item-b", "/products/item-c",
		"/products/item-d", "/products/item-e",
		"/products/category/electronics", "/products/category/books",
		"/products/category/clothing", "/products/category/furniture",
		"/blog", "/blog/post-1", "/blog/post-2", "/blog/post-3",
		"/blog/post-4", "/blog/post-5",
		"/blog/category/tech", "/blog/category/lifestyle",
		"/blog/category/business", "/blog/archive/2024",
		"/docs", "/docs/getting-started", "/docs/api",
		"/docs/api/authentication", "/docs/api/endpoints", "/docs/api/rate-limits",
		"/docs/faq", "/docs/changelog",
		"/login", "/register", "/dashboard", "/settings",
		"/settings/profile", "/settings/security", "/settings/notifications",
		"/reset-password",
		"/tools", "/tools/seo-analyzer", "/tools/keyword-planner",
		"/tools/backlink-checker",
		"/downloads", "/downloads/windows", "/downloads/macos", "/downloads/linux",
		"/community", "/community/forums", "/community/guidelines",
		"/support", "/support/tickets", "/support/knowledge-base",
	}

	referrers := []string{
		"", "", "",
		"https://www.google.com", "https://www.baidu.com", "https://www.bing.com",
		"https://github.com", "https://twitter.com", "https://weibo.com", "https://zhihu.com",
	}

	browsers := []string{"Chrome", "Firefox", "Safari", "Edge", "Opera"}
	browserVersions := []string{"120.0.0.0", "121.0.0.0", "122.0.0.0", "119.0.0.0", "118.0.0.0"}
	osNames := []string{"Windows NT 10.0", "Macintosh; Intel Mac OS X 14_0", "X11; Linux x86_64", "Linux; Android 14", "iPhone; CPU iPhone OS 17_0"}

	utmSources := []string{"google", "facebook", "twitter", "weibo", "zhihu", "newsletter", "partner"}
	utmMediums := []string{"cpc", "social", "email", "referral", "organic", ""}
	utmCampaigns := []string{"spring_sale", "product_launch", "brand_awareness", "retargeting", "holiday_promo", ""}

	hourlyDistribution := []float64{
		0.01, 0.01, 0.01, 0.01, 0.01, 0.02,
		0.03, 0.05, 0.08, 0.10, 0.12, 0.11,
		0.09, 0.08, 0.09, 0.10, 0.11, 0.10,
		0.08, 0.07, 0.05, 0.04, 0.03, 0.02,
	}

	totalEvents := 0
	current := start
	for current.Before(end) || current.Equal(end) {
		dayOfWeek := current.Weekday()
		sessionsPerDay := 80 + rand.Intn(40)
		if dayOfWeek == time.Saturday || dayOfWeek == time.Sunday {
			sessionsPerDay = sessionsPerDay / 2
		}

		for i := 0; i < sessionsPerDay; i++ {
			hour := weightedRandom(hourlyDistribution)
			minute := rand.Intn(60)
			second := rand.Intn(60)

			sessionStart := time.Date(
				current.Year(), current.Month(), current.Day(),
				hour, minute, second, 0, time.UTC,
			)

			browserIdx := rand.Intn(len(browsers))
			osIdx := rand.Intn(len(osNames))
			refIdx := rand.Intn(len(referrers))
			ip := generateRandomIP()

			osName := osNames[osIdx]
			if osIdx == 1 {
				osName = fmt.Sprintf("Macintosh; Intel Mac OS X 10_15_%d", rand.Intn(7))
			}
			userAgent := fmt.Sprintf("Mozilla/5.0 (%s) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s Safari/537.36",
				osName, browserVersions[browserIdx])

			pageCount := 1 + rand.Intn(8)
			if rand.Float64() < 0.3 {
				pageCount = 1
			}

			for p := 0; p < pageCount; p++ {
				if p > 0 {
					sessionStart = sessionStart.Add(time.Duration(1+rand.Intn(60)) * time.Second)
				}

				pageIdx := rand.Intn(len(pages))
				pathname := pages[pageIdx]

				eventName := "pageview"
				engagementTime := 0
				scrollDepth := uint8(0)
				interactive := false
				if rand.Float64() < 0.2 && p > 0 {
					eventName = "engagement"
					engagementTime = 1000 + rand.Intn(30000)
					scrollDepth = uint8(20 + rand.Intn(80))
					interactive = true
				}

				eventURL := fmt.Sprintf("https://%s%s", domain, pathname)
				if rand.Float64() < 0.2 {
					utmMedium := utmMediums[rand.Intn(len(utmMediums))]
					utmSource := utmSources[rand.Intn(len(utmSources))]
					utmCampaign := utmCampaigns[rand.Intn(len(utmCampaigns))]
					params := url.Values{}
					if utmMedium != "" {
						params.Set("utm_medium", utmMedium)
					}
					if utmSource != "" {
						params.Set("utm_source", utmSource)
					}
					if utmCampaign != "" {
						params.Set("utm_campaign", utmCampaign)
					}
					if len(params) > 0 {
						eventURL = eventURL + "?" + params.Encode()
					}
				}

				req := &common.EventRequest{
					Timestamp:      sessionStart,
					EventName:      eventName,
					URL:            eventURL,
					Domain:         domain,
					Referrer:       referrers[refIdx],
					Props:          map[string]any{},
					EngagementTime: engagementTime,
					ScrollDepth:    scrollDepth,
					Interactive:    interactive,
					UserAgent:      userAgent,
					Ip:             ip,
				}

				if err := enqueueWithRetry(queue, req); err != nil {
					return totalEvents, fmt.Errorf("入队失败: %w", err)
				}
				totalEvents++
			}
		}

		fmt.Printf("  %s: 生成 %d 个会话\n", current.Format("2006-01-02"), sessionsPerDay)
		current = current.AddDate(0, 0, 1)
	}

	now := time.Now()
	realtimeCount := 5 + rand.Intn(6)
	for i := 0; i < realtimeCount; i++ {
		sessionStart := now.Add(-time.Duration(rand.Intn(300)) * time.Second)
		browserIdx := rand.Intn(len(browsers))
		osIdx := rand.Intn(len(osNames))
		refIdx := rand.Intn(len(referrers))
		ip := generateRandomIP()

		osName := osNames[osIdx]
		if osIdx == 1 {
			osName = fmt.Sprintf("Macintosh; Intel Mac OS X 10_15_%d", rand.Intn(7))
		}
		userAgent := fmt.Sprintf("Mozilla/5.0 (%s) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s Safari/537.36",
			osName, browserVersions[browserIdx])

		pageCount := 1 + rand.Intn(4)
		for p := 0; p < pageCount; p++ {
			if p > 0 {
				sessionStart = sessionStart.Add(time.Duration(1+rand.Intn(30)) * time.Second)
			}
			pageIdx := rand.Intn(len(pages))
			pathname := pages[pageIdx]

			eventName := "pageview"
			engagementTime := 0
			scrollDepth := uint8(0)
			interactive := false
			if rand.Float64() < 0.3 && p > 0 {
				eventName = "engagement"
				engagementTime = 1000 + rand.Intn(15000)
				scrollDepth = uint8(30 + rand.Intn(70))
				interactive = true
			}

			eventURL := fmt.Sprintf("https://%s%s", domain, pathname)
			if rand.Float64() < 0.3 {
				utmMedium := utmMediums[rand.Intn(len(utmMediums))]
				utmSource := utmSources[rand.Intn(len(utmSources))]
				utmCampaign := utmCampaigns[rand.Intn(len(utmCampaigns))]
				params := url.Values{}
				if utmMedium != "" {
					params.Set("utm_medium", utmMedium)
				}
				if utmSource != "" {
					params.Set("utm_source", utmSource)
				}
				if utmCampaign != "" {
					params.Set("utm_campaign", utmCampaign)
				}
				if len(params) > 0 {
					eventURL = eventURL + "?" + params.Encode()
				}
			}

			req := &common.EventRequest{
				Timestamp:      sessionStart,
				EventName:      eventName,
				URL:            eventURL,
				Domain:         domain,
				Referrer:       referrers[refIdx],
				Props:          map[string]any{},
				EngagementTime: engagementTime,
				ScrollDepth:    scrollDepth,
				Interactive:    interactive,
				UserAgent:      userAgent,
				Ip:             ip,
			}

			if err := enqueueWithRetry(queue, req); err != nil {
				return totalEvents, fmt.Errorf("入队失败: %w", err)
			}
			totalEvents++
		}
	}

	fmt.Printf("  实时: 生成 %d 个会话\n", realtimeCount)
	return totalEvents, nil
}

func generateRandomIP() string {
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

func init() {
	rand.Seed(time.Now().UnixNano())
}
