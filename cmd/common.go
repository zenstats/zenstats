package cmd

import (
	"github.com/zenstats/zenstats/internal/bootstrap"
	"github.com/zenstats/zenstats/pkg/globals"
)

func Init() {
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
