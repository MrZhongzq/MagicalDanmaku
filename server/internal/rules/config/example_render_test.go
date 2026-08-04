package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules"
)

// 终审 Important-2：盲盒盈亏播报模板从来没有被任何测试真正渲染过。
//
// TestExampleConfigParses（example_test.go）只 Parse，不 Render；
// rules/spec/spec_test.go:184 的 TestAggregateByBlindBoxThroughToRule
// 用的是一条自己另外写的简化模板（`{{.blindBox.name}} 本轮盈亏
// {{.blindBox.profitYuan}} 元`），根本不是 config.example.yaml 或
// Danmaku.vue 里真正会发到直播间的那一条。终审实跑发现这条真模板在
// profit 为负、为零、或 blindBox 整组变量缺失时都有问题（双重否定
// "亏了 -11 元"、profit=0 走进 else 分支说成"亏了 0 元"、变量缺失时
// gt/lt 拿空字符串跟数字比较直接渲染报错，整条答谢发不出去）——全批次
// 没有一个测试渲染过这两条真正会用到的模板，这里补上。

// blindBoxProfitTemplate 从解析后的 config.example.yaml 里找出「盲盒
// 答谢」规则的模板文本——用真实解析出来的值而不是在测试里另外抄一份
// 字面量，这样如果有人改了 config.example.yaml 里的模板措辞，这条测试
// 会自动跟着测新文本，不会因为测试自己抄的旧文本还在而掩盖真实模板
// 已经变了的事实。
func blindBoxProfitTemplate(t *testing.T) string {
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
				if r.Name != "盲盒答谢" {
					continue
				}
				for _, action := range r.Do {
					if action.Type == rules.ActionDanmaku && len(action.Template) > 0 {
						return action.Template[0]
					}
				}
			}
		}
	}
	t.Fatal(`config.example.yaml 里没找到名为 "盲盒答谢" 的规则（或它没有 danmaku 模板）——` +
		`这条测试锚定的规则名/结构本身是不是变了？`)
	return ""
}

// blindBoxVars 构造 mergeBuckets 结算之后真实会产出的那一形状的 vars
// （见 rules/aggregate.go mergeBuckets 对 vars["blindBox"] 的赋值：
// count/profit 是 int64，profitYuan/profitAbsYuan 是 formatYuan 生成的
// 展示字符串），不依赖跑一遍完整的 Aggregator——这里只关心模板渲染
// 本身对不对。**类型必须跟生产一致**：早期版本这里传的是字符串
// "5400"，而 rules.tmplInt 只认 int/int32/int64/float32/float64，字符串
// 会被安全转成 0——传错类型会让这条测试全部渲染成"不赚不亏"分支，
// 看起来测试通过了实际什么都没验证到（第一次跑就抓到了这个自制 bug，
// 不是编出来的）。
func blindBoxVars(name string, count, profit int64, profitYuanStr, absProfitYuanStr string, includeBlindBox bool) map[string]any {
	vars := map[string]any{
		"user": map[string]any{"username": "张三"},
	}
	if includeBlindBox {
		vars["blindBox"] = map[string]any{
			"name":          name,
			"count":         count,
			"profit":        profit,
			"profitYuan":    profitYuanStr,
			"profitAbsYuan": absProfitYuanStr,
		}
	}
	return vars
}

