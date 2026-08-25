package bootstrap

import (
	"context"
	"log/slog"

	"github.com/zenstats/zenstats/internal/event"
	"github.com/zenstats/zenstats/internal/service"
	"github.com/zenstats/zenstats/pkg/geoip"
	"github.com/zenstats/zenstats/pkg/globals"
	"github.com/zenstats/zenstats/pkg/scheduler"
)

func InitCron() {
	cron := scheduler.GetCronManager()

	// 多副本部署时，每个定时任务只允许一个实例执行（PG advisory lock），
	// 否则周报/月报/流量告警邮件会按副本数重复发送。
	if db := globals.GetDB(); db != nil && db.Client != nil {
		scheduler.SetJobLocker(func(ctx context.Context, jobName string) (func(), bool) {
			release, acquired, err := db.TryAdvisoryLock(ctx, jobName)
			if err != nil {
				// 锁服务不可用时退化为直接执行，避免任务在单实例部署上被静默饿死。
				slog.Warn("cron: advisory lock unavailable, running without lock", "job", jobName, "error", err)
				return func() {}, true
			}
			return release, acquired
		})
	}

	cron.Start()

	// Update GeoIP database every 7 days
	cron.AddJob("0 0 1 */7 * *", func(params any) {
		if err := geoip.GetGeoIP().UpdateGeoIPDB(""); err != nil {
			slog.Error("failed to update GeoIP database", "error", err)
		}
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
