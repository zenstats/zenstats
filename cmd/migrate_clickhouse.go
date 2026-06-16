package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zenstats/zenstats/internal/store/clickhouse"
)

// MigrateClickHouseCmd 在线迁移 ClickHouse schema（仅 ALTER TABLE，不重建表）。
// 用于升级已有部署，所有变更幂等可重复执行。
var MigrateClickHouseCmd = &cobra.Command{
	Use:   "migrate-clickhouse",
	Short: "Apply online ClickHouse schema migrations (idempotent ALTER TABLE)",
	Long: `对已有的 ClickHouse 表执行在线 schema 变更，包括：
  - 列重命名 (device → screen_size)
  - 新增列 (version)
  - 兼容别名 (device ALIAS screen_size, entry_page_hostname ALIAS hostname)
  - 类型变更 (user_agent: LowCardinality → String CODEC)

所有 ALTER TABLE 语句均为幂等设计，可安全重复执行。

Docker 环境使用：
  docker compose exec zenstats /app/zenstats migrate-clickhouse

手动部署使用：
  ./bin/zenstats migrate-clickhouse

注意：sessions 表引擎从 events 版本列切换到 version 列需手动重建表。
`,
	Run: func(cmd *cobra.Command, args []string) {
		m := clickhouse.NewMigration()
		if err := m.RunOnline(); err != nil {
			fmt.Printf("ClickHouse online migration failed: %v\n", err)
			return
		}
		fmt.Println("ClickHouse online migration completed.")
	},
}

func init() {
	RootCmd.AddCommand(MigrateClickHouseCmd)
}
