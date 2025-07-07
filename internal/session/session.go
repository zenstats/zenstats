package session

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/zenstats/zenstats/internal/store/clickhouse/models"
)

type SessionManager struct {
	machineID      uint64
	balancer       *Balancer[*models.Sessions]
	writeBuffer    *WriteBuffer
	shutdownCtx    context.Context
	shutdownCancel context.CancelFunc

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
			return nil, fmt.Errorf("session not found")
		}
		s.refreshSessionCache(findSession, event.Timestamp)
		return nil, nil
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

		slog.Debug("update session", "oldSession", oldSession.Timestamp, "updateSession", updateSession.Timestamp)
		slog.Debug("save session", "findSession", findSession.Timestamp, "oldSession", oldSession.Timestamp)

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
	entryPage := ""
	exitPage := ""
	pageviews := int32(0)
	if event.Name == "pageview" {
		hostName = event.HostName
		entryPage = event.PathName
		exitPage = event.PathName
		pageviews += 1
	}
	isBounce := uint8(0)
	if event.Name == "pageview" || !event.Interactive {
		isBounce = uint8(1)
	}
	sessionId := (uint64(time.Now().UnixNano()) << 24) | (s.machineID << 16) | uint64(rand.Uint32()&0xFFFF)

	session := &models.Sessions{
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
		if session.PageViews > 1 || (event.Interactive && !pageview) {
			newSession.IsBounce = 0
		}
	}

	newSession.Duration = uint32(time.Since(session.Start).Seconds())

	newSession.Events += 1

	slog.Debug("update session", "newSession", newSession.Duration, "session", session.Duration)

	return newSession
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

func (s *SessionManager) Shutdown() {
	slog.Info("Shutdown session manager")
	s.shutdownCancel()

	s.writeBuffer.Shutdown()
}

func (s *SessionManager) CopySession(session *models.Sessions) *models.Sessions {
	if session == nil {
		return nil
	}

	copied := &models.Sessions{
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
