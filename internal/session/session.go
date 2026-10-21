package session

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"sync/atomic"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/zenstats/zenstats/internal/store/clickhouse/models"
	"github.com/zenstats/zenstats/internal/store/clickhouse/repository"
)

// DefaultSessionCacheSize 是 session LRU 缓存的默认容量（活跃 visitor 数上限）。
// 原值 1000 过小：活跃访客超过 1000 时 session 被 LRU 淘汰，下一事件到来只能回读 ClickHouse，
// 而此时 writeBuffer 中可能还有尚未 flush 的更新版本 → 读到旧行、再生成新行，
// 产生永远无法 collapse 的重复 session。200k 个 *Sessions 约几十~两百 MB 内存，
// 对单实例足够覆盖绝大多数站点的 30 分钟活跃访客数。
const DefaultSessionCacheSize = 200_000

// sessionCacheTTL 与业务上的 session 超时（30 分钟无活动）一致。
const sessionCacheTTL = 30 * time.Minute

// dbLookupTimeout 限制缓存 miss 时回读 ClickHouse 的耗时，
// 避免 CH 慢查询拖住 balancer worker（同一 worker 上所有用户的事件都会被阻塞）。
const dbLookupTimeout = 2 * time.Second

type SessionManager struct {
	machineID      uint64
	balancer       *Balancer[*models.Sessions]
	writeBuffer    *WriteBuffer
	shutdownCtx    context.Context
	shutdownCancel context.CancelFunc
	versionCounter uint64 // 单调递增的版本号计数器

	cache *expirable.LRU[string, *models.Sessions]
}

// NewSessionManager 使用 DefaultSessionCacheSize 创建 SessionManager（签名保持兼容）。
func NewSessionManager(ctx context.Context, batchSize int) *SessionManager {
	return NewSessionManagerWithCacheSize(ctx, batchSize, DefaultSessionCacheSize)
}

// NewSessionManagerWithCacheSize 允许显式指定 session LRU 缓存容量；cacheSize<=0 时回退到默认值。
func NewSessionManagerWithCacheSize(ctx context.Context, batchSize int, cacheSize int) *SessionManager {
	ctx, cancel := context.WithCancel(ctx)
	if cacheSize <= 0 {
		cacheSize = DefaultSessionCacheSize
	}
	l := expirable.NewLRU[string, *models.Sessions](cacheSize, nil, sessionCacheTTL)

	s := &SessionManager{
		shutdownCtx:    ctx,
		shutdownCancel: cancel,
		balancer:       NewBalancer[*models.Sessions](),
		cache:          l,
		machineID:      rand.Uint64(),
		// 以纳秒墙钟为版本号基准，保证重启后新版本号始终大于历史行，
		// 避免 ORDER BY version DESC 读到重启前的陈旧行。
		versionCounter: uint64(time.Now().UnixNano()),
	}

	s.writeBuffer = NewWriteBuffer(ctx, batchSize, time.Second*5)
	s.writeBuffer.Start()

	return s
}

func (s *SessionManager) OnEvent(event *models.Events) (*models.Sessions, error) {
	slog.Debug("on event", "event", event)
	session, err := s.balancer.Dispatch(
		event.UserId,
		func() (*models.Sessions, error) {
			findSession := s.findSession(event)

			session, err := s.handleEvent(event, findSession)
			return session, err
		},
		time.Second*5,
	)
	if err != nil {
		return session, err
	}

	return session, nil
}

func (s *SessionManager) findSession(event *models.Events) *models.Sessions {
	session, _ := s.cache.Get(s.generateSessionCacheKey(event.UserId, event.SiteId))
	return session
}

func (s *SessionManager) handleEvent(event *models.Events, findSession *models.Sessions) (*models.Sessions, error) {
	// Cache expiry is an implementation detail; session membership is determined
	// by event time even after a cache miss, restart, or out-of-order arrival.
	if !sessionContainsEvent(findSession, event.Timestamp) {
		loaded, err := s.loadSessionFromDB(event)
		if err != nil && !errors.Is(err, repository.ErrSessionNotFound) {
			return nil, err
		}
		findSession = loaded
	}
	if findSession == nil {
		if event.Name == "engagement" {
			return nil, nil
		}
		created := s.newSession(event)
		s.writeBuffer.Add(created)
		s.updateSessionCache(created)
		return created, nil
	}

	old := s.CopySession(findSession)
	old.Sign = -1
	updated := s.updateSession(findSession, event)
	updated.Sign = 1
	s.writeBuffer.Add(old)
	s.writeBuffer.Add(updated)
	s.updateSessionCache(updated)
	return updated, nil
}

