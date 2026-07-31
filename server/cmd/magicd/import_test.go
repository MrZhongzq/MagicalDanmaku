package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportRequiresConfigPath(t *testing.T) {
	err := runImport([]string{"-owner", "admin"})
	if err == nil {
		t.Fatal("缺少 -c 应报错")
	}
	if !strings.Contains(err.Error(), "-c") {
		t.Errorf("错误信息应提到 -c，实际: %v", err)
	}
}

func TestImportRequiresOwner(t *testing.T) {
	err := runImport([]string{"-c", "x.yaml"})
	if err == nil {
		t.Fatal("缺少 -owner 应报错")
	}
	if !strings.Contains(err.Error(), "owner") {
		t.Errorf("错误信息应提到 -owner，实际: %v", err)
	}
}

func TestImportRejectsMissingConfigFile(t *testing.T) {
	err := runImport([]string{"-c", "/不存在的路径/config.yaml", "-owner", "admin"})
	if err == nil {
		t.Fatal("配置文件不存在应报错")
	}
}

// 配置校验要在连数据库之前完成：配置写错就该立刻报错，
// 而不是等连上库才发现
func TestImportValidatesConfigBeforeConnecting(t *testing.T) {
	t.Setenv("MAGICD_DATABASE_URL", "")

	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfg, []byte(`
accounts:
  - name: 小号
    cookieFile: cookie.txt
    rooms:
      - id: "123"
        rules:
          - name: 坏规则
            on: [没有这种事件]
            do:
              - type: log
`), 0o600); err != nil {
		t.Fatalf("写配置文件报错: %v", err)
	}

	err := runImport([]string{"-c", cfg, "-owner", "admin"})
	if err == nil {
		t.Fatal("非法配置应报错")
	}
	if strings.Contains(err.Error(), "MAGICD_DATABASE_URL") {
		t.Errorf("应先报配置错误而非数据库未配置，实际: %v", err)
	}
}

func TestReadCookieFileRejectsEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cookie.txt")
	if err := os.WriteFile(path, []byte("   \n"), 0o600); err != nil {
		t.Fatalf("写文件报错: %v", err)
	}
	if _, err := readCookieFile(path); err == nil {
		t.Error("空 Cookie 文件应报错")
	}
}

func TestReadCookieFileTrimsWhitespace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cookie.txt")
	if err := os.WriteFile(path, []byte("  SESSDATA=abc\n"), 0o600); err != nil {
		t.Fatalf("写文件报错: %v", err)
	}
	got, err := readCookieFile(path)
	if err != nil {
		t.Fatalf("读取报错: %v", err)
	}
	if got != "SESSDATA=abc" {
		t.Errorf("= %q", got)
	}
}
