package event

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/zenstats/zenstats/internal/common"
	"github.com/zenstats/zenstats/internal/service"
	"github.com/zenstats/zenstats/internal/session"
	"github.com/zenstats/zenstats/internal/store/clickhouse/models"
	"github.com/zenstats/zenstats/internal/store/clickhouse/repository"
	"github.com/zenstats/zenstats/pkg/generic"
	"github.com/zenstats/zenstats/pkg/geoip"
	"github.com/zenstats/zenstats/pkg/pool"
	uaparser "github.com/zenstats/zenstats/pkg/ua_parser"
)

type EventWork struct {
	wg             sync.WaitGroup
	queue          *generic.DynamicQueue[*common.EventRequest]
	batchSize      int                       // batchSize 表示每次批量处理的任务数量
	taskChan       chan *common.EventRequest // taskChan 是一个通道，用于接收任务
	shutdownCtx    context.Context           // shutdownCtx 是一个取消上下文，用于关闭任务
	shutdownCancel context.CancelFunc        // shutdownCancel 是一个取消函数，用于取消任务
	pool           *pool.Pool

	writeBuffer *WriteBuffer

	uaparser       *uaparser.UAParser
	sessionManager *session.SessionManager
	siteService    *service.SiteService
}

func NewEventWork(q *generic.DynamicQueue[*common.EventRequest], batchSize int) (*EventWork, error) {
	ctx, cancel := context.WithCancel(context.Background())

	e := &EventWork{
		queue:          q,
		batchSize:      batchSize,
		taskChan:       make(chan *common.EventRequest, 1000),
		shutdownCtx:    ctx,
		shutdownCancel: cancel,
		pool:           pool.NewPool(),
		uaparser:       uaparser.New(),
		sessionManager: session.NewSessionManager(ctx, batchSize),
		siteService:    service.GetSiteService(),
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

	lock := sync.Mutex{}

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
				lock.Lock()
				defer lock.Unlock()
				slog.Debug("process worker done", "request", item, "processed", processed)

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

func (e *EventWork) processEvent(eventRequest *common.EventRequest) *models.Events {
	if eventRequest == nil {
		return nil
	}
	slog.Debug("processEvent", "request", eventRequest)

	var eventResult models.Events
	// 将eventRequest 转换为eventResult
	eventResult.Name = eventRequest.EventName
	eventResult.URL = eventRequest.URL
	eventResult.HostName = eventRequest.Domain
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
	var pathname string
	parsedURL, err := url.Parse(eventRequest.URL)
	if err == nil {
		pathname = parsedURL.Path
	}
	eventResult.UserId = userId
	eventResult.PathName = pathname
	// parse props
	for key, value := range eventRequest.Props {
		eventResult.MetaKey = append(eventResult.MetaKey, key)
		eventResult.MetaValue = append(eventResult.MetaValue, fmt.Sprintf("%v", value))
	}
	//TODO 部分逻辑过滤
	/*
		1. 对用户UA进行验证 屏蔽非法请求  drop_verification_agent ->done
		2. 对IP进行验证 删除数据中心IP   drop_datacenter_ip  -> not now
		3. 对hostname 进行验证 仅允许白名单  drop_shield_rule_hostname
		4. 对pathname 进行验证 删除需要排除的路径 drop_shield_rule_page
		5. 对IP进行验证 删除需要排除的ip drop_shield_rule_ip
		6. 对IP的地理位置进行验证 屏蔽国家  drop_shield_rule_country  -- 这个放在PutGeolocation后执行
	*/

	// parse UA
	client := e.PutUserAgent(&eventResult)
	// drop_verification_agent
	if e.dropVerificationAgent(client) {
		slog.Debug("drop verification agent", "isbot", client.Screen.Bot)
		return nil
	}
	// parse IP
	e.PutGeolocation(&eventResult)
	// parse source
	e.PutSourceInfo(&eventResult, eventRequest)

	// register_session
	session, err := e.sessionManager.OnEvent(&eventResult)
	if err != nil {
		slog.Error("register session error", "error", err)
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
	salt := "" // 后续看是否需要在启动时生成随机salt

	hash := sha256.New()
	hash.Write([]byte(ip + user_agent + domain + salt))
	hashBytes := hash.Sum(nil)

	return binary.LittleEndian.Uint64(hashBytes[:8]), nil
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
		slog.Error("Failed to get geolocation data", "error", err, "ip", event.IP)
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
	event.Coordinates = geoData.Coordinates
}

// PutSourceInfo updates the session attributes with source information from the event request.
// It extracts referrer information and UTM parameters from the request URL and stores them in the session attributes.
func (e *EventWork) PutSourceInfo(event *models.Events, eventRequest *common.EventRequest) {

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
