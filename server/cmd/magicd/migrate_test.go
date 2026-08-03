package main

import (
	"strings"
	"testing"
)

// TestResolveAdminPasswordRequiresEnvOnEmptyDatabase 钉住本任务的核心行为：
// 空库且没设 MAGICD_ADMIN_PASSWORD 时必须报错，而不是退回随机密码——
// 那正是无头部署反馈里"密码只在标准输出打印一次，只能翻日志找"的坑。
func TestResolveAdminPasswordRequiresEnvOnEmptyDatabase(t *testing.T) {
	_, err := resolveAdminPassword(0, "")
	if err == nil {
		t.Fatal("空库且未设置环境变量应报错")
	}
	if !strings.Contains(err.Error(), adminPasswordEnvVar) {
		t.Errorf("错误信息应提示环境变量名，实际: %v", err)
	}
	if !strings.Contains(err.Error(), "openssl") {
		t.Errorf("错误信息应给出可执行的解决办法，实际: %v", err)
	}
}

// TestResolveAdminPasswordUsesEnvOnEmptyDatabase 验证设置了环境变量时，
// 用的就是这个值。
func TestResolveAdminPasswordUsesEnvOnEmptyDatabase(t *testing.T) {
	pass, err := resolveAdminPassword(0, "一个足够强的密码123")
	if err != nil {
		t.Fatalf("报错: %v", err)
	}
	if pass != "一个足够强的密码123" {
		t.Errorf("pass = %q", pass)
	}
}

// TestResolveAdminPasswordSkipsWhenUsersExist 验证库里已有用户时，
// 不管环境变量设没设，都不该报错——EnsureAdmin 对这种情况本来就是
// 空操作，不该被这个环境变量卡住正常的重启/重跑 migrate。
func TestResolveAdminPasswordSkipsWhenUsersExist(t *testing.T) {
	if _, err := resolveAdminPassword(1, ""); err != nil {
		t.Errorf("库里已有用户时不该报错，实际: %v", err)
	}
	if _, err := resolveAdminPassword(3, "无所谓的值"); err != nil {
		t.Errorf("库里已有用户时不该报错，实际: %v", err)
	}
}
