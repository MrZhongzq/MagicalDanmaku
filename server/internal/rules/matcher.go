package rules

import (
	"log/slog"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

// Matcher 按事件类型与条件筛选命中的规则。
type Matcher struct {
	rules     []Rule
	eval      Evaluator
	log       *slog.Logger
	byType    map[event.Type][]Rule
	scheduled []Rule
}

// NewMatcher 创建匹配器。规则在此按事件类型预先分组，避免每次事件
// 都遍历全部规则——高频弹幕下这个开销不可忽略。
func NewMatcher(rs []Rule, ev Evaluator, log *slog.Logger) *Matcher {
	if log == nil {
		log = slog.Default()
	}
	m := &Matcher{
		rules:  rs,
		eval:   ev,
		log:    log,
		byType: make(map[event.Type][]Rule),
	}
	for _, r := range rs {
		if !r.Enabled {
			continue
		}
		if r.Schedule != "" {
			m.scheduled = append(m.scheduled, r)
			continue
		}
		for _, t := range r.On {
			m.byType[t] = append(m.byType[t], r)
		}
	}
	return m
}

// Match 返回命中该 Trigger 的全部规则，保持配置顺序。
//
// 单条规则的条件求值出错只跳过该规则并记日志，不影响其他规则。
func (m *Matcher) Match(tr Trigger) []Rule {
	candidates := m.byType[tr.Type]
	if len(candidates) == 0 {
		return nil
	}

	out := make([]Rule, 0, len(candidates))
	for _, r := range candidates {
		if r.When == nil {
			out = append(out, r)
			continue
		}
		ok, err := m.eval.Eval(*r.When, tr.Vars)
		if err != nil {
			m.log.Warn("规则条件求值失败，已跳过该规则",
				"rule", r.Name, "err", err)
			continue
		}
		if ok {
			out = append(out, r)
		}
	}
	return out
}

// RulesFor 返回监听指定事件类型的启用规则。
func (m *Matcher) RulesFor(t event.Type) []Rule {
	return m.byType[t]
}

// ScheduledRules 返回全部启用的定时规则。
func (m *Matcher) ScheduledRules() []Rule {
	return m.scheduled
}
