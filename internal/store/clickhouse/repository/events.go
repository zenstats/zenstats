package repository

import (
	"context"
	"log/slog"
	"os"

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
	batchInsert, err := e.conn.PrepareBatch(context.Background(), "INSERT INTO events")
	if err != nil {
		slog.Error("prepare batch", "error", err)
		os.Exit(1)
	}
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
			slog.Error("Failed to append", "event", event, "err", err)
		}
	}

	return batchInsert.Send()
}