func sessionContainsEvent(session *models.Sessions, timestamp time.Time) bool {
	return session != nil && !timestamp.Before(session.Start) && !timestamp.After(session.Timestamp.Add(sessionCacheTTL))
}

func (s *SessionManager) newSession(event *models.Events) *models.Sessions {

	hostName := ""
	entryPage := event.PathName // 始终设置，避免非 pageview 事件导致空白
	exitPage := event.PathName
	pageviews := int32(0)
	if event.Name == "pageview" {
		hostName = event.HostName
		pageviews += 1
	}
	isBounce := uint8(0)
	if event.Name == "pageview" || !event.Interactive {
		isBounce = uint8(1)
	}
	// Layout: bits[63:24]=timestamp(40b) | bits[23:16]=machineID(8b) | bits[15:0]=random(16b)
	// Mask machineID to 8 bits; without the mask a full-range uint64 would corrupt the timestamp bits.
	sessionId := (uint64(time.Now().UnixNano()) << 24) | ((s.machineID & 0xFF) << 16) | uint64(rand.Uint32()&0xFFFF)

	session := &models.Sessions{
		Version:        s.nextVersion(),
		Sign:           1,
		Duration:       0,
		PageViews:      pageviews,
		Events:         1,
		SessionId:      sessionId,
		SiteId:         event.SiteId,
		UserId:         event.UserId,
		Start:          event.Timestamp,
		Timestamp:      event.Timestamp,
		IP:             event.IP,
		IPv6:           event.IPv6,
		HostName:       hostName,
		EntryPage:      entryPage,
		ExitPage:       exitPage,
		PathName:       event.PathName,
		URL:            event.URL,
		EntryMetaKey:   event.MetaKey,
		EntryMetaValue: event.MetaValue,
		IsBounce:       isBounce,
	}

	session.UtmMedium = event.UtmMedium
	session.UtmSource = event.UtmSource
	session.UtmContent = event.UtmContent
	session.UtmTerm = event.UtmTerm
	session.UtmCampaign = event.UtmCampaign
	session.Channel = event.Channel
	session.ScreenSize = event.ScreenSize
	session.OperatingSystem = event.OperatingSystem
	session.OperatingSystemVersion = event.OperatingSystemVersion
	session.Browser = event.Browser
	session.BrowserVersion = event.BrowserVersion
	session.CityGeonameId = event.CityGeonameId
	session.CountryCode = event.CountryCode
	session.ContinentGeonameId = event.ContinentGeonameId
	session.Coordinates = event.Coordinates
	session.Referrer = event.Referrer
	session.ReferrerSource = event.ReferrerSource

	return session
}

func (s *SessionManager) updateSession(session *models.Sessions, event *models.Events) *models.Sessions {
	newSession := s.CopySession(session)
	// 新状态行赋新版本号，Sign 由调用方设置为 +1
	newSession.Version = s.nextVersion()

	slog.Debug("update session", "newSession", newSession.Duration, "session", session.Duration)

	pageview := event.Name == "pageview"
	var pageviews int32
	if pageview {
		pageviews = session.PageViews + 1
	} else {
		pageviews = session.PageViews
	}
	newSession.PageViews = pageviews

	if event.Timestamp.After(newSession.Timestamp) {
		newSession.Timestamp = event.Timestamp
	}
	if session.EntryPage == "" && pageview {
		newSession.EntryPage = event.PathName
	}

	if pageview && !event.Timestamp.Before(session.Timestamp) {
		newSession.ExitPage = event.PathName
	}

	if session.HostName == "" && pageview {
		newSession.HostName = event.HostName
	}
	if session.IsBounce == 1 {
		if pageviews > 1 || (event.Interactive && !pageview && event.Name != "engagement") {
			newSession.IsBounce = 0
		}
	}

	newSession.Duration = durationSeconds(session.Start, newSession.Timestamp)

	newSession.Events += 1

	slog.Debug("update session", "newSession", newSession.Duration, "session", session.Duration)

	return newSession
}

// durationSeconds 计算会话时长（秒），对乱序到达（end 早于 start）的事件钳制为 0，
// 避免 uint32 下溢产生约 42 亿秒的异常时长污染 avg(duration) 聚合。
func durationSeconds(start, end time.Time) uint32 {
	d := end.Sub(start).Seconds()
	if d < 0 {
		return 0
	}
	return uint32(d)
}

func (s *SessionManager) updateSessionCache(session *models.Sessions) *models.Sessions {
	key := s.generateSessionCacheKey(session.UserId, session.SiteId)
	// A late event for an earlier session must not evict the current session.
	if current, ok := s.cache.Get(key); !ok || current.SessionId == session.SessionId || !session.Timestamp.Before(current.Timestamp) {
		s.cache.Add(key, session)
	}
	return session
}

