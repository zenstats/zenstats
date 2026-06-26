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
	eventsInstance *Events
)

type Events struct {
	conn clickhouse.Conn
}

func GetEventsRepository() *Events {
	eventOnce.Do(func() {
		conn := cl.GetConnection()
		if conn == nil {
			return
		}
		eventsInstance = &Events{conn: conn}
	})
	return eventsInstance
}

func (e *Events) BatchInsert(ctx context.Context, events []*models.Events) error {
	batchInsert, err := e.conn.PrepareBatch(ctx, `INSERT INTO events (
		timestamp, name, site_id, user_id, session_id,
		url, hostname, pathname, referrer, referrer_source,
		operating_system, screen_size,
		utm_medium, utm_source, utm_content, utm_term, utm_campaign,
		"meta.key", "meta.value",
		browser, browser_version, user_agent, operating_system_version,
		engagement_time, scroll_depth,
		ipv4, country_code, continent_geoname_id, city_geoname_id, coordinates,
		ipv6, channel
	)`)
	if err != nil {
		slog.Error("prepare batch", "error", err)
		return fmt.Errorf("prepare batch: %w", err)
	}
	skipped := 0
	for _, event := range events {
		slog.Debug("insert event", "event", event)
		coords := []any{event.Coordinates.Latitude, event.Coordinates.Longitude}

		err = batchInsert.Append(
			event.Timestamp,
			event.Name,
			event.SiteId,
			event.UserId,
			event.SessionId,
			event.URL,
			event.HostName,
			event.PathName,
			event.Referrer,
			event.ReferrerSource,
			event.OperatingSystem,
			event.ScreenSize,
			event.UtmMedium,
			event.UtmSource,
			event.UtmContent,
			event.UtmTerm,
			event.UtmCampaign,
			event.MetaKey,
			event.MetaValue,
			event.Browser,
			event.BrowserVersion,
			event.UserAgent,
			event.OperatingSystemVersion,
			event.EngagementTime,
			event.ScrollDepth,
			event.IP,
			event.CountryCode,
			event.ContinentGeonameId,
			event.CityGeonameId,
			coords,
			event.IPv6,
			event.Channel,
		)

		if err != nil {
			slog.Error("skip malformed event on append", "err", err)
			skipped++
			continue
		}
	}

	if skipped > 0 {
		slog.Warn("events batch skipped malformed rows", "skipped", skipped, "total", len(events))
	}
	return batchInsert.Send()
}
