package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules"
)

// fullYAML 覆盖三层嵌套的完整形态：账号 → 直播间 → 规则。
const fullYAML = `
accounts:
  - name: 主播号
    cookieFile: cookie-main.txt
    rateLimit: 1.5s
    rooms:
      - id: "1706666491"
        cooldownGroups:
          moderation: 5s
        rules:
          - name: 广告禁言
            on: [danmaku]
            when:
              any:
                - field: text
                  op: regex
                  value: "(广告|加群)"
                - field: text
                  op: contains
                  value: "违禁词"
            cooldownGroup: moderation
            do:
              - type: block
                hours: 1

  - name: 小号
    cookieFile: cookie-sub.txt
    rateLimit: 2s
    rooms:
      - id: "1706666491"
        cooldownGroups:
          greeting: 5s
        rules:
          - name: 舰长进场欢迎
            enabled: true
            on: [user_enter]
            when:
              field: user.guardLevel
              op: ">"
              value: 0
            aggregate:
              window: 2s
              by: type
            cooldownGroup: greeting
            cooldown: 3s
            do:
              - type: danmaku
                template:
                  - "欢迎 {{join .users \"、\"}} 回家~"
                  - "{{join .users \"、\"}} 来啦！"

      - id: "22222222"
        rules:
          - name: 定时广告
            schedule: "0 */5 * * * *"
            do:
              - type: danmaku
                template: ["关注主播不迷路~"]
`

func TestParseAccountStructure(t *testing.T) {
	c, err := Parse([]byte(fullYAML))
	if err != nil {
		t.Fatalf("Parse 失败: %v", err)
	}

	if len(c.Accounts) != 2 {
		t.Fatalf("账号数 = %d, 期望 2", len(c.Accounts))
	}
	if c.Accounts[0].Name != "主播号" || c.Accounts[0].CookieFile != "cookie-main.txt" {
		t.Errorf("Accounts[0] = %+v", c.Accounts[0])
	}
	if time.Duration(c.Accounts[0].RateLimit) != 1500*time.Millisecond {
		t.Errorf("主播号 rateLimit = %v", time.Duration(c.Accounts[0].RateLimit))
	}
	if time.Duration(c.Accounts[1].RateLimit) != 2*time.Second {
		t.Errorf("小号 rateLimit = %v", time.Duration(c.Accounts[1].RateLimit))
	}

	// 小号连了两个直播间
	if len(c.Accounts[1].Rooms) != 2 {
		t.Fatalf("小号的直播间数 = %d, 期望 2", len(c.Accounts[1].Rooms))
	}
	if c.Accounts[1].Rooms[0].ID != "1706666491" || c.Accounts[1].Rooms[1].ID != "22222222" {
		t.Errorf("小号的直播间 = %v %v", c.Accounts[1].Rooms[0].ID, c.Accounts[1].Rooms[1].ID)
	}
}

func TestParseSameRoomUnderDifferentAccounts(t *testing.T) {
	// 同一直播间被两个账号连接，是两个独立绑定
	c, err := Parse([]byte(fullYAML))
	if err != nil {
		t.Fatalf("Parse 失败: %v", err)
	}
	if c.Accounts[0].Rooms[0].ID != c.Accounts[1].Rooms[0].ID {
		t.Fatal("前置条件错误：两个账号应连同一个直播间")
	}
	// 各自的规则完全独立
	if c.Accounts[0].Rooms[0].Rules[0].Name != "广告禁言" {
		t.Errorf("主播号的规则 = %q", c.Accounts[0].Rooms[0].Rules[0].Name)
	}
	if c.Accounts[1].Rooms[0].Rules[0].Name != "舰长进场欢迎" {
		t.Errorf("小号的规则 = %q", c.Accounts[1].Rooms[0].Rules[0].Name)
	}
}

func TestParseRoomCooldownGroups(t *testing.T) {
	c, err := Parse([]byte(fullYAML))
	if err != nil {
		t.Fatalf("Parse 失败: %v", err)
	}
	g := c.Accounts[1].Rooms[0].CooldownGroups
	if time.Duration(g["greeting"]) != 5*time.Second {
		t.Errorf("greeting = %v", time.Duration(g["greeting"]))
	}
	// 未配置冷却组的直播间应为空而非 nil 解引用panic
	if len(c.Accounts[1].Rooms[1].CooldownGroups) != 0 {
		t.Errorf("未配置时应为空，实际 %v", c.Accounts[1].Rooms[1].CooldownGroups)
	}
}

