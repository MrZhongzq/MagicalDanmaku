package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules"
)

// 本文件补齐 P6 任务 2 要求的渲染测试：不只改字符串就交差（P4-4 时期
// 盲盒答谢那两条模板从来没有被任何测试真正渲染过，长期输出"亏了 -11
// 元"这种双重否定，直到终审才发现）。除了任务点名必须补的盲盒两条模板，
// 这里也顺带覆盖"检查其它模板有没有同样的空格问题"点名的几处
// （进房欢迎、礼物答谢、PK 播报）——都是真正会发到直播间的默认模板。
//
// 手法与 example_render_test.go 的 TestExampleConfigBlindBoxProfitTemplate
// MatchesDanmakuVueLiteral 一致：不在 Go 测试里另抄一份字面量，而是从
// 前端源文件里原样读出来再渲染——这样如果有人改了前端的模板文案而忘了
// 同步更新预期输出，测试会先在"读不到这个片段"这一步就报错，不会悄悄
// 继续拿一份过时的字面量渲染出"看起来通过了"的假绿。

// readFrontendSource 读取相对 web/src 的前端源文件全文。
func readFrontendSource(t *testing.T, relPath string) string {
	t.Helper()
	full := filepath.Join("..", "..", "..", "..", "web", "src", relPath)
	b, err := os.ReadFile(full)
	if err != nil {
		t.Fatalf("读取前端源文件 %s 失败: %v", full, err)
	}
	return string(b)
}

// extractSingleQuoted 从源码里找到形如 needle'内容' 的第一处匹配，
// 返回内容部分（不含引号）。needle 通常是变量名+等号（或数组开头），
// 用来定位到具体是哪一条模板字面量，避免文件里其它单引号字符串误命中。
func extractSingleQuoted(t *testing.T, src, needle string) string {
	t.Helper()
	idx := strings.Index(src, needle)
	if idx < 0 {
		t.Fatalf("源码里没有找到锚点 %q——模板常量是不是被重命名或挪动了？", needle)
	}
	rest := src[idx+len(needle):]
	start := strings.IndexByte(rest, '\'')
	if start < 0 {
		t.Fatalf("锚点 %q 之后没有找到单引号字符串", needle)
	}
	rest = rest[start+1:]
	end := strings.IndexByte(rest, '\'')
	if end < 0 {
		t.Fatalf("锚点 %q 对应的单引号字符串没有闭合", needle)
	}
	return rest[:end]
}

// TestBlindBoxWithoutProfitTemplateRenders 补齐"不带盈亏"这一条盲盒答谢
// 模板的渲染测试——config.example.yaml 里没有对应的示范规则（那边只有
// 带盈亏的版本），这条模板只存在于 Danmaku.vue，之前完全没有被渲染过。
func TestBlindBoxWithoutProfitTemplateRenders(t *testing.T) {
	src := readFrontendSource(t, "pages/Danmaku.vue")
	tmpl := extractSingleQuoted(t, src, "const BLIND_BOX_TEMPLATE_WITHOUT_PROFIT =")

	r := rules.NewRenderer(nil)
	got, err := r.RenderOne(tmpl, map[string]any{
		"user":     map[string]any{"username": "张三"},
		"blindBox": map[string]any{"name": "幸运盲盒", "count": int64(3)},
	})
	if err != nil {
		t.Fatalf("渲染报错: %v", err)
	}
	const want = "感谢张三开出了3次幸运盲盒！"
	if got != want {
		t.Errorf("渲染结果 = %q, 期望 %q", got, want)
	}
}

