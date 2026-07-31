package rules

import (
	"math/rand"
	"strings"
	"testing"
)

func newTestRenderer() *Renderer {
	// 固定种子保证随机选择可复现
	return NewRenderer(rand.New(rand.NewSource(1)))
}

func TestRenderSimpleField(t *testing.T) {
	r := newTestRenderer()
	vars := map[string]any{"user": map[string]any{"username": "路人甲"}}

	got, err := r.RenderOne("欢迎 {{.user.username}} 进入直播间", vars)
	if err != nil {
		t.Fatalf("RenderOne 失败: %v", err)
	}
	if got != "欢迎 路人甲 进入直播间" {
		t.Errorf("= %q", got)
	}
}

func TestRenderMissingFieldIsEmpty(t *testing.T) {
	r := newTestRenderer()
	got, err := r.RenderOne("值是[{{.不存在}}]", map[string]any{})
	if err != nil {
		t.Fatalf("缺失字段不应报错: %v", err)
	}
	if got != "值是[]" {
		t.Errorf("缺失字段应渲染为空串，实际 %q", got)
	}
}

func TestRenderMissingNestedFieldIsEmpty(t *testing.T) {
	r := newTestRenderer()
	vars := map[string]any{"user": map[string]any{"username": "甲"}}
	got, err := r.RenderOne("[{{.user.medal.name}}]", vars)
	if err != nil {
		t.Fatalf("缺失嵌套字段不应报错: %v", err)
	}
	if got != "[]" {
		t.Errorf("= %q", got)
	}
}

func TestRenderJoin(t *testing.T) {
	r := newTestRenderer()
	vars := map[string]any{"users": []string{"甲", "乙", "丙"}}
	got, err := r.RenderOne(`欢迎 {{join .users "、"}} 回家`, vars)
	if err != nil {
		t.Fatalf("RenderOne 失败: %v", err)
	}
	if got != "欢迎 甲、乙、丙 回家" {
		t.Errorf("= %q", got)
	}
}

func TestRenderJoinHandlesAnySlice(t *testing.T) {
	r := newTestRenderer()
	// Vars 里的数组可能是 []any
	vars := map[string]any{"users": []any{"甲", "乙"}}
	got, err := r.RenderOne(`{{join .users ","}}`, vars)
	if err != nil {
		t.Fatalf("RenderOne 失败: %v", err)
	}
	if got != "甲,乙" {
		t.Errorf("= %q", got)
	}
}

func TestRenderSimplifyName(t *testing.T) {
	cases := map[string]string{
		"路人甲":          "路人甲",
		"【官方】某某某":      "某某某",
		"某某某_official": "某某某",
		"·-·某某·-·":     "某某",
		"某某某-许许的蓷":     "某某某-许许的蓷",
	}
	r := newTestRenderer()
	for in, want := range cases {
		vars := map[string]any{"n": in}
		got, err := r.RenderOne("{{simplifyName .n}}", vars)
		if err != nil {
			t.Fatalf("RenderOne 失败: %v", err)
		}
		if got != want {
			t.Errorf("simplifyName(%q) = %q, 期望 %q", in, got, want)
		}
	}
}

func TestRenderTruncate(t *testing.T) {
	r := newTestRenderer()
	vars := map[string]any{"s": "一二三四五六七八九十"}
	got, err := r.RenderOne("{{truncate .s 5}}", vars)
	if err != nil {
		t.Fatalf("RenderOne 失败: %v", err)
	}
	if got != "一二三四五" {
		t.Errorf("= %q（应按字符而非字节截断）", got)
	}
}

func TestRenderPick(t *testing.T) {
	r := newTestRenderer()
	got, err := r.RenderOne(`{{pick "早上好" "中午好" "晚上好"}}`, map[string]any{})
	if err != nil {
		t.Fatalf("RenderOne 失败: %v", err)
	}
	if got != "早上好" && got != "中午好" && got != "晚上好" {
		t.Errorf("pick 应返回其中之一，实际 %q", got)
	}
}

