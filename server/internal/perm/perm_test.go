package perm_test

import (
	"strings"
	"testing"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/perm"
)

func TestAllContainsSevenPermissions(t *testing.T) {
	all := perm.All()
	if len(all) != 7 {
		t.Fatalf("权限点数量 = %d, 期望 7: %v", len(all), all)
	}
	seen := make(map[perm.Permission]bool, len(all))
	for _, p := range all {
		if seen[p] {
			t.Errorf("权限点 %q 重复", p)
		}
		seen[p] = true
	}
	for _, want := range []perm.Permission{
		perm.RuleRead, perm.RuleWrite, perm.DanmakuSend,
		perm.UserBlock, perm.AccountManage, perm.MemberManage, perm.EventRead,
	} {
		if !seen[want] {
			t.Errorf("All() 缺少 %q", want)
		}
	}
}

func TestParseKnownPermission(t *testing.T) {
	got, err := perm.Parse("rule:write")
	if err != nil {
		t.Fatalf("Parse 报错: %v", err)
	}
	if got != perm.RuleWrite {
		t.Errorf("Parse(\"rule:write\") = %q, 期望 %q", got, perm.RuleWrite)
	}
}

func TestParseUnknownPermissionListsValidOnes(t *testing.T) {
	_, err := perm.Parse("rule:delete")
	if err == nil {
		t.Fatal("未知权限点应报错")
	}
	// 报错信息要能直接告诉用户合法值有哪些，否则只能翻文档
	if !strings.Contains(err.Error(), "rule:write") {
		t.Errorf("错误信息应列出合法权限点，实际: %v", err)
	}
}

func TestParseListSplitsOnComma(t *testing.T) {
	got, err := perm.ParseList("rule:read, rule:write ,event:read")
	if err != nil {
		t.Fatalf("ParseList 报错: %v", err)
	}
	want := []perm.Permission{perm.RuleRead, perm.RuleWrite, perm.EventRead}
	if len(got) != len(want) {
		t.Fatalf("ParseList 返回 %v, 期望 %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("第 %d 项 = %q, 期望 %q", i, got[i], want[i])
		}
	}
}

func TestParseListRejectsEmpty(t *testing.T) {
	if _, err := perm.ParseList("  "); err == nil {
		t.Fatal("空权限列表应报错")
	}
}

func TestParseListDeduplicates(t *testing.T) {
	got, err := perm.ParseList("rule:read,rule:read")
	if err != nil {
		t.Fatalf("ParseList 报错: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("重复项应去重，实际 %v", got)
	}
}

func TestOwnerBypassExcludesMemberManage(t *testing.T) {
	if perm.OwnerBypass(perm.MemberManage) {
		t.Error("所有者不该凭所有权获得 member:manage——那是新增的委派权，" +
			"不是他已有的收缩性权力的弱化版本")
	}
	for _, p := range perm.All() {
		if p == perm.MemberManage {
			continue
		}
		if !perm.OwnerBypass(p) {
			t.Errorf("所有者应凭所有权获得 %s", p)
		}
	}
}

func TestStringsRoundTrip(t *testing.T) {
	ss := perm.Strings([]perm.Permission{perm.RuleRead, perm.UserBlock})
	if len(ss) != 2 || ss[0] != "rule:read" || ss[1] != "user:block" {
		t.Errorf("Strings = %v", ss)
	}
}
