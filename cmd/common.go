package cmd

import (
	"log/slog"

	"github.com/zenstats/zenstats/config"
	"github.com/zenstats/zenstats/internal/bootstrap"
	"github.com/zenstats/zenstats/pkg/globals"
)

func Init() {
	if config.Conf.MaxmindLicenseKey == "" {
		slog.Warn("maxmind_license_key is not configured, will use Loyalsoldier/geoip as fallback GeoIP source")
	}

	bootstrap.InitLog()
	bootstrap.InitWorkQueue()
	bootstrap.InitClickhouseTable()
	bootstrap.InitGeoIP()
	bootstrap.InitPostgres()
	bootstrap.InitSystemConfig()
}

func InitServer() {
	Init()
	bootstrap.InitCron()
}

func Release() {
	globals.GetDB().Client.Close()
}