// TestGuardBuyThanksTemplateRenders 覆盖上舰答谢的默认模板——它跟盲盒
// 答谢一样带 {{if}} 分支（新购/续费），是这次改动里除盲盒外唯一还有
// 条件逻辑的模板，同一类"改完没渲染验证过"的风险不该只堵盲盒那两条。
//
// Danmaku.vue 里这条模板用 `+` 拼成两段（见 defaultGuardDraft），这里
// 分别按锚点提取两段再拼接，不依赖 `+` 拼接的具体折行格式。
func TestGuardBuyThanksTemplateRenders(t *testing.T) {
	src := readFrontendSource(t, "pages/Danmaku.vue")

	// guard 相关字段是这条模板独有的标志，直接用它定位第一段——两段
	// 紧挨着都以单引号开头，没有一个通用锚点能同时区分开，只能各自
	// 手写定位逻辑（下面的 seg2 同理）。
	idx := strings.Index(src, "'感谢{{.user.username}}{{if .guard.isRenew}}")
	if idx < 0 {
		t.Fatal("源码里没有找到上舰答谢默认模板的第一段——模板文案是不是变了？")
	}
	rest := src[idx+1:]
	end1 := strings.IndexByte(rest, '\'')
	if end1 < 0 {
		t.Fatal("上舰答谢默认模板第一段没有闭合引号")
	}
	seg1 := rest[:end1]

	rest2 := rest[end1+1:]
	start2 := strings.IndexByte(rest2, '\'')
	if start2 < 0 {
		t.Fatal("上舰答谢默认模板找不到第二段")
	}
	rest2 = rest2[start2+1:]
	end2 := strings.IndexByte(rest2, '\'')
	if end2 < 0 {
		t.Fatal("上舰答谢默认模板第二段没有闭合引号")
	}
	seg2 := rest2[:end2]

	tmpl := seg1 + seg2
	r := rules.NewRenderer(nil)

	cases := []struct {
		name    string
		isRenew bool
		want    string
	}{
		{"新购", false, "感谢张三开通3个月舰长，感谢老板的支持！"},
		{"续费", true, "感谢张三续费3个月舰长，感谢老板的支持！"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			vars := map[string]any{
				"user":  map[string]any{"username": "张三"},
				"guard": map[string]any{"isRenew": c.isRenew, "count": int64(3), "name": "舰长"},
			}
			got, err := r.RenderOne(tmpl, vars)
			if err != nil {
				t.Fatalf("渲染报错: %v", err)
			}
			if got != c.want {
				t.Errorf("渲染结果 = %q, 期望 %q", got, c.want)
			}
		})
	}
}

// TestPkMatchTemplateRenders 覆盖 PK 匹配信息播报的默认模板——用户真机
// 反馈第 2 条点名"PK 播报"也要检查空格问题。
func TestPkMatchTemplateRenders(t *testing.T) {
	src := readFrontendSource(t, "components/PkPanel.vue")

	idx := strings.Index(src, "'对面主播是{{.pk.opponent.uname}}")
	if idx < 0 {
		t.Fatal("源码里没有找到 PK 匹配信息默认模板的第一段——模板文案是不是变了？")
	}
	rest := src[idx+1:]
	end1 := strings.IndexByte(rest, '\'')
	if end1 < 0 {
		t.Fatal("PK 匹配信息默认模板第一段没有闭合引号")
	}
	seg1 := rest[:end1]

	rest2 := rest[end1+1:]
	start2 := strings.IndexByte(rest2, '\'')
	if start2 < 0 {
		t.Fatal("PK 匹配信息默认模板找不到第二段")
	}
	rest2 = rest2[start2+1:]
	end2 := strings.IndexByte(rest2, '\'')
	if end2 < 0 {
		t.Fatal("PK 匹配信息默认模板第二段没有闭合引号")
	}
	seg2 := rest2[:end2]

	tmpl := seg1 + seg2
	r := rules.NewRenderer(nil)
	vars := map[string]any{
		"pk": map[string]any{
			"opponent": map[string]any{
				"uname":      "花花",
				"online":     int64(1024),
				"guardTotal": int64(8),
			},
		},
	}
	got, err := r.RenderOne(tmpl, vars)
	if err != nil {
		t.Fatalf("渲染报错: %v", err)
	}
	const want = "对面主播是花花，直播间1024人在线，大航海8位，一起加油！"
	if got != want {
		t.Errorf("渲染结果 = %q, 期望 %q", got, want)
	}
}

// TestPkVisitTemplateRenders 覆盖 PK 串门欢迎的默认模板。
func TestPkVisitTemplateRenders(t *testing.T) {
	src := readFrontendSource(t, "components/PkPanel.vue")
	tmpl := extractSingleQuoted(t, src, "visitTemplates: [")

	r := rules.NewRenderer(nil)
	got, err := r.RenderOne(tmpl, map[string]any{"user": map[string]any{"username": "张三"}})
	if err != nil {
		t.Fatalf("渲染报错: %v", err)
	}
	const want = "欢迎对面直播间的朋友张三来串门认识一下~"
	if got != want {
		t.Errorf("渲染结果 = %q, 期望 %q", got, want)
	}
}

