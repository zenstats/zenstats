package event

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log/slog"
	mathrand "math/rand"
	"net"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/zenstats/zenstats/internal/model"
	"github.com/zenstats/zenstats/internal/service"
	"github.com/zenstats/zenstats/internal/session"
	"golang.org/x/net/publicsuffix"

	"github.com/zenstats/zenstats/internal/store/clickhouse/models"
	"github.com/zenstats/zenstats/internal/store/clickhouse/repository"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent/systemconfig"
	"github.com/zenstats/zenstats/pkg/generic"
	"github.com/zenstats/zenstats/pkg/geoip"
	"github.com/zenstats/zenstats/pkg/globals"
	"github.com/zenstats/zenstats/pkg/pool"
	uaparser "github.com/zenstats/zenstats/pkg/ua_parser"
)

type EventWork struct {
	wg             sync.WaitGroup
	queue          *generic.DynamicQueue[*model.EventRequest]
	batchSize      int                      // batchSize 表示每次批量处理的任务数量
	taskChan       chan *model.EventRequest // taskChan 是一个通道，用于接收任务
	shutdownCtx    context.Context          // shutdownCtx 是一个取消上下文，用于关闭任务
	shutdownCancel context.CancelFunc       // shutdownCancel 是一个取消函数，用于取消任务
	pool           *pool.Pool

	writeBuffer *WriteBuffer

	uaparser       *uaparser.UAParser
	sessionManager *session.SessionManager
	siteService    *service.SiteService

	historicalThreshold time.Duration // historicalThreshold 表示历史数据阈值，超过此阈值的事件跳过会话管理

	hostPatternCache sync.Map // 缓存hostname正则表达式

	currentSalt string // 用户ID生成盐值，从数据库持久化加载，服务重启后保持不变
}

func NewEventWork(q *generic.DynamicQueue[*model.EventRequest], batchSize int, historicalThreshold time.Duration) (*EventWork, error) {
	ctx, cancel := context.WithCancel(context.Background())

	e := &EventWork{
		queue:               q,
		batchSize:           batchSize,
		taskChan:            make(chan *model.EventRequest, 1000),
		shutdownCtx:         ctx,
		shutdownCancel:      cancel,
		pool:                pool.NewPool(),
		uaparser:            uaparser.New(),
		sessionManager:      session.NewSessionManager(ctx, batchSize),
		siteService:         service.GetSiteService(),
		historicalThreshold: historicalThreshold,
		currentSalt:         loadOrCreateSalt(ctx),
	}
	e.writeBuffer = NewWriteBuffer(ctx, batchSize, time.Second*5)

	e.writeBuffer.Start()

	return e, nil
}

func (e *EventWork) Run() {
	slog.Info("Event worker started")

	// 启动任务分发协程
	e.wg.Add(1)
	go e.dispatch()

	// 启动处理协程
	e.wg.Add(1)
	go e.processWorker()
}

// 接收任务、处理任务，并对结果进行批处理以进行刷新。
func (e *EventWork) processWorker() {
	defer e.wg.Done()

	for {
		select {
		case item, ok := <-e.taskChan:
			if !ok {
				return
			}
			if item == nil {
				continue
			}
			// 分发到协程池处理
			e.pool.Submit(func() {
				processed := e.processEvent(item)
				if processed == nil {
					return
				}
				slog.Debug("process worker done", "request", item, "processed", processed)
				// WriteBuffer.Add 内部已有自己的 mutex 保护，无需外部额外加锁
				e.writeBuffer.Add(processed)
			})

		case <-e.shutdownCtx.Done():
			return
		}
	}
}

// dispatch 负责将任务分发到任务通道中。
func (e *EventWork) dispatch() {
	defer e.wg.Done()
	defer close(e.taskChan)

	for {
		select {
		case <-e.shutdownCtx.Done():
			return
		default:
			item := e.queue.Dequeue()
			if item == nil {
				continue
			}
			select {
			case e.taskChan <- item:
			case <-e.shutdownCtx.Done():
				return
			}
		}
	}
}

