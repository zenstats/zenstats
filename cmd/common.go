package cmd

import (
	"github.com/zenstats/zenstats/internal/bootstrap"
)

func Init() {
	bootstrap.InitLog()
	bootstrap.InitWorkQueue()
	bootstrap.InitCron()
	bootstrap.InitClickhouseTable()

	bootstrap.InitGeoIP()
}

func Release() {
	// db.Close()
}
