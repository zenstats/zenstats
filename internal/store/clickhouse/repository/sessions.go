package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

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
	batchInsert, err := s.conn.PrepareBatch(ctx, `INSERT INTO sessions (
		"start", timestamp, session_id, version, sign, is_bounce,
		entry_page, exit_page, pageviews, events, duration,
		site_id, user_id, url, hostname, pathname,
		referrer, referrer_source, operating_system, screen_size,
		utm_medium, utm_source, utm_content, utm_term, utm_campaign,
		"entry_meta.key", "entry_meta.value",
		browser, browser_version, user_agent, operating_system_version,
		ipv4, country_code, continent_geoname_id, city_geoname_id, coordinates,
		ipv6, channel
	)`)
	if err != nil {
		slog.Error("prepare batch", "error", err)
		return fmt.Errorf("prepare batch: %w", err)
	}
	skipped := 0
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
			session.ScreenSize,
			session.UtmMedium,
			session.UtmSource,
			session.UtmContent,
			session.UtmTerm,
			session.UtmCampaign,
			session.EntryMetaKey,
			session.EntryMetaValue,
			session.Browser,
			session.BrowserVersion,
			session.UserAgent,
			session.OperatingSystemVersion,

			ipv4Column(session.IP),
			session.CountryCode,
			session.ContinentGeonameId,
			session.CityGeonameId,
			coordinates,

			ipv6Column(session.IP, session.IPv6),
			session.Channel,
		)

		if err != nil {
			slog.Error("skip malformed session on append", "err", err)
			skipped++
			continue
		}
	}

	if skipped > 0 {
		slog.Warn("sessions batch skipped malformed rows", "skipped", skipped, "total", len(sessions))
	}
	return batchInsert.Send()
}

// ErrSessionNotFound distinguishes an absent session from a failed lookup.
var ErrSessionNotFound = errors.New("session not found")

// GetActiveSessionAt restores the latest collapsed state containing the event's
// timestamp, including late events for an earlier session. Query bounds follow
// event time rather than wall time.
func (s *Sessions) GetActiveSessionAt(ctx context.Context, userId, siteId uint64, timestamp time.Time, inactivity time.Duration) (*models.Sessions, error) {
	rows, err := s.conn.Query(ctx,
		`SELECT start, timestamp, session_id, version, sign, is_bounce,
				entry_page, exit_page, pageviews, events, duration,
				site_id, user_id, url, hostname, pathname,
				referrer, referrer_source, operating_system, screen_size,
				utm_medium, utm_source, utm_content, utm_term, utm_campaign,
				entry_meta.key, entry_meta.value,
				browser, browser_version, user_agent, operating_system_version,
				ipv4, country_code, continent_geoname_id, city_geoname_id, coordinates,
				ipv6, channel
		 FROM sessions FINAL
		 WHERE site_id = ? AND user_id = ? AND start >= ? AND start <= ?
		   AND timestamp >= ? AND sign = 1
		 ORDER BY start DESC, version DESC
		 LIMIT 1`,
		siteId, userId, timestamp.AddDate(0, 0, -7), timestamp, timestamp.Add(-inactivity),
	)
	if err != nil {
		return nil, fmt.Errorf("query session: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("query session rows: %w", err)
		}
		return nil, ErrSessionNotFound
	}

	var session models.Sessions
	coordinates := []float64{0, 0}

	err = rows.Scan(
		&session.Start,
		&session.Timestamp,
		&session.SessionId,
		&session.Version,
		&session.Sign,
		&session.IsBounce,
		&session.EntryPage,
		&session.ExitPage,
		&session.PageViews,
		&session.Events,
		&session.Duration,
		&session.SiteId,
		&session.UserId,
		&session.URL,
		&session.HostName,
		&session.PathName,
		&session.Referrer,
		&session.ReferrerSource,
		&session.OperatingSystem,
		&session.ScreenSize,
		&session.UtmMedium,
		&session.UtmSource,
		&session.UtmContent,
		&session.UtmTerm,
		&session.UtmCampaign,
		&session.EntryMetaKey,
		&session.EntryMetaValue,
		&session.Browser,
		&session.BrowserVersion,
		&session.UserAgent,
		&session.OperatingSystemVersion,
		&session.IP,
		&session.CountryCode,
		&session.ContinentGeonameId,
		&session.CityGeonameId,
		&coordinates,
		&session.IPv6,
		&session.Channel,
	)
	if err != nil {
		return nil, fmt.Errorf("scan session: %w", err)
	}

	session.Coordinates.Latitude = coordinates[0]
	session.Coordinates.Longitude = coordinates[1]

	return &session, nil
}
