package seed

import (
	"context"
	"fmt"
	"math/rand"
	"net/url"
	"time"

	"github.com/zenstats/zenstats/internal/model"
	"github.com/zenstats/zenstats/internal/event"
	"github.com/zenstats/zenstats/internal/store/postgresql"
	"github.com/zenstats/zenstats/pkg/generic"

	cl "github.com/zenstats/zenstats/internal/store/clickhouse"
)

// Generator 种子数据生成器。
type Generator struct {
	queue     *generic.DynamicQueue[*model.EventRequest]
	eventWork *event.EventWork
	client    *postgresql.Client
}

// NewGenerator 创建种子数据生成器。
// queue 和 eventWork 由调用方提前创建并启动。
func NewGenerator(queue *generic.DynamicQueue[*model.EventRequest], eventWork *event.EventWork, client *postgresql.Client) *Generator {
	return &Generator{
		queue:     queue,
		eventWork: eventWork,
		client:    client,
	}
}

// RunOptions 种子生成运行参数。
type RunOptions struct {
	Days  int  // 生成天数
	Clean bool // 是否清空已有 ClickHouse 数据
	Test  bool // 测试模式（固定随机种子 + 缩减数据量）
}

// Run 执行种子数据生成。
func (g *Generator) Run(ctx context.Context, opts RunOptions) error {
	sites, err := g.client.Client.Site.Query().All(ctx)
	if err != nil {
		return fmt.Errorf("读取站点失败: %w", err)
	}
	if len(sites) == 0 {
		return fmt.Errorf("没有找到站点，请先创建站点")
	}

	// 清空已有数据
	if opts.Clean {
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

	now := time.Now()
	startDate := now.AddDate(0, 0, -opts.Days)

	fmt.Printf("为 %d 个站点生成 %d 天数据 (%s → %s)\n\n",
		len(sites), opts.Days,
		startDate.Format("2006-01-02"), now.Format("2006-01-02"))

	totalEvents := 0
	for _, site := range sites {
		fmt.Printf("▸ 站点 %s (ID: %d)\n", site.Domain, site.ID)
		count, err := g.generateSiteData(site.Domain, startDate, now, opts.Test)
		if err != nil {
			fmt.Printf("  ✗ 失败: %v\n", err)
			continue
		}
		totalEvents += count
		fmt.Printf("  ✓ 入队 %d 个事件\n\n", count)
	}

	fmt.Printf("共入队 %d 个事件，等待处理...\n", totalEvents)
	timeout := time.After(10 * time.Minute)
	for !g.queue.IsEmpty() {
		select {
		case <-timeout:
			fmt.Println("等待超时，强制关闭")
			goto shutdown
		default:
			time.Sleep(500 * time.Millisecond)
			remaining := g.queue.Size()
			if remaining > 0 && remaining%1000 < 500 {
				fmt.Printf("  队列剩余: %d\n", remaining)
			}
		}
	}
	fmt.Println("队列已清空，等待缓冲区刷新...")
	time.Sleep(5 * time.Second)

shutdown:
	g.eventWork.Shutdown()
	fmt.Println("✅ 数据生成完成")
	return nil
}

// generateSiteData 为指定站点生成指定日期范围内的所有模拟事件。
func (g *Generator) generateSiteData(domain string, start, end time.Time, testMode bool) (int, error) {
	totalEvents := 0
	current := start

	for current.Before(end) || current.Equal(end) {
		dayOfWeek := current.Weekday()

		baseSessions := 80 + rand.Intn(40)
		if testMode {
			baseSessions = 30
		}
		switch dayOfWeek {
		case time.Saturday:
			baseSessions = int(float64(baseSessions) * 0.7)
		case time.Sunday:
			baseSessions = int(float64(baseSessions) * 0.6)
		}

		for s := 0; s < baseSessions; s++ {
			hour := PickWeightedHour()
			minute := rand.Intn(60)
			second := rand.Intn(60)

			sessionStart := time.Date(
				current.Year(), current.Month(), current.Day(),
				hour, minute, second, 0, time.UTC,
			)

			device := PickDevice()
			ua := BuildUserAgent(device)
			country := PickCountry()
			ip := GenerateGeoIP(country)
			referrer := BuildReferrer()
			screenSize := device.ScreenSizes[rand.Intn(len(device.ScreenSizes))]

			// Session-level UTM
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

			// Page view count per session
			pageCount := 1
			r := rand.Float64()
			if testMode {
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

			isBounce := pageCount == 1 && rand.Float64() < 0.6
			sessionEvents := 0

			for p := 0; p < pageCount; p++ {
				if p > 0 {
					gap := 5 + rand.Intn(120)
					sessionStart = sessionStart.Add(time.Duration(gap) * time.Second)
				}

				page := PickPage()
				eventURL := fmt.Sprintf("https://%s%s", domain, page)

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

				req := &model.EventRequest{
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

				if err := enqueueWithRetry(g.queue, req); err != nil {
					return totalEvents, fmt.Errorf("入队失败: %w", err)
				}
				totalEvents++
				sessionEvents++

				if p == 0 {
					referrer = fmt.Sprintf("https://%s%s", domain, pages[0])
				}
			}

			// Engagement event
			if !isBounce && sessionEvents >= 2 && rand.Float64() < 0.5 {
				engagementTime := 5000 + rand.Intn(45000)
				scrollDepth := uint8(20 + rand.Intn(80))

				lastPage := pages[rand.Intn(len(pages))]
				lastURL := fmt.Sprintf("https://%s%s", domain, lastPage)

				req := &model.EventRequest{
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
				if err := enqueueWithRetry(g.queue, req); err != nil {
					return totalEvents, fmt.Errorf("入队失败: %w", err)
				}
				totalEvents++
			}

			// Extra tracked events
			extraEventCount := WeightedPick([]int{70, 17, 8, 4, 1}, []int{0, 1, 2, 3, 4})
			for e := 0; e < extraEventCount; e++ {
				eventTime := sessionStart.Add(time.Duration(5+rand.Intn(180)) * time.Second)
				req := generateExtraEvent(domain, referrer, ua, ip, eventTime, screenSize)
				if req != nil {
					if err := enqueueWithRetry(g.queue, req); err != nil {
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
		device := PickDevice()
		ua := BuildUserAgent(device)
		country := PickCountry()
		ip := GenerateGeoIP(country)
		screenSize := device.ScreenSizes[rand.Intn(len(device.ScreenSizes))]
		eventTime := now.Add(-time.Duration(rand.Intn(300)) * time.Second)

		page := PickPage()
		eventURL := fmt.Sprintf("https://%s%s", domain, page)

		req := &model.EventRequest{
			Timestamp:   eventTime,
			EventName:   "pageview",
			URL:         eventURL,
			Domain:      domain,
			Referrer:    BuildReferrer(),
			Props:       map[string]any{"screen_size": screenSize},
			Interactive: false,
			UserAgent:   ua,
			Ip:          ip,
		}
		if err := enqueueWithRetry(g.queue, req); err != nil {
			return totalEvents, fmt.Errorf("入队失败: %w", err)
		}
		totalEvents++
	}

	return totalEvents, nil
}

// generateExtraEvent 生成非 pageview 事件（外链点击、文件下载、表单提交、自定义事件）。
func generateExtraEvent(domain, referrer, ua, ip string, eventTime time.Time, screenSize string) *model.EventRequest {
	r := rand.Float64()
	switch {
	case r < 0.25:
		link := outboundLinks[rand.Intn(len(outboundLinks))]
		return &model.EventRequest{
			Timestamp: eventTime,
			EventName: "Outbound Link: Click",
			URL:       fmt.Sprintf("https://%s%s", domain, PickPage()),
			Domain:    domain,
			Referrer:  referrer,
			Props:     map[string]any{"url": link, "screen_size": screenSize},
			UserAgent: ua,
			Ip:        ip,
		}
	case r < 0.45:
		file := fileDownloads[rand.Intn(len(fileDownloads))]
		return &model.EventRequest{
			Timestamp: eventTime,
			EventName: "File Download",
			URL:       fmt.Sprintf("https://%s%s", domain, PickPage()),
			Domain:    domain,
			Referrer:  referrer,
			Props:     map[string]any{"url": file, "screen_size": screenSize},
			UserAgent: ua,
			Ip:        ip,
		}
	case r < 0.60:
		return &model.EventRequest{
			Timestamp: eventTime,
			EventName: "Form: Submission",
			URL:       fmt.Sprintf("https://%s%s", domain, PickPage()),
			Domain:    domain,
			Referrer:  referrer,
			Props:     map[string]any{"screen_size": screenSize},
			UserAgent: ua,
			Ip:        ip,
		}
	default:
		ce := customEvents[rand.Intn(len(customEvents))]
		props := map[string]any{"screen_size": screenSize}
		for k, v := range ce.Props {
			props[k] = v
		}
		return &model.EventRequest{
			Timestamp: eventTime,
			EventName: ce.Name,
			URL:       fmt.Sprintf("https://%s%s", domain, PickPage()),
			Domain:    domain,
			Referrer:  referrer,
			Props:     props,
			UserAgent: ua,
			Ip:        ip,
		}
	}
}

// enqueueWithRetry 尝试将事件入队，带重试机制。
func enqueueWithRetry(queue *generic.DynamicQueue[*model.EventRequest], req *model.EventRequest) error {
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