// TestExampleConfigEnterWelcomeTemplatesRender 覆盖 config.example.yaml
// 「舰长进场欢迎」的两条模板——用户真机反馈第 2 条点名"进房欢迎"也要
// 检查空格问题，且这条模板用了 join 函数，验证空格改动没有破坏语法。
func TestExampleConfigEnterWelcomeTemplatesRender(t *testing.T) {
	templates := exampleRuleTemplates(t, "舰长进场欢迎")
	if len(templates) != 2 {
		t.Fatalf("模板条数 = %d, 期望 2", len(templates))
	}

	r := rules.NewRenderer(nil)
	vars := map[string]any{"users": []string{"张三", "李四"}}

	got0, err := r.RenderOne(templates[0], vars)
	if err != nil {
		t.Fatalf("渲染第 1 条报错: %v", err)
	}
	if want := "欢迎张三、李四回家~"; got0 != want {
		t.Errorf("第 1 条渲染结果 = %q, 期望 %q", got0, want)
	}

	got1, err := r.RenderOne(templates[1], vars)
	if err != nil {
		t.Fatalf("渲染第 2 条报错: %v", err)
	}
	if want := "张三、李四来啦！"; got1 != want {
		t.Errorf("第 2 条渲染结果 = %q, 期望 %q", got1, want)
	}
}

// TestExampleConfigGiftThanksTemplateRenders 覆盖 config.example.yaml
// 「礼物答谢」模板。
func TestExampleConfigGiftThanksTemplateRenders(t *testing.T) {
	templates := exampleRuleTemplates(t, "礼物答谢")
	if len(templates) != 1 {
		t.Fatalf("模板条数 = %d, 期望 1", len(templates))
	}

	r := rules.NewRenderer(nil)
	vars := map[string]any{
		"user": map[string]any{"username": "张三"},
		"gift": map[string]any{"name": "辣条", "count": int64(1)},
	}
	got, err := r.RenderOne(templates[0], vars)
	if err != nil {
		t.Fatalf("渲染报错: %v", err)
	}
	const want = "感谢张三的辣条 x1！"
	if got != want {
		t.Errorf("渲染结果 = %q, 期望 %q", got, want)
	}
}

// TestExampleConfigGuardThanksTemplateRenders 覆盖 config.example.yaml
// 「上舰答谢」模板。
func TestExampleConfigGuardThanksTemplateRenders(t *testing.T) {
	templates := exampleRuleTemplates(t, "上舰答谢")
	if len(templates) != 1 {
		t.Fatalf("模板条数 = %d, 期望 1", len(templates))
	}

	r := rules.NewRenderer(nil)
	vars := map[string]any{
		"user":  map[string]any{"username": "张三"},
		"guard": map[string]any{"name": "舰长"},
	}
	got, err := r.RenderOne(templates[0], vars)
	if err != nil {
		t.Fatalf("渲染报错: %v", err)
	}
	const want = "感谢张三开通舰长！"
	if got != want {
		t.Errorf("渲染结果 = %q, 期望 %q", got, want)
	}
}

// exampleRuleTemplates 从解析后的 config.example.yaml 里找出名为 ruleName
// 的规则的全部 danmaku 模板文本——与 example_render_test.go 里
// blindBoxProfitTemplate 是同一个手法（用真实解析出来的值而不是在测试
// 里另外抄一份字面量），这里抽成通用版本供多条规则复用。
func exampleRuleTemplates(t *testing.T, ruleName string) []string {
	t.Helper()
	b, err := os.ReadFile("../../../../config.example.yaml")
	if err != nil {
		t.Fatalf("读示例配置报错: %v", err)
	}
	cfg, err := Parse(b)
	if err != nil {
		t.Fatalf("示例配置解析失败: %v", err)
	}

	for _, a := range cfg.Accounts {
		for _, room := range a.Rooms {
			for _, r := range room.Rules {
				if r.Name != ruleName {
					continue
				}
				for _, action := range r.Do {
					if action.Type == rules.ActionDanmaku && len(action.Template) > 0 {
						return action.Template
					}
				}
			}
		}
	}
	t.Fatalf("config.example.yaml 里没找到名为 %q 的规则（或它没有 danmaku 模板）——"+
		"这条测试锚定的规则名/结构本身是不是变了？", ruleName)
	return nil
}