func TestParseRuleFields(t *testing.T) {
	c, err := Parse([]byte(fullYAML))
	if err != nil {
		t.Fatalf("Parse 失败: %v", err)
	}
	r := c.Accounts[1].Rooms[0].Rules[0]

	if r.Name != "舰长进场欢迎" {
		t.Errorf("Name = %q", r.Name)
	}
	if !r.Enabled {
		t.Error("Enabled 应为 true")
	}
	if len(r.On) != 1 || r.On[0] != event.TypeUserEnter {
		t.Errorf("On = %v", r.On)
	}
	if r.Cooldown != 3*time.Second {
		t.Errorf("Cooldown = %v", r.Cooldown)
	}
	if r.CooldownGroup != "greeting" {
		t.Errorf("CooldownGroup = %q", r.CooldownGroup)
	}
	if r.Aggregate == nil {
		t.Fatal("Aggregate 不应为 nil")
	}
	if r.Aggregate.Window != 2*time.Second || r.Aggregate.By != rules.AggregateByType {
		t.Errorf("Aggregate = %+v", *r.Aggregate)
	}
	if len(r.Do) != 1 || r.Do[0].Type != rules.ActionDanmaku {
		t.Errorf("Do = %+v", r.Do)
	}
	if len(r.Do[0].Template) != 2 {
		t.Errorf("Template 数 = %d, 期望 2", len(r.Do[0].Template))
	}
}

func TestParseNormalizesOperatorAliases(t *testing.T) {
	c, err := Parse([]byte(fullYAML))
	if err != nil {
		t.Fatalf("Parse 失败: %v", err)
	}
	// 配置里写的是 ">"，应被归一化为 "gt"
	if got := c.Accounts[1].Rooms[0].Rules[0].When.Op; got != "gt" {
		t.Errorf("Op = %q, 期望归一化为 gt", got)
	}
}

func TestParseAllOperatorAliases(t *testing.T) {
	cases := map[string]string{
		">": "gt", ">=": "gte", "<": "lt", "<=": "lte",
		"==": "eq", "=": "eq", "!=": "ne", "<>": "ne",
		"gt": "gt", "contains": "contains",
	}
	for alias, want := range cases {
		y := `
accounts:
  - name: A
    cookieFile: c.txt
    rooms:
      - id: "1"
        rules:
          - name: 测试
            on: [danmaku]
            when: {field: text, op: "` + alias + `", value: "x"}
            do: [{type: log}]
`
		c, err := Parse([]byte(y))
		if err != nil {
			t.Errorf("别名 %q 解析失败: %v", alias, err)
			continue
		}
		if got := c.Accounts[0].Rooms[0].Rules[0].When.Op; got != want {
			t.Errorf("别名 %q 归一化为 %q, 期望 %q", alias, got, want)
		}
	}
}

func TestParseNestedCondition(t *testing.T) {
	c, err := Parse([]byte(fullYAML))
	if err != nil {
		t.Fatalf("Parse 失败: %v", err)
	}
	w := c.Accounts[0].Rooms[0].Rules[0].When
	if w == nil || len(w.Any) != 2 {
		t.Fatalf("嵌套条件解析错误: %+v", w)
	}
	if w.Any[0].Op != "regex" || w.Any[1].Op != "contains" {
		t.Errorf("子条件 = %+v", w.Any)
	}
}

func TestParseScheduledRule(t *testing.T) {
	c, err := Parse([]byte(fullYAML))
	if err != nil {
		t.Fatalf("Parse 失败: %v", err)
	}
	r := c.Accounts[1].Rooms[1].Rules[0]
	if r.Schedule != "0 */5 * * * *" {
		t.Errorf("Schedule = %q", r.Schedule)
	}
	if len(r.On) != 0 {
		t.Errorf("定时规则不应有 On，实际 %v", r.On)
	}
}

func TestParseEnabledDefaultsToTrue(t *testing.T) {
	y := `
accounts:
  - name: A
    cookieFile: c.txt
    rooms:
      - id: "1"
        rules:
          - name: 未写 enabled
            on: [danmaku]
            do: [{type: log}]
`
	c, err := Parse([]byte(y))
	if err != nil {
		t.Fatalf("Parse 失败: %v", err)
	}
	if !c.Accounts[0].Rooms[0].Rules[0].Enabled {
		t.Error("未写 enabled 时应默认启用——写了规则却不生效最反直觉")
	}
}

func TestParseExplicitDisable(t *testing.T) {
	y := `
accounts:
  - name: A
    cookieFile: c.txt
    rooms:
      - id: "1"
        rules:
          - name: 显式禁用
            enabled: false
            on: [danmaku]
            do: [{type: log}]
`
	c, err := Parse([]byte(y))
	if err != nil {
		t.Fatalf("Parse 失败: %v", err)
	}
	if c.Accounts[0].Rooms[0].Rules[0].Enabled {
		t.Error("显式写 false 应被尊重")
	}
}

