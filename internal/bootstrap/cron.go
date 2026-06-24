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

	// 每7天更新 GeoIP 数据库
	cron.AddJob("0 0 1 */7 * *", func(params any) {
		geoip.GetGeoIP().UpdateGeoIPDB("")
	}, nil)

	// 每周一凌晨 3:00 更新垃圾 referrer 列表
	cron.AddJob("0 0 3 * * 1", func(params any) {
		ctx := context.Background()
		if err := event.RefreshSpamBlocklist(ctx); err != nil {
			slog.Error("failed to refresh spam blocklist", "error", err)
		}
	}, nil)

	// 注册邮件报告定时任务（周报 + 月报）
	service.RegisterReportCron()

	// 注册流量异常检测定时任务（每小时）
	service.RegisterTrafficAlertCron()
}
