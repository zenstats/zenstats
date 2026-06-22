package log

import (
	"io"
	"log/slog"
	"os"

	"github.com/zenstats/zenstats/config"
	"gopkg.in/natefinch/lumberjack.v2"
)

func Init() {
	level := config.Conf.LogLevel
	var slogLevel slog.Level
	switch level {
	case "debug":
		slogLevel = slog.LevelDebug
	case "info":
		slogLevel = slog.LevelInfo
	case "warn":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}

	var writer io.Writer

	// Docker: log to stdout (Docker json-file driver handles rotation).
	// Bare-metal: log to file via lumberjack (auto rotation).
	if os.Getenv("ZENSTATS_LOG_OUTPUT") == "stdout" {
		writer = os.Stdout
	} else {
		if err := os.MkdirAll("./log", 0755); err != nil {
			slog.Error("failed to create log directory", "error", err)
		}
		writer = &lumberjack.Logger{
			Filename:   "./log/zenstats.log",
			MaxSize:    50,   // MB
			MaxBackups: 10,
			MaxAge:     30,   // days
			Compress:   true,
		}
	}

	handler := slog.NewJSONHandler(writer, &slog.HandlerOptions{
		AddSource: false,
		Level:     slogLevel,
	})

	slog.SetDefault(slog.New(handler))
}