func (s *SessionManager) generateSessionCacheKey(userId, siteId uint64) string {
	return fmt.Sprintf("session:%d:%d", userId, siteId)
}

// loadSessionFromDB 在缓存 miss 时恢复 session 的最新状态。
//
// 设计：先查 writeBuffer 中尚未落库（含正在写入 CH 途中）的行，再回读 ClickHouse。
// 原因：session 的每次更新都是"tombstone(-1, 旧 version) + 新状态(+1, 新 version)"成对写入
// writeBuffer，最多延迟 flushInterval（5s）+ 重试时间才到达 CH。如果缓存刚好在这段窗口内
// 淘汰了该 session，直接读 CH 会拿到旧版本，再基于旧版本生成新行，导致：
//   - 旧版本被 tombstone 两次（多出一条 -1 行），
//   - 新旧两条 +1 状态行并存且永远无法 collapse。
//
// writeBuffer 中的行是"最新事实"，优先级高于 CH；只有 buffer 中没有该用户的行时才查 CH。
// 同一 userId 的事件由 balancer 串行处理，因此不存在并发写同一 key 的竞态。
func (s *SessionManager) loadSessionFromDB(event *models.Events) (*models.Sessions, error) {
	if pending := s.writeBuffer.FindPendingAt(event.UserId, event.SiteId, event.Timestamp); pending != nil {
		// 返回副本，避免调用方后续修改影响 buffer 中即将写入的行
		return s.CopySession(pending), nil
	}

	repo := repository.GetSessionsRepository()
	if repo == nil {
		return nil, fmt.Errorf("sessions repository not initialized")
	}
	ctx, cancel := context.WithTimeout(s.shutdownCtx, dbLookupTimeout)
	defer cancel()
	return repo.GetActiveSessionAt(ctx, event.UserId, event.SiteId, event.Timestamp, sessionCacheTTL)
}

func (s *SessionManager) WriteSession(session *models.Sessions) {
	s.writeBuffer.Add(session)
}

func (s *SessionManager) Shutdown() {
	slog.Info("Shutdown session manager")
	// Flush remaining sessions before cancelling the context.
	// Cancelling first would make wb.ctx done, causing writeBatch to fail mid-flush.
	s.writeBuffer.Shutdown()
	s.shutdownCancel()
}

// nextVersion returns the next monotonically increasing version number
func (s *SessionManager) nextVersion() uint64 {
	return atomic.AddUint64(&s.versionCounter, 1)
}

// CopySession 忠实复制一个 session，保留原始 version。
// 用于生成 Sign=-1 抵消行（tombstone）：抵消行必须与它要抵消的 Sign=+1 行
// 具有相同的 version，VersionedCollapsingMergeTree 才能正确 collapse 这一对。
// 若需要生成"新状态"行，调用方应在复制后显式调用 nextVersion() 赋新版本号。
func (s *SessionManager) CopySession(session *models.Sessions) *models.Sessions {
	if session == nil {
		return nil
	}

	copied := &models.Sessions{
		Version:                session.Version,
		Sign:                   session.Sign,
		Duration:               session.Duration,
		PageViews:              session.PageViews,
		Events:                 session.Events,
		SessionId:              session.SessionId,
		SiteId:                 session.SiteId,
		UserId:                 session.UserId,
		Start:                  session.Start,
		Timestamp:              session.Timestamp,
		IP:                     session.IP,
		IPv6:                   session.IPv6,
		HostName:               session.HostName,
		EntryPage:              session.EntryPage,
		ExitPage:               session.ExitPage,
		PathName:               session.PathName,
		URL:                    session.URL,
		EntryMetaKey:           session.EntryMetaKey,
		EntryMetaValue:         session.EntryMetaValue,
		IsBounce:               session.IsBounce,
		UtmMedium:              session.UtmMedium,
		UtmSource:              session.UtmSource,
		UtmContent:             session.UtmContent,
		UtmTerm:                session.UtmTerm,
		UtmCampaign:            session.UtmCampaign,
		Channel:                session.Channel,
		ScreenSize:             session.ScreenSize,
		OperatingSystem:        session.OperatingSystem,
		OperatingSystemVersion: session.OperatingSystemVersion,
		Browser:                session.Browser,
		BrowserVersion:         session.BrowserVersion,
		CityGeonameId:          session.CityGeonameId,
		CountryCode:            session.CountryCode,
		ContinentGeonameId:     session.ContinentGeonameId,
		Coordinates:            session.Coordinates,
		Referrer:               session.Referrer,
		ReferrerSource:         session.ReferrerSource,
	}

	return copied
}
