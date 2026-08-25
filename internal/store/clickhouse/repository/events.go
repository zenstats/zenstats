package repository

import (
	"context"
	"fmt"
	"log/slog"
	"net"

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
		ipv6, channel, revenue, revenue_currency
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
			ipv4Column(event.IP),
			event.CountryCode,
			event.ContinentGeonameId,
			event.CityGeonameId,
			coords,
			ipv6Column(event.IP, event.IPv6),
			event.Channel,
			event.Revenue,
			event.RevenueCurrency,
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

// ipv4Column 返回可安全写入 ClickHouse IPv4 列的值：IPv4 / IPv4-mapped 地址返回 4 字节形式，
// 纯 IPv6 或空返回 nil（列写 0.0.0.0）。clickhouse-go 对 16 字节 net.IP 会直接取末 4 字节，
// 把 IPv6 地址写成无意义的 IPv4，故必须在此拦截。
func ipv4Column(ip net.IP) net.IP {
	if ip == nil {
		return nil
	}
	return ip.To4()
}

// ipv6Column 返回写入 IPv6 列的值：优先显式的 ipv6 字段；否则若 ip 为纯 IPv6 地址则使用它；
// 都不满足返回 nil（列写 ::）。
func ipv6Column(ip, ipv6 net.IP) net.IP {
	if ipv6 != nil {
		return ipv6
	}
	if ip != nil && ip.To4() == nil && len(ip) == net.IPv6len {
		return ip
	}
	return nil
}