func (e *EventWork) processEvent(eventRequest *model.EventRequest) *models.Events {
	if eventRequest == nil {
		return nil
	}
	slog.Debug("processEvent", "request", eventRequest)

	var eventResult models.Events
	// 将eventRequest 转换为eventResult
	eventResult.Name = eventRequest.EventName
	eventResult.URL = eventRequest.URL
	eventResult.Props = eventRequest.Props
	eventResult.EngagementTime = eventRequest.EngagementTime
	eventResult.ScrollDepth = eventRequest.ScrollDepth
	eventResult.UserAgent = eventRequest.UserAgent
	eventResult.IP = net.ParseIP(eventRequest.Ip)
	// set timestamp
	eventResult.Timestamp = eventRequest.Timestamp
	eventResult.Interactive = eventRequest.Interactive
	// set siteid
	site, err := e.siteService.GetSiteByDomain(e.shutdownCtx, eventRequest.Domain)
	if err != nil {
		return nil
	}
	eventResult.SiteId = uint64(site.ID)

	// set userid and path
	userId, err := e.generateUserID(eventRequest.Ip, eventRequest.UserAgent, eventRequest.Domain)
	if err != nil {
		userId = 0
	}
	var pathname, hostname, urlstring string
	if !strings.Contains(eventRequest.URL, "://") {
		urlstring = "http://" + eventRequest.URL
	} else {
		urlstring = eventRequest.URL
	}
	parsedURL, err := url.Parse(urlstring)
	if err == nil {
		pathname = parsedURL.Path
		hostname = parsedURL.Host
		// SPA hash 路由支持：h=1 时 pathname 拼接 URL fragment
		// 例如 example.com/#/dashboard → /#/dashboard
		if eventRequest.Hash == 1 && parsedURL.Fragment != "" {
			pathname = parsedURL.Path + "#" + parsedURL.Fragment
		}
	}

	eventResult.UserId = userId
	eventResult.PathName = pathname
	eventResult.HostName = hostname

	// parse props — 限制数量和长度
	const maxProps = 30
	const maxPropKeyLength = 256
	const maxPropValueLength = 1024

	count := 0
	for key, value := range eventRequest.Props {
		if count >= maxProps {
			break
		}
		if len(key) == 0 || len(key) > maxPropKeyLength {
			continue
		}
		valueStr := fmt.Sprintf("%v", value)
		if len(valueStr) > maxPropValueLength {
			valueStr = valueStr[:maxPropValueLength]
		}
		eventResult.MetaKey = append(eventResult.MetaKey, key)
		eventResult.MetaValue = append(eventResult.MetaValue, valueStr)
		count++
	}

	/*
		1. 对用户UA进行验证 屏蔽非法请求  drop_verification_agent ->done
		2. 对IP进行验证 删除数据中心IP   drop_datacenter_ip  -> done（在 externalevent.go 入口层）
		3. 对hostname 进行验证 仅允许白名单  drop_shield_rule_hostname ->done
		4. 对pathname 进行验证 删除需要排除的路径 drop_shield_rule_page ->done
		5. 对IP进行验证 删除需要排除的ip drop_shield_rule_ip ->done
		6. 对IP的地理位置进行验证 屏蔽国家  drop_shield_rule_country  -- 这个放在PutGeolocation后执行 ->done
	*/

	// parse UA
	client := e.PutUserAgent(&eventResult)
	// drop_verification_agent
	if e.dropVerificationAgent(client) {
		slog.Debug("drop verification agent", "isbot", client.Screen.Bot)
		return nil
	}
	// 3. 对hostname进行验证 仅允许白名单
	if e.dropShieldRuleHostname(site, eventResult.HostName) {
		slog.Debug("hostname blocked by shield rule", "hostname", eventResult.HostName)
		return nil
	}
	// parse IP
	e.PutGeolocation(&eventResult)
	// 5. 对IP进行验证 删除需要排除的ip
	if e.dropShieldRuleIP(site, eventResult.IP) {
		slog.Debug("IP blocked by shield rule", "ip", eventResult.IP)
		return nil
	}
	// 6. 对IP的地理位置进行验证 屏蔽国家
	if e.dropShieldRuleCountry(site, eventResult.CountryCode) {
		slog.Debug("country blocked by shield rule", "country", eventResult.CountryCode)
		return nil
	}
	// parse source
	e.PutSourceInfo(&eventResult, eventRequest)

	// compute acquisition channel based on referrer and UTM parameters
	e.PutChannel(&eventResult)

	// 历史数据处理：超过阈值的事件跳过会话管理，直接生成会话记录
	if e.historicalThreshold > 0 && time.Since(eventRequest.Timestamp) > e.historicalThreshold {
		eventResult.SessionId = generateSeedSessionId(eventRequest.Timestamp)
		historicalSession := createHistoricalSession(&eventResult)
		e.sessionManager.WriteSession(historicalSession)
		return &eventResult
	}

	// register_session
	session, err := e.sessionManager.OnEvent(&eventResult)
	if err != nil {
		slog.Debug("register session error", "error", err)
		return nil
	}
	slog.Debug("register session success", "session", session)
	if session == nil {
		return nil
	}
	eventResult.SessionId = session.SessionId

	return &eventResult
}

