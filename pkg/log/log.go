package log

import (
	"log/slog"

	"github.com/zenstats/zenstats/config"
	"gopkg.in/natefinch/lumberjack.v2"
)

func Init() {
	lumberjackLogger := &lumberjack.Logger{
		Filename:   "./log/zenstats.log", // 日志文件路径
		MaxSize:    50,                   // 单个日志文件最大大小（单位：MB）
		MaxBackups: 10,                   // 保留的旧日志文件个数
		MaxAge:     30,                   // 保留的旧日志文件最大天数（单位：天）
		Compress:   true,                 // 是否压缩旧日志文件
	}
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
	handler := slog.NewJSONHandler(lumberjackLogger, &slog.HandlerOptions{
		// AddSource: slogLevel == slog.LevelDebug, // 如果设置为 true，日志输出中将包含源代码的位置（文件名和行号）
		AddSource: false,
		Level:     slogLevel,
	})

	// 创建自定义 Logger
	logger := slog.New(handler)

	// 替换默认日志记录器
	slog.SetDefault(logger)
}
