package bootstrap

import (
	"context"
	"log/slog"

	"github.com/zenstats/zenstats/internal/event"
	"github.com/zenstats/zenstats/internal/service"
	"github.com/zenstats/zenstats/pkg/geoip"
	"github.com/zenstats/zenstats/pkg/scheduler"
)

func InitCron() {
	cron := scheduler.GetCronManager()
	cron.Start()

	// Update GeoIP database every 7 days
	cron.AddJob("0 0 1 */7 * *", func(params any) {
		geoip.GetGeoIP().UpdateGeoIPDB("")
	}, nil)

	// Update spam referrer list every Monday at 03:00
	cron.AddJob("0 0 3 * * 1", func(params any) {
		ctx := context.Background()
		if err := event.RefreshSpamBlocklist(ctx); err != nil {
			slog.Error("failed to refresh spam blocklist", "error", err)
		}
	}, nil)

	// Register email report cron jobs (weekly + monthly)
	service.RegisterReportCron()

	// Register traffic anomaly detection cron job (hourly)
	service.RegisterTrafficAlertCron()
}
