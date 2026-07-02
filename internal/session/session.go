package session

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"sync/atomic"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/zenstats/zenstats/internal/store/clickhouse/models"
	"github.com/zenstats/zenstats/internal/store/clickhouse/repository"
)

type SessionManager struct {
	machineID      uint64
	balancer       *Balancer[*models.Sessions]
	writeBuffer    *WriteBuffer
	shutdownCtx    context.Context
	shutdownCancel context.CancelFunc
	versionCounter uint64 // 单调递增的版本号计数器

	cache *expirable.LRU[string, *models.Sessions]
}

func NewSessionManager(ctx context.Context, batchSize int) *SessionManager {
	ctx, cancel := context.WithCancel(ctx)
	l := expirable.NewLRU[string, *models.Sessions](1000, nil, 30*time.Minute)

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
	if event.Name == "engagement" {
		if findSession == nil {
			// 从 ClickHouse 加载最近的活跃 session
			loadedSession, err := s.loadSessionFromDB(event.UserId, event.SiteId)
			if err != nil {
				slog.Debug("session not found in cache or DB", "user_id", event.UserId, "site_id", event.SiteId)
				return nil, nil
			}
			// Write Sign=-1 tombstone for the existing DB row before mutating.
			// tombstone 保留 loadedSession 的 version，与 DB 中的 Sign=+1 行配对 collapse。
			tombstone := s.CopySession(loadedSession)
			tombstone.Sign = -1
			s.writeBuffer.Add(tombstone)

			// 生成新状态行：赋新版本号（必须高于 tombstone），Sign=+1
			newState := s.CopySession(loadedSession)
			newState.Version = s.nextVersion()
			newState.Sign = 1
			newState.Duration = durationSeconds(newState.Start, event.Timestamp)
			newState.Timestamp = event.Timestamp
			newState.Events += 1
			s.writeBuffer.Add(newState)
			s.updateSessionCache(newState)
			return newState, nil
		}
		oldSession := s.CopySession(findSession)
		oldSession.Sign = -1
		s.writeBuffer.Add(oldSession)

		newSession := s.CopySession(findSession)
		newSession.Version = s.nextVersion()
		newSession.Sign = 1
		newSession.Duration = durationSeconds(newSession.Start, event.Timestamp)
		newSession.Timestamp = event.Timestamp
		newSession.Events += 1
		s.writeBuffer.Add(newSession)
		s.updateSessionCache(newSession)
		return newSession, nil
	}

	// pageview / 自定义事件：缓存未命中时从 ClickHouse 加载
	if findSession == nil {
		loadedSession, err := s.loadSessionFromDB(event.UserId, event.SiteId)
		if err == nil && loadedSession != nil {
			findSession = loadedSession
		}
	}

	// if session exists and event is not engagement
	// update session
	if findSession != nil {
		// copy old session
		oldSession := s.CopySession(findSession)
		oldSession.Sign = -1
		s.writeBuffer.Add(oldSession)
		// update session
		updateSession := s.updateSession(findSession, event)
		updateSession.Sign = 1
		s.writeBuffer.Add(updateSession)

		s.updateSessionCache(updateSession)

		return updateSession, nil
	}

	// if session does not exist
	// new session
	session := s.newSession(event)
	s.writeBuffer.Add(session)
	s.updateSessionCache(session)

	return session, nil

}

func (s *SessionManager) newSession(event *models.Events) *models.Sessions {

	hostName := ""
	entryPage := event.PathName  // 始终设置，避免非 pageview 事件导致空白
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

	newSession.Timestamp = event.Timestamp
	if session.EntryPage == "" && pageview {
		newSession.EntryPage = event.PathName
	}

	if pageview {
		newSession.ExitPage = event.PathName
	}

	if session.HostName == "" && pageview {
		newSession.HostName = event.HostName
	}
	if session.IsBounce == 1 {
		if pageviews > 1 || (event.Interactive && !pageview) {
			newSession.IsBounce = 0
		}
	}

	newSession.Duration = durationSeconds(session.Start, event.Timestamp)

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

func (s *SessionManager) refreshSessionCache(session *models.Sessions, timestamp time.Time) *models.Sessions {
	session.Timestamp = timestamp

	return s.updateSessionCache(session)
}

func (s *SessionManager) updateSessionCache(session *models.Sessions) *models.Sessions {

	s.cache.Add(s.generateSessionCacheKey(session.UserId, session.SiteId), session)

	return session
}

func (s *SessionManager) generateSessionCacheKey(userId, siteId uint64) string {
	return fmt.Sprintf("session:%d:%d", userId, siteId)
}

func (s *SessionManager) loadSessionFromDB(userId, siteId uint64) (*models.Sessions, error) {
	repo := repository.GetSessionsRepository()
	if repo == nil {
		return nil, fmt.Errorf("sessions repository not initialized")
	}
	return repo.GetMostRecentActiveSession(context.Background(), userId, siteId)
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