// TestExampleConfigBlindBoxProfitTemplateRenders 覆盖正/负/零三种盈亏
// ——终审报告点名要求的三个场景，逐字断言渲染结果，不接受双重否定
// （"亏了 -11 元"）或者 profit=0 时说"亏了 0 元"这种错误文案。
func TestExampleConfigBlindBoxProfitTemplateRenders(t *testing.T) {
	tmpl := blindBoxProfitTemplate(t)
	r := rules.NewRenderer(nil)

	cases := []struct {
		name string
		vars map[string]any
		want string
	}{
		{
			name: "盈利",
			// 数值对齐用户真实样本（幸运盲盒 profit=+5400，见
			// aggregate_test.go TestAggregateByBlindBoxSeparatesProfitByName）。
			vars: blindBoxVars("幸运盲盒", 1, 5400, "5.4", "5.4", true),
			// P6 任务 2：变量与中文之间不留空格（"x1" 前那个空格是英文
			// 字母边界，不属于这次改动范围，原样保留）。
			want: "感谢张三的幸运盲盒 x1，赚了5.4元！",
		},
		{
			name: "亏损",
			// 数值对齐用户真实样本（心动盲盒 profit=-11000）。
			// profitYuan 自带负号（"-11"），但模板的亏损分支应该读
			// profitAbsYuan（"11"，不带负号），不该拼出"亏了 -11 元"。
			vars: blindBoxVars("心动盲盒", 1, -11000, "-11", "11", true),
			want: "感谢张三的心动盲盒 x1，亏了11元！",
		},
		{
			name: "打平",
			vars: blindBoxVars("幸运盲盒", 1, 0, "0", "0", true),
			want: "感谢张三的幸运盲盒 x1，不赚不亏！",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := r.RenderOne(tmpl, c.vars)
			if err != nil {
				t.Fatalf("渲染报错: %v", err)
			}
			if got != c.want {
				t.Errorf("渲染结果 = %q, 期望 %q", got, c.want)
			}
		})
	}
}

// TestExampleConfigBlindBoxProfitTemplateSurvivesMissingBlindBoxVars 是
// 终审实跑发现的第二个问题的回归测试：如果有人在自定义弹幕姬页配了
// `by: blindBox` 却忘了加 `when: gift.isBlindBox`，每一条普通礼物都会
// 走进这条聚合规则，此时 vars 里根本没有 "blindBox" 这个键。旧模板
// `{{if gt .blindBox.profit 0}}` 会因为 .blindBox.profit 渲染成空字符串、
// gt 拿空字符串跟数字比较报 "incompatible types for comparison" 而
// 渲染失败、整条答谢发不出去。新模板用 {{int ...}} 包一层，缺失时安全
// 退化成 0，落到"不赚不亏"分支，不炸也不发错误文案。
func TestExampleConfigBlindBoxProfitTemplateSurvivesMissingBlindBoxVars(t *testing.T) {
	tmpl := blindBoxProfitTemplate(t)
	r := rules.NewRenderer(nil)

	vars := blindBoxVars("", 0, 0, "", "", false) // 不含 blindBox 整组变量

	got, err := r.RenderOne(tmpl, vars)
	if err != nil {
		t.Fatalf("blindBox 变量整组缺失时不应该渲染报错，实际: %v", err)
	}
	if strings.Contains(got, "<no value>") {
		t.Errorf("渲染结果不应该出现 <no value>，实际: %q", got)
	}
	if !strings.Contains(got, "不赚不亏") {
		t.Errorf("blindBox 变量缺失时应该优雅退化到「不赚不亏」分支，实际渲染结果: %q", got)
	}
}

// TestExampleConfigBlindBoxProfitTemplateMatchesDanmakuVueLiteral 是终审
// 指出的跨语言字面量问题的低成本封堵（同类封堵的先例见
// pk_pipeline_test.go 的 TestPKOpponentSnapshotSubCommandMatchesFrontendLiteral）：
// config.example.yaml 与 web/src/pages/Danmaku.vue 的
// BLIND_BOX_TEMPLATE_WITH_PROFIT 常量必须是逐字相同的模板——这条真正
// 会发到直播间的模板文案，两边各写一份字面量，任何一边改了措辞/变量名
// 而另一边没跟上，表现是同一句播报在「内置盲盒答谢」与「自定义配置」
// 两条路径上不一致，没有任何报错能发现。
func TestExampleConfigBlindBoxProfitTemplateMatchesDanmakuVueLiteral(t *testing.T) {
	tmpl := blindBoxProfitTemplate(t)

	vuePath := filepath.Join("..", "..", "..", "..", "web", "src", "pages", "Danmaku.vue")
	vueSrc, err := os.ReadFile(vuePath)
	if err != nil {
		t.Fatalf("读取 Danmaku.vue 失败（路径 %s 是否还对）: %v", vuePath, err)
	}

	literal := "'" + tmpl + "'"
	if !strings.Contains(string(vueSrc), literal) {
		t.Errorf("Danmaku.vue 里没有找到跟 config.example.yaml「盲盒答谢」规则完全一致的 "+
			"BLIND_BOX_TEMPLATE_WITH_PROFIT 字面量，两边已经漂开——期望包含:\n%s", literal)
	}
}