func (e *EventWork) Shutdown() {
	e.shutdownCancel()

	done := make(chan struct{})
	go func() {
		e.queue.Close()
		e.wg.Wait()

		e.writeBuffer.Shutdown()
		e.sessionManager.Shutdown()
		e.pool.Release()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		slog.Warn("worker shutdown timeout")
	}

}

func (e *EventWork) generateUserID(ip, user_agent, domain string) (uint64, error) {
	// 使用启动时生成的随机 salt + 根域名做跨子域归一化
	salt := e.currentSalt
	rootDomain := getRootDomain(domain)

	hash := sha256.New()
	hash.Write([]byte(ip + user_agent + rootDomain + salt))
	hashBytes := hash.Sum(nil)

	return binary.LittleEndian.Uint64(hashBytes[:8]), nil
}

// getRootDomain 从 hostname 提取注册域名（如 www.example.com → example.com，
// www.example.co.uk → example.co.uk）。使用公共后缀列表正确处理多级 TLD。
// IPv4 地址和无法解析的 hostname 直接返回原值。
func getRootDomain(hostname string) string {
	if hostname == "" {
		return ""
	}
	// IP 地址直接返回
	if net.ParseIP(hostname) != nil {
		return hostname
	}
	// 去除端口号（如果有）
	if h, _, err := net.SplitHostPort(hostname); err == nil {
		hostname = h
	}
	// 使用公共后缀列表提取注册域名
	rootDomain, err := publicsuffix.EffectiveTLDPlusOne(strings.TrimRight(hostname, "."))
	if err != nil {
		// 回退：取最后两段
		parts := strings.Split(strings.TrimRight(hostname, "."), ".")
		if len(parts) >= 2 {
			return parts[len(parts)-2] + "." + parts[len(parts)-1]
		}
		return hostname
	}
	return rootDomain
}

// PutUserAgent parses the given user agent string and updates the session attributes
// with the extracted device, operating system, browser and version information.
//
// Parameters:
//   - ua: event
//
// The function uses the useragent.Parse method to extract:
//   - ScreenSize information (stored in event.ScreenSize)
//   - Operating system (stored in event.OperatingSystem)
//   - OS version (stored in event.OperatingSystemVersion)
//   - Browser name (stored in event.Browser)
//   - Browser version (stored in event.BrowserVersion)
func (e *EventWork) PutUserAgent(event *models.Events) *uaparser.Client {
	client := e.uaparser.Parse(event.UserAgent)

	event.ScreenSize = client.Screen.Family
	event.OperatingSystem = client.Os.Family
	event.OperatingSystemVersion = client.Os.ToVersionString()
	event.Browser = client.UserAgent.Family
	event.BrowserVersion = client.UserAgent.ToVersionString()

	return client
}

