package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

// 日志轮转的默认参数。长期运行的机器人写 INFO 日志会无限增长，
// 轮转不是可选项。
const (
	defaultMaxSizeMB  = 50
	defaultMaxBackups = 5
	defaultMaxAgeDays = 30
)

// SystemOptions 配置系统日志。
//
// 系统日志走 stderr 与文件而非数据库：数据库连不上时，
// 「数据库连不上」这条日志本身还得写得出来。
type SystemOptions struct {
	Level      string // debug / info / warn / error，空则用 info
	File       string // 空则只写 stderr
	MaxSizeMB  int
	MaxBackups int
	MaxAgeDays int
	JSON       bool // 写 JSON 而非文本
}

// SystemOptionsFromEnv 从环境变量读配置。
func SystemOptionsFromEnv() SystemOptions {
	o := SystemOptions{
		Level:      os.Getenv("MAGICD_LOG_LEVEL"),
		File:       os.Getenv("MAGICD_LOG_FILE"),
		MaxSizeMB:  defaultMaxSizeMB,
		MaxBackups: defaultMaxBackups,
		MaxAgeDays: defaultMaxAgeDays,
	}
	if o.Level == "" {
		o.Level = "info"
	}
	return o
}

// noopCloser 是无文件时返回的空关闭器。
type noopCloser struct{}

func (noopCloser) Close() error { return nil }

// SetupSystem 装配系统日志并设为 slog 的默认 Logger。
//
// 返回的 Closer 用于关闭日志文件；无文件时关闭是空操作。
func SetupSystem(opts SystemOptions) (io.Closer, error) {
	level, err := parseLevel(opts.Level)
	if err != nil {
		return nil, err
	}

	// stderr 永远写：容器里 docker logs 看的就是它
	writers := []io.Writer{os.Stderr}
	closer := io.Closer(noopCloser{})

	if opts.File != "" {
		if dir := filepath.Dir(opts.File); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, fmt.Errorf("logging: 创建日志目录 %s 失败: %w", dir, err)
			}
		}
		lj := &lumberjack.Logger{
			Filename:   opts.File,
			MaxSize:    orDefault(opts.MaxSizeMB, defaultMaxSizeMB),
			MaxBackups: orDefault(opts.MaxBackups, defaultMaxBackups),
			MaxAge:     orDefault(opts.MaxAgeDays, defaultMaxAgeDays),
			Compress:   true,
		}
		writers = append(writers, lj)
		closer = lj
	}

	out := io.MultiWriter(writers...)
	handlerOpts := &slog.HandlerOptions{Level: level}

	var h slog.Handler
	if opts.JSON {
		h = slog.NewJSONHandler(out, handlerOpts)
	} else {
		h = slog.NewTextHandler(out, handlerOpts)
	}
	slog.SetDefault(slog.New(h))
	return closer, nil
}

// parseLevel 把级别名转成 slog.Level。
func parseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("logging: 未知的日志级别 %q，合法值为 debug, info, warn, error", s)
	}
}

// orDefault 在值非正时返回默认值。
func orDefault(v, def int) int {
	if v <= 0 {
		return def
	}
	return v
}
