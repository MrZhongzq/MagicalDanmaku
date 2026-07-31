// Package config 负责从 YAML 加载并校验规则配置。
//
// 配置结构与运行模型同构，三层嵌套：账号 → 直播间 → 规则。
// 每个「账号-直播间」组合是一个独立的运行单元（绑定）。
//
// 配置格式只是一种序列化。规则模型本身存储无关，P3 迁进数据库时
// 核心逻辑不动，只换加载器。
package config

import (
	"fmt"
	"os"
	"regexp"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/scheduler"
)

// Duration 包装 time.Duration，支持 YAML 中的 "1.5s" 形式。
type Duration time.Duration

// UnmarshalYAML 解析形如 "1.5s"、"500ms"、"2m" 的时长字符串。
func (d *Duration) UnmarshalYAML(n *yaml.Node) error {
	var s string
	if err := n.Decode(&s); err != nil {
		return fmt.Errorf("时长必须是字符串，如 \"1.5s\": %w", err)
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("非法的时长 %q: %w", s, err)
	}
	*d = Duration(parsed)
	return nil
}

// Config 是完整配置。
type Config struct {
	Accounts []Account
}

// Account 是一个已登录账号及其连接的全部直播间。
type Account struct {
	Name       string
	CookieFile string
	RateLimit  Duration // 该账号全部直播间共享的发送间隔
	MaxLength  int      // 单条弹幕字符上限，0 表示用默认值
	Rooms      []Room
}

// Room 是某个账号在某个直播间的配置，即一个绑定。
type Room struct {
	ID             string
	CooldownGroups map[string]Duration
	Rules          []rules.Rule
}

// Binding 是摊平后的运行单元。
type Binding struct {
	AccountName    string
	CookieFile     string
	RateLimit      time.Duration
	MaxLength      int
	RoomID         string
	CooldownGroups map[string]time.Duration
	Rules          []rules.Rule
}

// Bindings 把三层结构摊平成运行单元列表，保持配置顺序。
func (c *Config) Bindings() []Binding {
	var out []Binding
	for _, a := range c.Accounts {
		for _, r := range a.Rooms {
			groups := make(map[string]time.Duration, len(r.CooldownGroups))
			for k, v := range r.CooldownGroups {
				groups[k] = time.Duration(v)
			}
			out = append(out, Binding{
				AccountName:    a.Name,
				CookieFile:     a.CookieFile,
				RateLimit:      time.Duration(a.RateLimit),
				MaxLength:      a.MaxLength,
				RoomID:         r.ID,
				CooldownGroups: groups,
				Rules:          r.Rules,
			})
		}
	}
	return out
}

// ---- 以下为 YAML 线上格式，与领域模型分离 ----

type configYAML struct {
	Accounts []accountYAML `yaml:"accounts"`
}

type accountYAML struct {
	Name       string     `yaml:"name"`
	CookieFile string     `yaml:"cookieFile"`
	RateLimit  Duration   `yaml:"rateLimit"`
	MaxLength  int        `yaml:"maxLength"`
	Rooms      []roomYAML `yaml:"rooms"`
}

type roomYAML struct {
	ID             string              `yaml:"id"`
	CooldownGroups map[string]Duration `yaml:"cooldownGroups"`
	Rules          []ruleYAML          `yaml:"rules"`
}

type ruleYAML struct {
	Name          string         `yaml:"name"`
	Enabled       *bool          `yaml:"enabled"` // 指针以区分「未写」与「写了 false」
	On            []string       `yaml:"on"`
	Schedule      string         `yaml:"schedule"`
	When          *conditionYAML `yaml:"when"`
	Aggregate     *aggregateYAML `yaml:"aggregate"`
	Cooldown      Duration       `yaml:"cooldown"`
	CooldownGroup string         `yaml:"cooldownGroup"`
	Do            []actionYAML   `yaml:"do"`
}

type conditionYAML struct {
	Field  string          `yaml:"field"`
	Op     string          `yaml:"op"`
	Value  any             `yaml:"value"`
	All    []conditionYAML `yaml:"all"`
	Any    []conditionYAML `yaml:"any"`
	Not    *conditionYAML  `yaml:"not"`
	Script string          `yaml:"script"`
}

