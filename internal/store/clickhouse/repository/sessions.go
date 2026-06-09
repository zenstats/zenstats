package repository

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ClickHouse/clickhouse-go/v2"
	cl "github.com/zenstats/zenstats/internal/store/clickhouse"
	"github.com/zenstats/zenstats/internal/store/clickhouse/models"
)

var (
	sessionsInstance *Sessions
)

type Sessions struct {
	conn clickhouse.Conn
}

func GetSessionsRepository() *Sessions {
	sessionOnce.Do(func() {
		conn := cl.GetConnection()
		if conn == nil {
			return
		}
		sessionsInstance = &Sessions{conn: conn}
	})
	return sessionsInstance
}

func (s *Sessions) BatchInsert(ctx context.Context, sessions []*models.Sessions) error {
	batchInsert, err := s.conn.PrepareBatch(ctx, "INSERT INTO sessions")
	if err != nil {
		slog.Error("prepare batch", "error", err)
		return fmt.Errorf("prepare batch: %w", err)
	}
	for _, session := range sessions {
		slog.Debug("insert session", "session", session)
		coordinates := []float64{session.Coordinates.Latitude, session.Coordinates.Longitude}

		err = batchInsert.Append(
			session.Start,
			session.Timestamp,
			session.SessionId,
			session.Version,
			session.Sign,
			session.IsBounce,
			session.EntryPage,
			session.ExitPage,
			session.PageViews,
			session.Events,
			session.Duration,

			session.SiteId,
			session.UserId,
			session.URL,
			session.HostName,
			session.PathName,
			session.Referrer,
			session.ReferrerSource,
			session.OperatingSystem,
			session.UtmMedium,
			session.UtmSource,
			session.UtmContent,
			session.UtmTerm,
			session.UtmCampaign,
			session.EntryMetaKey,
			session.EntryMetaValue,
			session.ScreenSize,
			session.Browser,
			session.BrowserVersion,
			session.UserAgent,
			session.OperatingSystemVersion,

			session.IP,
			session.CountryCode,
			session.ContinentGeonameId,
			session.CityGeonameId,
			coordinates,

			session.IPv6,
			session.Channel,
		)

		if err != nil {
			slog.Error("Failed to append", "session", session, "err", err)
		}
	}

	return batchInsert.Send()
}
