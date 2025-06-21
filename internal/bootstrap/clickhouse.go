package bootstrap

import "github.com/zenstats/zenstats/internal/store/clickhouse"

func InitClickhouseTable() {
	migration := clickhouse.NewMigration()
	err := migration.Run()
	if err != nil {
		panic(err)
	}
}
