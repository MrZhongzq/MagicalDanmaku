package buildinfo

import (
	"runtime"
	"strings"
	"testing"
)

func TestGetFillsRuntimeFields(t *testing.T) {
	i := Get()
	if i.GoVersion != runtime.Version() {
		t.Errorf("GoVersion = %q, 期望 %q", i.GoVersion, runtime.Version())
	}
	want := runtime.GOOS + "/" + runtime.GOARCH
	if i.Platform != want {
		t.Errorf("Platform = %q, 期望 %q", i.Platform, want)
	}
	if i.Version == "" {
		t.Error("Version 不得为空")
	}
}

func TestStringIncludesVersionAndCommit(t *testing.T) {
	i := Info{Version: "v7.0.0", Commit: "abc1234"}
	got := i.String()
	if !strings.Contains(got, "v7.0.0") || !strings.Contains(got, "abc1234") {
		t.Errorf("String() = %q", got)
	}
}

func TestStringOmitsEmptyCommit(t *testing.T) {
	i := Info{Version: "dev"}
	got := i.String()
	if strings.Contains(got, "()") {
		t.Errorf("提交为空时不应出现空括号: %q", got)
	}
	if !strings.Contains(got, "dev") {
		t.Errorf("String() = %q", got)
	}
}

func TestDetailNeverLeavesBlankFields(t *testing.T) {
	i := Info{Version: "", Commit: "", Date: "", GoVersion: "go1.24", Platform: "linux/amd64"}
	got := i.Detail()
	// 每一行冒号后都必须有内容，否则输出看起来像坏掉了
	for _, line := range strings.Split(got, "\n") {
		if strings.HasSuffix(strings.TrimSpace(line), ":") {
			t.Errorf("存在空字段行: %q", line)
		}
	}
	if !strings.Contains(got, "未知") {
		t.Errorf("空字段应显示占位符，实际:\n%s", got)
	}
}

func TestDetailIncludesAllFields(t *testing.T) {
	i := Info{Version: "v7.0.0", Commit: "abc1234", Date: "2026-07-31T09:00:00Z",
		GoVersion: "go1.24", Platform: "windows/amd64"}
	got := i.Detail()
	for _, want := range []string{"v7.0.0", "abc1234", "2026-07-31T09:00:00Z", "go1.24", "windows/amd64"} {
		if !strings.Contains(got, want) {
			t.Errorf("Detail() 缺少 %q，实际:\n%s", want, got)
		}
	}
}
