package spec

import (
	"fmt"
	"regexp"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/scheduler"
)

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

// ToRule 把序列化形式转成领域模型并校验。
//
// 校验在这里做完，非法规则不允许进入运行期——配置写错却静默忽略，
// 比直接报错更难排查。
func (r Rule) ToRule() (rules.Rule, error) {
	out := rules.Rule{
		Name:          r.Name,
		Enabled:       true, // 未写 enabled 时默认启用：写了规则却不生效最反直觉
		Schedule:      r.Schedule,
		Cooldown:      time.Duration(r.Cooldown),
		CooldownGroup: r.CooldownGroup,
		Suppress:      r.Suppress,
	}
	if r.Enabled != nil {
		out.Enabled = *r.Enabled
	}

	for _, name := range r.On {
		t, ok := knownEventTypes[name]
		if !ok {
			return out, fmt.Errorf("未知的事件类型 %q", name)
		}
		out.On = append(out.On, t)
	}

	if r.Schedule != "" {
		if err := scheduler.ValidateSpec(r.Schedule); err != nil {
			return out, err
		}
	}

	if r.When != nil {
		c, err := r.When.ToCondition()
		if err != nil {
			return out, err
		}
		out.When = &c
	}

	if r.Aggregate != nil {
		out.Aggregate = &rules.AggregateSpec{
			Window:   time.Duration(r.Aggregate.Window),
			MinCount: r.Aggregate.MinCount,
			By:       rules.AggregateBy(r.Aggregate.By),
		}
	}

	for _, a := range r.Do {
		out.Do = append(out.Do, rules.Action{
			Type:          rules.ActionType(a.Type),
			Template:      a.Template,
			TemplateMulti: a.TemplateMulti,
			Pick:          a.Pick,
			Hours:         a.Hours,
			Script:        a.Script,
		})
	}

	if err := out.Validate(); err != nil {
		return out, err
	}
	return out, nil
}

// ToCondition 递归转换条件并做正则预编译校验。
func (c Condition) ToCondition() (rules.Condition, error) {
	out := rules.Condition{
		Field:  c.Field,
		Op:     normalizeOp(c.Op),
		Value:  c.Value,
		Script: c.Script,
	}

	// 正则在转换时就编译一次，把错误从运行时提前到启动时
	if out.Op == "regex" {
		pattern, _ := c.Value.(string)
		if _, err := regexp.Compile(pattern); err != nil {
			return out, fmt.Errorf("非法的正则表达式 %q: %w", pattern, err)
		}
	}

	for _, sub := range c.All {
		s, err := sub.ToCondition()
		if err != nil {
			return out, err
		}
		out.All = append(out.All, s)
	}
	for _, sub := range c.Any {
		s, err := sub.ToCondition()
		if err != nil {
			return out, err
		}
		out.Any = append(out.Any, s)
	}
	if c.Not != nil {
		s, err := c.Not.ToCondition()
		if err != nil {
			return out, err
		}
		out.Not = &s
	}
	return out, nil
}

// normalizeOp 把符号别名归一化为内部枚举名。
func normalizeOp(op string) string {
	if alias, ok := opAliases[op]; ok {
		return alias
	}
	return op
}