func TestRenderConditional(t *testing.T) {
	r := newTestRenderer()
	vars := map[string]any{"user": map[string]any{"guardLevel": 3, "username": "甲"}}
	tmpl := `{{if gt (int .user.guardLevel) 0}}舰长{{end}}{{.user.username}}`
	got, err := r.RenderOne(tmpl, vars)
	if err != nil {
		t.Fatalf("RenderOne 失败: %v", err)
	}
	if got != "舰长甲" {
		t.Errorf("= %q", got)
	}
}

func TestRenderPicksFromMultipleTemplates(t *testing.T) {
	r := newTestRenderer()
	tmpls := []string{"A", "B", "C"}
	seen := map[string]bool{}
	for i := 0; i < 60; i++ {
		got, err := r.Render(tmpls, map[string]any{})
		if err != nil {
			t.Fatalf("Render 失败: %v", err)
		}
		seen[got] = true
	}
	if len(seen) != 3 {
		t.Errorf("60 次应覆盖全部 3 条模板，实际只出现 %v", seen)
	}
}

func TestRenderSingleTemplate(t *testing.T) {
	r := newTestRenderer()
	got, err := r.Render([]string{"只有一条"}, map[string]any{})
	if err != nil {
		t.Fatalf("Render 失败: %v", err)
	}
	if got != "只有一条" {
		t.Errorf("= %q", got)
	}
}

func TestRenderEmptyListFails(t *testing.T) {
	r := newTestRenderer()
	if _, err := r.Render(nil, map[string]any{}); err == nil {
		t.Error("空模板列表应当报错")
	}
}

func TestRenderBadSyntaxFails(t *testing.T) {
	r := newTestRenderer()
	if _, err := r.RenderOne("{{.未闭合", map[string]any{}); err == nil {
		t.Error("语法错误应当报错")
	}
	// 错误信息应能定位问题
	_, err := r.RenderOne("{{未知函数 .x}}", map[string]any{})
	if err == nil || !strings.Contains(err.Error(), "模板") {
		t.Errorf("错误信息应提及模板，实际 %v", err)
	}
}

func TestRewriteFieldChains(t *testing.T) {
	cases := map[string]string{
		`{{.user.username}}`:              `{{(get . "user.username")}}`,
		`{{.text}}`:                       `{{(get . "text")}}`,
		`{{simplifyName .user.username}}`: `{{simplifyName (get . "user.username")}}`,
		`{{join .users "、"}}`:             `{{join (get . "users") "、"}}`,
		// 引号内的点不得被改写
		`{{pick "a.b" "c.d"}}`: `{{pick "a.b" "c.d"}}`,
		// 动作外的点不得被改写
		`价格 3.14 元 {{.x}}`: `价格 3.14 元 {{(get . "x")}}`,
		// 孤立点原样保留
		`{{.}}`: `{{.}}`,
		// 无动作时原样返回
		`纯文本`: `纯文本`,
	}
	for in, want := range cases {
		if got := rewriteFieldChains(in); got != want {
			t.Errorf("rewriteFieldChains(%q)\n  = %q\n期望 %q", in, got, want)
		}
	}
}

func TestRenderDoesNotRewriteInsideStringLiteral(t *testing.T) {
	r := newTestRenderer()
	got, err := r.RenderOne(`{{pick "1.5秒"}}`, map[string]any{})
	if err != nil {
		t.Fatalf("RenderOne 失败: %v", err)
	}
	if got != "1.5秒" {
		t.Errorf("字符串字面量内的点不应被改写，实际 %q", got)
	}
}

func TestRenderChineseFieldName(t *testing.T) {
	r := newTestRenderer()
	vars := map[string]any{"中文键": "值"}
	got, err := r.RenderOne(`{{.中文键}}`, vars)
	if err != nil {
		t.Fatalf("RenderOne 失败: %v", err)
	}
	if got != "值" {
		t.Errorf("= %q", got)
	}
}
