package bootstrap

import (
	"context"
	"log/slog"

	"github.com/zenstats/zenstats/internal/service"
)

func InitSystemConfig() {
	ctx := context.Background()
	configService := service.GetSystemConfigService()

	// Initialize default config
	configService.InitDefaults(ctx)

	// Load config from database into memory
	if err := configService.LoadConfigsFromDB(ctx); err != nil {
		slog.Warn("failed to load system configs from database", "error", err)
	} else {
		slog.Info("system configs loaded from database")
	}
}
