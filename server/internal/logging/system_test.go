package logging_test

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/logging"
)

func TestSetupSystemWritesToFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "magicd.log")

	closer, err := logging.SetupSystem(logging.SystemOptions{
		Level: "info", File: path, JSON: true,
	})
	if err != nil {
		t.Fatalf("装配系统日志报错: %v", err)
	}

	slog.Info("测试消息", "键", "值")
	if err := closer.Close(); err != nil {
		t.Fatalf("关闭报错: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取日志文件报错: %v", err)
	}
	if !strings.Contains(string(data), "测试消息") {
		t.Errorf("日志文件里没有这条消息: %s", data)
	}
	if !strings.Contains(string(data), `"键":"值"`) {
		t.Errorf("结构化字段未写入: %s", data)
	}
}

func TestSetupSystemWithoutFileStillWorks(t *testing.T) {
	// 不配文件时只写 stderr，这是单机跑的默认形态
	closer, err := logging.SetupSystem(logging.SystemOptions{Level: "info"})
	if err != nil {
		t.Fatalf("无文件时应正常装配: %v", err)
	}
	slog.Info("只写 stderr")
	if err := closer.Close(); err != nil {
		t.Errorf("关闭报错: %v", err)
	}
}

func TestSetupSystemRespectsLevel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "magicd.log")

	closer, err := logging.SetupSystem(logging.SystemOptions{
		Level: "warn", File: path, JSON: true,
	})
	if err != nil {
		t.Fatalf("装配报错: %v", err)
	}

	slog.Debug("调试消息")
	slog.Info("信息消息")
	slog.Warn("警告消息")
	if err := closer.Close(); err != nil {
		t.Fatalf("关闭报错: %v", err)
	}

	data, _ := os.ReadFile(path)
	s := string(data)
	if strings.Contains(s, "调试消息") || strings.Contains(s, "信息消息") {
		t.Errorf("warn 级别不该记录 debug/info: %s", s)
	}
	if !strings.Contains(s, "警告消息") {
		t.Errorf("warn 级别应记录 warn: %s", s)
	}
}

func TestSetupSystemRejectsUnknownLevel(t *testing.T) {
	_, err := logging.SetupSystem(logging.SystemOptions{Level: "详细"})
	if err == nil {
		t.Fatal("未知级别应报错")
	}
	// 报错要列出合法值，否则用户只能翻文档
	if !strings.Contains(err.Error(), "warn") {
		t.Errorf("错误信息应列出合法级别，实际: %v", err)
	}
}

func TestSetupSystemEmptyLevelDefaultsToInfo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "magicd.log")

	closer, err := logging.SetupSystem(logging.SystemOptions{File: path, JSON: true})
	if err != nil {
		t.Fatalf("空级别应默认为 info: %v", err)
	}
	slog.Info("信息消息")
	slog.Debug("调试消息")
	if err := closer.Close(); err != nil {
		t.Fatalf("关闭报错: %v", err)
	}

	data, _ := os.ReadFile(path)
	s := string(data)
	if !strings.Contains(s, "信息消息") {
		t.Errorf("默认应记录 info: %s", s)
	}
	if strings.Contains(s, "调试消息") {
		t.Errorf("默认不该记录 debug: %s", s)
	}
}

func TestSetupSystemCreatesParentDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "logs", "sub", "magicd.log")

	closer, err := logging.SetupSystem(logging.SystemOptions{File: path})
	if err != nil {
		t.Fatalf("应自动建目录: %v", err)
	}
	slog.Info("消息")
	if err := closer.Close(); err != nil {
		t.Fatalf("关闭报错: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("日志文件未创建: %v", err)
	}
}

func TestSystemOptionsFromEnv(t *testing.T) {
	t.Setenv("MAGICD_LOG_LEVEL", "debug")
	t.Setenv("MAGICD_LOG_FILE", "/tmp/x.log")

	o := logging.SystemOptionsFromEnv()
	if o.Level != "debug" {
		t.Errorf("Level = %q", o.Level)
	}
	if o.File != "/tmp/x.log" {
		t.Errorf("File = %q", o.File)
	}
}

func TestSystemOptionsFromEnvDefaults(t *testing.T) {
	t.Setenv("MAGICD_LOG_LEVEL", "")
	t.Setenv("MAGICD_LOG_FILE", "")

	o := logging.SystemOptionsFromEnv()
	if o.Level != "info" {
		t.Errorf("默认级别 = %q, 期望 info", o.Level)
	}
	if o.File != "" {
		t.Errorf("默认不写文件，实际 %q", o.File)
	}
	if o.MaxSizeMB <= 0 || o.MaxBackups <= 0 {
		t.Errorf("轮转参数应有默认值: %+v", o)
	}
}
