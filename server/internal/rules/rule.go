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

// Pick 决定一个 danmaku 动作有多条模板时怎么挑。
//
// 空串或 PickRandom：随机挑一条（默认，与历史行为一致）。
// PickSequential：按顺序轮流用，到末尾回到第一条。
const (
	PickRandom     = "random"
	PickSequential = "sequential"
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

	// Suppress 列出本规则命中后要跳过的规则名。
	//
	// 典型场景：给某位舰长配了专属进房欢迎，就不该再触发通用进房欢迎，
	// 否则他进房会被欢迎两次。
	//
	// **只对同一次触发生效**，不是全局开关。**只对事件驱动（On）的规则
	// 生效**：定时（Schedule）规则一次调用只触发自己，不存在「本次触发
	// 命中的其他规则」这个集合，配了会被 Validate 拒绝。
	Suppress []string
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

	// Schedule 触发的规则配 Suppress 是无声死配置，拒绝掉。
	//
	// cron 按规则名逐条注册任务，一次调用只触发一条规则，根本不存在
	// 「本次触发命中的其他规则」这个集合可供压制——FireScheduled 也
	// 确实不消费这个字段。
	//
	// 不拦的话它能通过全部校验、运行时被彻底忽略、不报错不记日志。
	// 与「压制不存在的规则名」是同一类问题：静默不生效非常难查。
	if hasSchedule && len(r.Suppress) > 0 {
		return fmt.Errorf("rules: 规则 %q 是定时触发的，配 suppress 不会生效——"+
			"压制只在同一次事件触发命中多条规则时才有意义", r.Name)
	}

	if len(r.Do) == 0 {
		return fmt.Errorf("rules: 规则 %q 的动作列表不能为空", r.Name)
	}
	for i, a := range r.Do {
		if err := a.Validate(); err != nil {
			return fmt.Errorf("rules: 规则 %q 的第 %d 个动作非法: %w", r.Name, i+1, err)
		}

		// 只配 TemplateMulti 而没有 Aggregate 的组合必然失败，在这里拦住。
		//
		// 没有 Aggregate 的规则走 PassthroughTrigger，count 恒为 1
		// （aggregate.go），永远选不中 TemplateMulti；而 Template 是空的，
		// 每次触发都会报「模板列表为空」。
		//
		// 不拦的话用户看到的是「规则不生效」，日志里每次触发一条错误，
		// 查不到为什么——把一个配置错误推迟到了运行期。
		//
		// 反过来，只配 TemplateMulti 且*有* Aggregate 时不拦：规则可能
		// 配了 MinCount > 1，那样根本不会有单人触发，此时是合法配置
		// （用户就是只要多人合并欢迎，单人不发言），拦了会挡住它。
		if a.Type == ActionDanmaku && len(a.Template) == 0 &&
			len(a.TemplateMulti) > 0 && r.Aggregate == nil {
			return fmt.Errorf("rules: 规则 %q 的第 %d 个动作只配了 templateMulti，"+
				"但这条规则没有配 aggregate——不合并的触发永远只有一个人，"+
				"templateMulti 用不上。请改用 template，或给规则加上 aggregate",
				r.Name, i+1)
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

	// 自我压制是个无意义的死配置：压制只在「先执行的规则压制后执行的」
	// 时才生效，规则不可能压制自己。
	for _, s := range r.Suppress {
		if s == r.Name {
			return fmt.Errorf("rules: 规则 %q 不能在 suppress 中压制自己", r.Name)
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

	// Type == ActionDanmaku 时使用，多条则按 Pick 指定的方式挑一条。
	// count == 1（单人触发）时用这套。
	Template []string

	// TemplateMulti 是合并触发（count > 1）时用的模板。
	//
	// 为什么要两套：「欢迎 张三 回家」与「欢迎 张三、李四、王五 回家」
	// 句式本就不同，共用一套必然有一边别扭。
	//
	// 留空则不论单人多人都用 Template——保持与历史配置兼容。
	TemplateMulti []string

	// Type == ActionDanmaku 时使用，控制 Template/TemplateMulti 有多条时
	// 怎么挑，见 PickRandom / PickSequential。空串等同 PickRandom，
	// 与引入本字段之前的历史配置兼容。
	Pick string

	// Type == ActionBlock 时使用，禁言小时数
	Hours int

	// Type == ActionScript 时使用
	Script string
}

// Validate 校验动作。
func (a Action) Validate() error {
	switch a.Type {
	case ActionDanmaku:
		// Template 与 TemplateMulti 二选一提供即可：只配 TemplateMulti
		// 也是合法的（比如一条只处理合并欢迎、单人不发言的规则）。
		if len(a.Template) == 0 && len(a.TemplateMulti) == 0 {
			return fmt.Errorf("danmaku 动作必须提供 template 或 templateMulti 之一")
		}
		switch a.Pick {
		case "", PickRandom, PickSequential:
			// 合法取值，空串等同 PickRandom
		default:
			return fmt.Errorf("pick 取值非法 %q，合法值为 %q 或 %q", a.Pick, PickRandom, PickSequential)
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
// 窗口从本轮首个事件起算固定时长：不论期间又来多少事件，Window
// 一到就结算，不会被后续事件拖延，因此「3 分钟内陆续进场」会被
// 算作同一批，但结算时刻始终封顶在首个事件之后的 Window 时长处。
type AggregateSpec struct {
	// Window 是从首个事件起算的合并窗口时长，到期即结算。
	Window time.Duration

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
