// Package rules 实现声明式规则引擎：结构化条件匹配 + 模板/脚本动作。
package rules

import (
	"fmt"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

// ActionType 是动作类型。
type ActionType string

// 全部动作类型。
const (
	ActionDanmaku ActionType = "danmaku" // 发弹幕
	ActionBlock   ActionType = "block"   // 禁言
	ActionScript  ActionType = "script"  // 执行脚本
	ActionLog     ActionType = "log"     // 只记日志，用于调试规则
)

// AggregateBy 是合并窗口的分组键。
type AggregateBy string

// 全部分组方式。
const (
	AggregateByType AggregateBy = "type" // 按事件类型：窗口内全部合成一条
	AggregateByUser AggregateBy = "user" // 按类型+UID：仅去重不聚合
	AggregateByGift AggregateBy = "gift" // 按类型+UID+礼物名：数量累加
)

// validOps 是条件支持的全部操作符。
var validOps = map[string]bool{
	"eq": true, "ne": true,
	"gt": true, "gte": true, "lt": true, "lte": true,
	"contains": true, "prefix": true, "suffix": true, "regex": true,
	"in": true,
}

// Rule 是一条规则。
//
// 触发方式二选一：On 由事件驱动，Schedule 由 cron 驱动。
type Rule struct {
	Name          string         // 规则名，用于日志与去重，必填
	Enabled       bool           // 是否启用
	On            []event.Type   // 事件触发：监听的事件类型
	Schedule      string         // 定时触发：6 字段 cron 表达式
	When          *Condition     // 过滤条件，nil 表示无条件
	Aggregate     *AggregateSpec // 合并窗口，nil 表示不合并
	Do            []Action       // 动作列表，按序执行
	Cooldown      time.Duration  // 本规则最小触发间隔
	CooldownGroup string         // 命名冷却组，可空
}

// Validate 校验规则自身的完整性。
func (r Rule) Validate() error {
	if r.Name == "" {
		return fmt.Errorf("rules: 规则名不能为空")
	}

	hasOn := len(r.On) > 0
	hasSchedule := r.Schedule != ""
	if hasOn && hasSchedule {
		return fmt.Errorf("rules: 规则 %q 的 on 与 schedule 互斥，不能同时指定", r.Name)
	}
	if !hasOn && !hasSchedule {
		return fmt.Errorf("rules: 规则 %q 必须指定 on 或 schedule 之一", r.Name)
	}

	if len(r.Do) == 0 {
		return fmt.Errorf("rules: 规则 %q 的动作列表不能为空", r.Name)
	}
	for i, a := range r.Do {
		if err := a.Validate(); err != nil {
			return fmt.Errorf("rules: 规则 %q 的第 %d 个动作非法: %w", r.Name, i+1, err)
		}
	}

	if r.When != nil {
		if err := r.When.Validate(); err != nil {
			return fmt.Errorf("rules: 规则 %q 的条件非法: %w", r.Name, err)
		}
	}
	if r.Aggregate != nil {
		if err := r.Aggregate.Validate(); err != nil {
			return fmt.Errorf("rules: 规则 %q 的合并规格非法: %w", r.Name, err)
		}
	}
	return nil
}

// Condition 是一个条件节点。
//
// Field（叶子）、All、Any、Not、Script 五种形态互斥，只能有一个生效。
type Condition struct {
	Field string // Vars 中的路径，如 "user.guardLevel"
	Op    string // 操作符，见 validOps
	Value any    // 比较值

	All []Condition // 全部满足
	Any []Condition // 任一满足
	Not *Condition  // 取反

	Script string // JS 表达式逃生舱，须返回 boolean
}

// Validate 递归校验条件树。
func (c Condition) Validate() error {
	// 统计有几种形态被指定
	forms := 0
	if c.Field != "" {
		forms++
	}
	if len(c.All) > 0 {
		forms++
	}
	if len(c.Any) > 0 {
		forms++
	}
	if c.Not != nil {
		forms++
	}
	if c.Script != "" {
		forms++
	}

	switch {
	case forms == 0:
		return fmt.Errorf("条件为空，须指定 field / all / any / not / script 之一")
	case forms > 1:
		return fmt.Errorf("field / all / any / not / script 只能指定一个")
	}

	if c.Field != "" && !validOps[c.Op] {
		return fmt.Errorf("未知的操作符 %q", c.Op)
	}
	for i, sub := range c.All {
		if err := sub.Validate(); err != nil {
			return fmt.Errorf("all 的第 %d 项非法: %w", i+1, err)
		}
	}
	for i, sub := range c.Any {
		if err := sub.Validate(); err != nil {
			return fmt.Errorf("any 的第 %d 项非法: %w", i+1, err)
		}
	}
	if c.Not != nil {
		if err := c.Not.Validate(); err != nil {
			return fmt.Errorf("not 的子条件非法: %w", err)
		}
	}
	return nil
}

// Action 是一个动作。
type Action struct {
	Type ActionType

	// Type == ActionDanmaku 时使用，多条则随机挑一条
	Template []string

	// Type == ActionBlock 时使用，禁言小时数
	Hours int

	// Type == ActionScript 时使用
	Script string
}

// Validate 校验动作。
func (a Action) Validate() error {
	switch a.Type {
	case ActionDanmaku:
		if len(a.Template) == 0 {
			return fmt.Errorf("danmaku 动作必须提供至少一条模板")
		}
	case ActionScript:
		if a.Script == "" {
			return fmt.Errorf("script 动作必须提供脚本内容")
		}
	case ActionBlock, ActionLog:
		// 无必填字段
	default:
		return fmt.Errorf("未知的动作类型 %q", a.Type)
	}
	return nil
}

// AggregateSpec 描述如何合并窗口内的事件。
//
// 窗口是滚动的：每来一个新事件就重置静默计时，因此「3 分钟内陆续进场」
// 会被算作同一批。持续有人进场时静默期永不到来，故用 MaxWait 兜底。
type AggregateSpec struct {
	// Window 是静默时长：最后一个事件之后再等这么久就结算。
	Window time.Duration

	// MaxWait 是从首个事件起的最长等待，0 表示不设上限。
	//
	// 注意这是个陷阱：活跃房间里若 Window 设得较长（如 3 分钟）又不设
	// MaxWait，会因为总有人进场、静默期永不到来而一直不结算。
	// 房间越热闹越该设它。
	MaxWait time.Duration

	// MinCount 是启用合并所需的最小条目数。
	// 未达到时不合并，每个条目各自产出一条 Trigger。
	// 0 或 1 表示总是合并。
	MinCount int

	By AggregateBy // 分组键
}

// Validate 校验合并规格。
func (s AggregateSpec) Validate() error {
	if s.Window <= 0 {
		return fmt.Errorf("合并窗口必须大于 0")
	}
	if s.MaxWait > 0 && s.MaxWait < s.Window {
		return fmt.Errorf("maxWait(%v) 不能小于 window(%v)", s.MaxWait, s.Window)
	}
	if s.MinCount < 0 {
		return fmt.Errorf("minCount 不能为负")
	}
	switch s.By {
	case AggregateByType, AggregateByUser, AggregateByGift:
		return nil
	default:
		return fmt.Errorf("未知的分组键 %q", s.By)
	}
}
