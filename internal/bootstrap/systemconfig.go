package bootstrap

import (
	"context"
	"log/slog"

	"github.com/zenstats/zenstats/internal/service"
)

func InitSystemConfig() {
	ctx := context.Background()
	configService := service.GetSystemConfigService()

	// 初始化默认配置
	configService.InitDefaults(ctx)

	// 从数据库加载配置到内存
	if err := configService.LoadConfigsFromDB(ctx); err != nil {
		slog.Warn("failed to load system configs from database", "error", err)
	} else {
		slog.Info("system configs loaded from database")
	}
}
