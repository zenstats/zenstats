package cmd

import (
	"github.com/zenstats/zenstats/config"
	"github.com/zenstats/zenstats/internal/bootstrap"
	"github.com/zenstats/zenstats/pkg/globals"
)

func Init() {
	// 判断关键config是否设置
	if config.Conf.MaxmindLicenseKey == "" {
		panic("The maxmind_license_key is required. You can set it via the config.yaml file or by using the environment variable ZENSTATS_MAXMIND_LICENSE_KEY")
	}

	bootstrap.InitLog()
	bootstrap.InitWorkQueue()
	bootstrap.InitClickhouseTable()
	bootstrap.InitGeoIP()
	bootstrap.InitPostgres()
}

func InitServer() {
	Init()
	bootstrap.InitCron()
}

func Release() {
	globals.GetDB().Client.Close()
}