func TestParseAllowsSameRuleNameInDifferentBindings(t *testing.T) {
	// 规则名只需在单个绑定内唯一。同一条「进场欢迎」出现在多个绑定下
	// 是正常用法——未来靠模板功能减少重复，现在允许重复书写。
	y := `
accounts:
  - name: A
    cookieFile: a.txt
    rooms:
      - id: "1"
        rules:
          - name: 进场欢迎
            on: [user_enter]
            do: [{type: log}]
      - id: "2"
        rules:
          - name: 进场欢迎
            on: [user_enter]
            do: [{type: log}]
  - name: B
    cookieFile: b.txt
    rooms:
      - id: "1"
        rules:
          - name: 进场欢迎
            on: [user_enter]
            do: [{type: log}]
`
	if _, err := Parse([]byte(y)); err != nil {
		t.Errorf("不同绑定下的同名规则应当允许: %v", err)
	}
}

func TestParseRejectsDuplicateRuleNameWithinBinding(t *testing.T) {
	// 同一绑定内重名会让冷却状态互相干扰
	y := `
accounts:
  - name: A
    cookieFile: a.txt
    rooms:
      - id: "1"
        rules:
          - name: 重名
            on: [danmaku]
            do: [{type: log}]
          - name: 重名
            on: [gift]
            do: [{type: log}]
`
	if _, err := Parse([]byte(y)); err == nil {
		t.Error("同一绑定内规则重名应当报错")
	}
}

func TestParseRejectsDuplicateAccountName(t *testing.T) {
	y := `
accounts:
  - name: 重名账号
    cookieFile: a.txt
    rooms: [{id: "1", rules: [{name: R, on: [danmaku], do: [{type: log}]}]}]
  - name: 重名账号
    cookieFile: b.txt
    rooms: [{id: "2", rules: [{name: R, on: [danmaku], do: [{type: log}]}]}]
`
	if _, err := Parse([]byte(y)); err == nil {
		t.Error("账号重名应当报错")
	}
}

func TestParseRejectsDuplicateRoomWithinAccount(t *testing.T) {
	y := `
accounts:
  - name: A
    cookieFile: a.txt
    rooms:
      - id: "1"
        rules: [{name: R1, on: [danmaku], do: [{type: log}]}]
      - id: "1"
        rules: [{name: R2, on: [danmaku], do: [{type: log}]}]
`
	if _, err := Parse([]byte(y)); err == nil {
		t.Error("同一账号下直播间重复应当报错")
	}
}

func TestParseRejectsUnknownEventType(t *testing.T) {
	y := `
accounts:
  - name: A
    cookieFile: a.txt
    rooms:
      - id: "1"
        rules:
          - name: 坏事件类型
            on: [不存在的事件]
            do: [{type: log}]
`
	_, err := Parse([]byte(y))
	if err == nil {
		t.Fatal("未知事件类型应当报错")
	}
	if !strings.Contains(err.Error(), "事件类型") {
		t.Errorf("错误信息应指出问题所在，实际: %v", err)
	}
}

func TestParseErrorMentionsBinding(t *testing.T) {
	// 出错时要能定位到是哪个账号的哪个房间
	y := `
accounts:
  - name: 小号
    cookieFile: a.txt
    rooms:
      - id: "1706666491"
        rules:
          - name: 坏规则
            do: [{type: log}]
`
	_, err := Parse([]byte(y))
	if err == nil {
		t.Fatal("应当报错")
	}
	if !strings.Contains(err.Error(), "小号") || !strings.Contains(err.Error(), "1706666491") {
		t.Errorf("错误信息应含账号名与房间号，实际: %v", err)
	}
}

func TestParseRejectsInvalidRule(t *testing.T) {
	wrap := func(ruleYAML string) string {
		return "accounts:\n  - name: A\n    cookieFile: a.txt\n    rooms:\n      - id: \"1\"\n        rules:\n" + ruleYAML
	}
	cases := map[string]string{
		"既无 on 也无 schedule": wrap("          - name: 无触发\n            do: [{type: log}]\n"),
		"on 与 schedule 并存":  wrap("          - name: 双触发\n            on: [danmaku]\n            schedule: \"0 * * * * *\"\n            do: [{type: log}]\n"),
		"动作列表为空":            wrap("          - name: 无动作\n            on: [danmaku]\n"),
		"未知操作符":             wrap("          - name: 坏操作符\n            on: [danmaku]\n            when: {field: text, op: 不存在, value: x}\n            do: [{type: log}]\n"),
		"danmaku 缺模板":       wrap("          - name: 缺模板\n            on: [danmaku]\n            do: [{type: danmaku}]\n"),
		"非法 cron":           wrap("          - name: 坏 cron\n            schedule: 不是表达式\n            do: [{type: log}]\n"),
		"非法正则":              wrap("          - name: 坏正则\n            on: [danmaku]\n            when: {field: text, op: regex, value: \"([(\"}\n            do: [{type: log}]\n"),
	}
	for name, y := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(y)); err == nil {
				t.Error("应当报错")
			}
		})
	}
}