// PutGeolocation updates the session attributes with geolocation data based on the given IP address.
// It retrieves country, city, and continent information from the GeoIP database and stores their IDs in the session attributes.
func (e *EventWork) PutGeolocation(event *models.Events) {
	geoData, err := geoip.GetGeoIP().GetCountryAndRegion(event.IP.String())
	if err != nil {
		slog.Debug("GeoIP lookup failed, using defaults", "error", err, "ip", event.IP)
		event.CountryCode = "UN"
		return
	}
	repository := repository.GetLocationRepository()
	// create country
	repository.GetOrCreateById(context.Background(), "country", geoData.Country, geoData.IsoCode)
	city, err := repository.GetOrCreate(context.Background(), "city", geoData.City)
	if err == nil {
		event.CityGeonameId = city.ID
	}
	continent, err := repository.GetOrCreate(context.Background(), "continent", geoData.Continent)
	if err == nil {
		event.ContinentGeonameId = continent.ID
	}

	event.CountryCode = geoData.IsoCode
	if len(event.CountryCode) != 2 {
		event.CountryCode = "UN"
	}
	event.Coordinates = geoData.Coordinates
}

// PutSourceInfo updates the session attributes with source information from the event request.
// It extracts referrer information and UTM parameters from the request URL and stores them in the session attributes.
func (e *EventWork) PutSourceInfo(event *models.Events, eventRequest *model.EventRequest) {

	event.Referrer = e.formatReferrer(eventRequest.Referrer)
	parseReferer, err := url.Parse(eventRequest.Referrer)
	if err == nil {
		event.ReferrerSource = parseReferer.Host
	}

	parseUrl, err := url.Parse(eventRequest.URL)
	if err == nil {
		querys := parseUrl.Query()

		utmSource := querys.Get("utm_source")
		if utmSource == "" {
			utmSource = querys.Get("source")
		}
		if utmSource == "" {
			utmSource = querys.Get("ref")
		}
		event.UtmMedium = querys.Get("utm_medium")
		event.UtmSource = utmSource
		event.UtmContent = querys.Get("utm_content")
		event.UtmTerm = querys.Get("utm_term")
		event.UtmCampaign = querys.Get("utm_campaign")
	}
}

// PutChannel computes the acquisition channel based on referrer source and UTM parameters.
func (e *EventWork) PutChannel(event *models.Events) {
	// If UTM medium is set, classify by medium
	if event.UtmMedium != "" {
		medium := strings.ToLower(event.UtmMedium)
		switch {
		case medium == "paid" || medium == "cpc" || medium == "ppc":
			event.Channel = "Paid Search"
		case medium == "social" || medium == "social-network":
			event.Channel = "Social"
		case medium == "email":
			event.Channel = "Email"
		case medium == "affiliate":
			event.Channel = "Affiliate"
		case medium == "display" || medium == "banner" || medium == "cpm":
			event.Channel = "Display"
		case medium == "video":
			event.Channel = "Video"
		case medium == "audio":
			event.Channel = "Audio"
		case medium == "sms":
			event.Channel = "SMS"
		case medium == "push":
			event.Channel = "Push Notifications"
		default:
			event.Channel = "Other Campaign"
		}
		return
	}

	// If UTM source or campaign is set but no medium, classify as "Other Campaign"
	if event.UtmSource != "" || event.UtmCampaign != "" {
		event.Channel = "Other Campaign"
		return
	}

	// No UTM parameters, classify by referrer source
	if event.ReferrerSource == "" {
		event.Channel = "Direct"
		return
	}

	// Check if referrer source matches known search engines
	source := strings.ToLower(event.ReferrerSource)
	searchEngines := []string{"google", "bing", "baidu", "duckduckgo", "yahoo", "yandex", "ecosia", "qwant", "startpage", "brave"}
	for _, engine := range searchEngines {
		if strings.Contains(source, engine) {
			event.Channel = "Organic Search"
			return
		}
	}

	// Check social media
	socialNetworks := []string{"facebook", "twitter", "instagram", "linkedin", "pinterest", "tiktok", "reddit", "youtube", "weibo", "wechat", "douyin", "xiaohongshu"}
	for _, social := range socialNetworks {
		if strings.Contains(source, social) {
			event.Channel = "Social"
			return
		}
	}

	// Default: referral
	event.Channel = "Referral"
}