type aggregateYAML struct {
	Window   Duration `yaml:"window"`
	MaxWait  Duration `yaml:"maxWait"`
	MinCount int      `yaml:"minCount"`
	By       string   `yaml:"by"`
}

type actionYAML struct {
	Type     string   `yaml:"type"`
	Template []string `yaml:"template"`
	Hours    int      `yaml:"hours"`
	Script   string   `yaml:"script"`
}

// opAliases 把用户友好的符号归一化为内部枚举名。
//
// 用户不该被迫记住内部枚举名——写 ">" 比写 "gt" 自然得多。
var opAliases = map[string]string{
	">": "gt", ">=": "gte", "<": "lt", "<=": "lte",
	"==": "eq", "=": "eq", "!=": "ne", "<>": "ne",
}

// knownEventTypes 是允许出现在 on 字段中的事件类型。
var knownEventTypes = map[string]event.Type{
	string(event.TypeDanmaku):          event.TypeDanmaku,
	string(event.TypeSuperChat):        event.TypeSuperChat,
	string(event.TypeSuperChatDelete):  event.TypeSuperChatDelete,
	string(event.TypeGift):             event.TypeGift,
	string(event.TypeGiftCombo):        event.TypeGiftCombo,
	string(event.TypeGuardBuy):         event.TypeGuardBuy,
	string(event.TypeUserEnter):        event.TypeUserEnter,
	string(event.TypeUserFollow):       event.TypeUserFollow,
	string(event.TypeUserShare):        event.TypeUserShare,
	string(event.TypeUserLike):         event.TypeUserLike,
	string(event.TypeLiveStart):        event.TypeLiveStart,
	string(event.TypeLiveStop):         event.TypeLiveStop,
	string(event.TypeRoomChange):       event.TypeRoomChange,
	string(event.TypeUserBlocked):      event.TypeUserBlocked,
	string(event.TypeOnlineRankUpdate): event.TypeOnlineRankUpdate,
	string(event.TypeRoomStatsUpdate):  event.TypeRoomStatsUpdate,
	string(event.TypeBattle):           event.TypeBattle,
	string(event.TypeUnknown):          event.TypeUnknown,
}

// Load 从文件加载配置。
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: 读取配置文件 %s 失败: %w", path, err)
	}
	c, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("config: 解析 %s 失败: %w", path, err)
	}
	return c, nil
}

// Parse 解析配置内容并做完整校验。
//
// 校验失败即报错退出，不允许带病运行——配置写错却静默忽略，
// 比直接报错更难排查。
func Parse(data []byte) (*Config, error) {
	var raw configYAML
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("YAML 语法错误: %w", err)
	}
	if len(raw.Accounts) == 0 {
		return nil, fmt.Errorf("配置中没有任何账号")
	}

	c := &Config{}
	seenAccount := make(map[string]bool, len(raw.Accounts))

	for i, ay := range raw.Accounts {
		if ay.Name == "" {
			return nil, fmt.Errorf("第 %d 个账号缺少 name", i+1)
		}
		if seenAccount[ay.Name] {
			return nil, fmt.Errorf("账号名 %q 重复", ay.Name)
		}
		seenAccount[ay.Name] = true

		if ay.CookieFile == "" {
			return nil, fmt.Errorf("账号 %q 缺少 cookieFile", ay.Name)
		}
		if len(ay.Rooms) == 0 {
			return nil, fmt.Errorf("账号 %q 未配置任何直播间", ay.Name)
		}

		acc := Account{
			Name:       ay.Name,
			CookieFile: ay.CookieFile,
			RateLimit:  ay.RateLimit,
			MaxLength:  ay.MaxLength,
		}

		seenRoom := make(map[string]bool, len(ay.Rooms))
		for j, ry := range ay.Rooms {
			if ry.ID == "" {
				return nil, fmt.Errorf("账号 %q 的第 %d 个直播间缺少 id", ay.Name, j+1)
			}
			if seenRoom[ry.ID] {
				return nil, fmt.Errorf("账号 %q 下的直播间 %s 重复配置", ay.Name, ry.ID)
			}
			seenRoom[ry.ID] = true

			room := Room{ID: ry.ID, CooldownGroups: ry.CooldownGroups}

			// 规则名只需在单个绑定内唯一——同一条「进场欢迎」本来就会
			// 出现在多个绑定下。冷却按规则名记录，同绑定内重名会互相干扰。
			seenRule := make(map[string]bool, len(ry.Rules))
			for k, rl := range ry.Rules {
				r, err := convertRule(rl)
				if err != nil {
					return nil, fmt.Errorf("账号 %q 的直播间 %s 第 %d 条规则(%s)非法: %w",
						ay.Name, ry.ID, k+1, rl.Name, err)
				}
				if seenRule[r.Name] {
					return nil, fmt.Errorf("账号 %q 的直播间 %s 下规则名 %q 重复",
						ay.Name, ry.ID, r.Name)
				}
				seenRule[r.Name] = true
				room.Rules = append(room.Rules, r)
			}

			acc.Rooms = append(acc.Rooms, room)
		}
		c.Accounts = append(c.Accounts, acc)
	}
	return c, nil
}