func TestParseRejectsAccountWithoutCookieFile(t *testing.T) {
	y := `
accounts:
  - name: A
    rooms: [{id: "1", rules: [{name: R, on: [danmaku], do: [{type: log}]}]}]
`
	if _, err := Parse([]byte(y)); err == nil {
		t.Error("账号缺少 cookieFile 应当报错")
	}
}

func TestParseRejectsAccountWithoutRooms(t *testing.T) {
	y := `
accounts:
  - name: A
    cookieFile: a.txt
`
	if _, err := Parse([]byte(y)); err == nil {
		t.Error("账号未配置任何直播间应当报错")
	}
}

func TestParseRejectsEmptyConfig(t *testing.T) {
	if _, err := Parse([]byte("accounts: []")); err == nil {
		t.Error("没有任何账号应当报错")
	}
}

func TestParseRejectsBadYAML(t *testing.T) {
	if _, err := Parse([]byte("这不是: [合法的 YAML")); err == nil {
		t.Error("非法 YAML 应当报错")
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(fullYAML), 0o644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}

	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if len(c.Accounts) != 2 {
		t.Errorf("账号数 = %d", len(c.Accounts))
	}
}

func TestLoadMissingFileReportsPath(t *testing.T) {
	_, err := Load("/不存在的路径/config.yaml")
	if err == nil {
		t.Fatal("文件不存在应当报错")
	}
	if !strings.Contains(err.Error(), "config.yaml") {
		t.Errorf("错误信息应含路径，实际: %v", err)
	}
}

func TestDurationParsing(t *testing.T) {
	y := `
accounts:
  - name: A
    cookieFile: a.txt
    rateLimit: 1.5s
    rooms:
      - id: "1"
        cooldownGroups:
          a: 500ms
          b: 2m
        rules:
          - name: 规则
            on: [danmaku]
            cooldown: 1h
            do: [{type: log}]
`
	c, err := Parse([]byte(y))
	if err != nil {
		t.Fatalf("Parse 失败: %v", err)
	}
	if time.Duration(c.Accounts[0].RateLimit) != 1500*time.Millisecond {
		t.Errorf("rateLimit = %v", time.Duration(c.Accounts[0].RateLimit))
	}
	g := c.Accounts[0].Rooms[0].CooldownGroups
	if time.Duration(g["a"]) != 500*time.Millisecond {
		t.Errorf("a = %v", time.Duration(g["a"]))
	}
	if time.Duration(g["b"]) != 2*time.Minute {
		t.Errorf("b = %v", time.Duration(g["b"]))
	}
	if c.Accounts[0].Rooms[0].Rules[0].Cooldown != time.Hour {
		t.Errorf("cooldown = %v", c.Accounts[0].Rooms[0].Rules[0].Cooldown)
	}
}

func TestDurationRejectsBadFormat(t *testing.T) {
	y := `
accounts:
  - name: A
    cookieFile: a.txt
    rateLimit: 不是时长
    rooms:
      - id: "1"
        rules: [{name: R, on: [danmaku], do: [{type: log}]}]
`
	if _, err := Parse([]byte(y)); err == nil {
		t.Error("非法时长格式应当报错")
	}
}

func TestBindingsFlattens(t *testing.T) {
	// Bindings 把三层结构摊平成运行单元列表
	c, err := Parse([]byte(fullYAML))
	if err != nil {
		t.Fatalf("Parse 失败: %v", err)
	}
	bs := c.Bindings()
	if len(bs) != 3 {
		t.Fatalf("绑定数 = %d, 期望 3（主播号-1706666491、小号-1706666491、小号-22222222）", len(bs))
	}
	labels := make([]string, 0, len(bs))
	for _, b := range bs {
		labels = append(labels, b.AccountName+"@"+b.RoomID)
	}
	want := []string{"主播号@1706666491", "小号@1706666491", "小号@22222222"}
	for i := range want {
		if labels[i] != want[i] {
			t.Errorf("绑定[%d] = %q, 期望 %q", i, labels[i], want[i])
		}
	}
	if len(bs[0].Rules) != 1 || bs[0].Rules[0].Name != "广告禁言" {
		t.Errorf("第一个绑定的规则 = %v", bs[0].Rules)
	}
}