func (e *EventWork) formatReferrer(referrer string) string {
	parsedURL, err := url.Parse(referrer)
	if err != nil {
		return ""
	}

	host := parsedURL.Host
	path := strings.TrimSuffix(parsedURL.Path, "/")

	return host + path
}

func (e *EventWork) dropVerificationAgent(client *uaparser.Client) bool {
	return client.Screen.IsBot()
}

func (e *EventWork) dropShieldRuleHostname(site *ent.Site, hostname string) bool {
	domain := site.Domain
	cache := GetShieldRulesCache()

	rules, err := cache.GetHostnameRules(domain, func() ([]*ent.ShieldRulesHostname, error) {
		return e.siteService.ListShieldRuleHostname(e.shutdownCtx, site.ID)
	})
	if err != nil || len(rules) == 0 {
		return false
	}

	// 分离精确匹配和模式匹配的允许规则（白名单）
	exactHosts := make(map[string]struct{})
	var patternRules []*ent.ShieldRulesHostname

	for _, rule := range rules {
		if rule.Action != "allow" {
			continue
		}
		if rule.Hostname != "" {
			exactHosts[rule.Hostname] = struct{}{}
		}
		if rule.HostnamePattern != "" {
			patternRules = append(patternRules, rule)
		}
	}

	// 如果没有允许规则，阻止所有请求
	if len(exactHosts) == 0 && len(patternRules) == 0 {
		return true
	}

	// 检查精确匹配白名单
	if _, ok := exactHosts[hostname]; ok {
		return false
	}

	// 检查模式匹配白名单
	for _, rule := range patternRules {
		patternVal, ok := e.hostPatternCache.Load(rule.HostnamePattern)
		if !ok {
			unescapedPattern := strings.ReplaceAll(rule.HostnamePattern, `\\`, `\`)
			pattern, err := regexp.Compile(unescapedPattern)
			if err != nil {
				slog.Error("Failed to compile hostname pattern", "pattern", rule.HostnamePattern, "error", err)
				continue
			}
			patternVal, _ = e.hostPatternCache.LoadOrStore(rule.HostnamePattern, pattern)
		}
		pattern := patternVal.(*regexp.Regexp)
		if pattern.MatchString(hostname) {
			return false // 在白名单中，允许请求
		}
	}

	// 未匹配到任何白名单规则，阻止请求
	return true
}

func (e *EventWork) dropShieldRuleIP(site *ent.Site, ip net.IP) bool {
	domain := site.Domain
	cache := GetShieldRulesCache()

	rules, err := cache.GetIPRules(domain, func() ([]*ent.ShieldRulesIp, error) {
		return e.siteService.ListShieldRuleIP(e.shutdownCtx, site.ID)
	})
	if err != nil || len(rules) == 0 {
		return false
	}

	for _, rule := range rules {
		if rule.Inet.Contains(ip) {
			return rule.Action == "deny"
		}
	}
	return false
}

func (e *EventWork) dropShieldRuleCountry(site *ent.Site, countryCode string) bool {
	domain := site.Domain
	cache := GetShieldRulesCache()

	rules, err := cache.GetCountryRules(domain, func() ([]*ent.ShieldRulesCountry, error) {
		return e.siteService.ListShieldRuleCountry(e.shutdownCtx, site.ID)
	})
	if err != nil || len(rules) == 0 {
		return false
	}

	// 构建国家代码到action的map
	countryMap := make(map[string]string)
	for _, rule := range rules {
		countryMap[rule.CountryCode] = rule.Action
	}

	if action, ok := countryMap[countryCode]; ok {
		return action == "deny"
	}
	return false
}

// generateSeedSessionId 为种子数据生成会话ID
func generateSeedSessionId(t time.Time) uint64 {
	return (uint64(t.UnixNano()) << 24) | (uint64(mathrand.Intn(256)) << 16) | uint64(mathrand.Intn(65536))
}

// createHistoricalSession 为历史事件创建基础会话记录
func createHistoricalSession(event *models.Events) *models.Sessions {
	isBounce := uint8(1)
	if event.Name == "engagement" {
		isBounce = uint8(0)
	}

	pageViews := int32(0)
	if event.Name == "pageview" {
		pageViews = 1
	}

	session := &models.Sessions{
		Version:                1,
		Sign:                   1,
		Duration:               0,
		PageViews:              pageViews,
		Events:                 1,
		SessionId:              event.SessionId,
		SiteId:                 event.SiteId,
		UserId:                 event.UserId,
		Start:                  event.Timestamp,
		Timestamp:              event.Timestamp,
		IP:                     event.IP,
		IPv6:                   event.IPv6,
		HostName:               event.HostName,
		EntryPage:              event.PathName,
		ExitPage:               event.PathName,
		PathName:               event.PathName,
		URL:                    event.URL,
		EntryMetaKey:           event.MetaKey,
		EntryMetaValue:         event.MetaValue,
		IsBounce:               isBounce,
		UtmMedium:              event.UtmMedium,
		UtmSource:              event.UtmSource,
		UtmContent:             event.UtmContent,
		UtmTerm:                event.UtmTerm,
		UtmCampaign:            event.UtmCampaign,
		Channel:                event.Channel,
		ScreenSize:             event.ScreenSize,
		OperatingSystem:        event.OperatingSystem,
		OperatingSystemVersion: event.OperatingSystemVersion,
		Browser:                event.Browser,
		BrowserVersion:         event.BrowserVersion,
		CityGeonameId:          event.CityGeonameId,
		CountryCode:            event.CountryCode,
		ContinentGeonameId:     event.ContinentGeonameId,
		Coordinates:            event.Coordinates,
		Referrer:               event.Referrer,
		ReferrerSource:         event.ReferrerSource,
	}

	return session
}

// loadOrCreateSalt 从数据库 system_config 表加载 event_salt。
// 如果数据库中不存在，则生成新的 salt 并持久化存储，确保服务重启后用户 ID 不变。
// 如果记录存在但 value 为空（如 InitDefaults 初始化的空记录），
// 则用 Update 填充值，避免 Create 唯一键冲突静默失败导致 salt 永不写入。
func loadOrCreateSalt(ctx context.Context) string {
	db := globals.GetDB()
	if db == nil || db.Client == nil {
		slog.Warn("DB not available for salt loading, using random salt")
		return generateSalt()
	}

	// 尝试从 system_config 读取已持久化的 salt
	cfg, err := db.Client.SystemConfig.Query().
		Where(systemconfig.Key("general.event_salt")).
		Only(ctx)
	if err == nil {
		// 记录已存在：value 非空直接返回，为空则更新填充
		if cfg.Value != "" {
			return cfg.Value
		}
		newSalt := generateSalt()
		updated, err := db.Client.SystemConfig.UpdateOne(cfg).
			SetValue(newSalt).
			Save(ctx)
		if err != nil {
			slog.Warn("failed to update empty event_salt, using random salt", "error", err)
			return newSalt
		}
		slog.Info("regenerated empty event_salt in database")
		return updated.Value
	}

	// 记录不存在：生成新 salt 并持久化
	newSalt := generateSalt()
	created, err := db.Client.SystemConfig.Create().
		SetKey("general.event_salt").
		SetValue(newSalt).
		SetDescription("事件用户ID哈希盐值，用于生成匿名用户标识").
		SetGroupName("general").
		Save(ctx)
	if err != nil {
		slog.Warn("failed to persist event_salt, using random salt", "error", err)
		// 即使持久化失败也返回 salt，只是重启后会变化
		return newSalt
	}
	return created.Value
}

// generateSalt 生成一个 16 字节随机盐值用于用户 ID 哈希。
func generateSalt() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// fallback：用时间戳作为盐值
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