// convertRule 把线上格式转成领域模型并校验。
func convertRule(ry ruleYAML) (rules.Rule, error) {
	r := rules.Rule{
		Name:          ry.Name,
		Enabled:       true, // 未写 enabled 时默认启用：写了规则却不生效最反直觉
		Schedule:      ry.Schedule,
		Cooldown:      time.Duration(ry.Cooldown),
		CooldownGroup: ry.CooldownGroup,
	}
	if ry.Enabled != nil {
		r.Enabled = *ry.Enabled
	}

	for _, name := range ry.On {
		t, ok := knownEventTypes[name]
		if !ok {
			return r, fmt.Errorf("未知的事件类型 %q", name)
		}
		r.On = append(r.On, t)
	}

	if ry.Schedule != "" {
		if err := scheduler.ValidateSpec(ry.Schedule); err != nil {
			return r, err
		}
	}

	if ry.When != nil {
		c, err := convertCondition(*ry.When)
		if err != nil {
			return r, err
		}
		r.When = &c
	}

	if ry.Aggregate != nil {
		r.Aggregate = &rules.AggregateSpec{
			Window:   time.Duration(ry.Aggregate.Window),
			MaxWait:  time.Duration(ry.Aggregate.MaxWait),
			MinCount: ry.Aggregate.MinCount,
			By:       rules.AggregateBy(ry.Aggregate.By),
		}
	}

	for _, a := range ry.Do {
		r.Do = append(r.Do, rules.Action{
			Type:     rules.ActionType(a.Type),
			Template: a.Template,
			Hours:    a.Hours,
			Script:   a.Script,
		})
	}

	if err := r.Validate(); err != nil {
		return r, err
	}
	return r, nil
}

// convertCondition 递归转换条件并做正则预编译校验。
func convertCondition(cy conditionYAML) (rules.Condition, error) {
	c := rules.Condition{
		Field:  cy.Field,
		Op:     normalizeOp(cy.Op),
		Value:  cy.Value,
		Script: cy.Script,
	}

	// 正则在加载时就编译一次，把错误从运行时提前到启动时
	if c.Op == "regex" {
		pattern, _ := cy.Value.(string)
		if _, err := regexp.Compile(pattern); err != nil {
			return c, fmt.Errorf("非法的正则表达式 %q: %w", pattern, err)
		}
	}

	for _, sub := range cy.All {
		s, err := convertCondition(sub)
		if err != nil {
			return c, err
		}
		c.All = append(c.All, s)
	}
	for _, sub := range cy.Any {
		s, err := convertCondition(sub)
		if err != nil {
			return c, err
		}
		c.Any = append(c.Any, s)
	}
	if cy.Not != nil {
		s, err := convertCondition(*cy.Not)
		if err != nil {
			return c, err
		}
		c.Not = &s
	}
	return c, nil
}

// normalizeOp 把符号别名归一化为内部枚举名。
func normalizeOp(op string) string {
	if alias, ok := opAliases[op]; ok {
		return alias
	}
	return op
}
