package bootstrap

import (
	"github.com/zenstats/zenstats/pkg/geoip"
	"github.com/zenstats/zenstats/pkg/scheduler"
)

func InitCron() {
	cron := scheduler.GetCronManager()
	cron.Start()

	// 每天1点更新 GeoIP 数据库
	cron.AddJob("0 0 1 * * *", func(params interface{}) {
		geoip.GetGeoIP().UpdateGeoIPDB("")
	}, nil)
}
