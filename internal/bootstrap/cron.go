package bootstrap

import (
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
}
