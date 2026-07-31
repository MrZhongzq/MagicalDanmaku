package rules

import "github.com/MrZhongzq/MagicalDanmaku/server/internal/event"

// ActivitySink 接收引擎的业务动向，用于生成业务日志。
//
// 两个方法都必须**立即返回**：它们跑在事件处理的关键路径上，
// 阻塞会直接拖慢弹幕响应。实现方应当把工作丢进队列而不是原地做完。
//
// 本包不关心日志去了哪里——落库、打文件还是丢掉，是实现方的事。
type ActivitySink interface {
	// RecordEvent 报告收到一个事件。无论是否命中规则都会调用：
	// 业务日志是完整的房间流水，不是「触发过规则的事件」的子集。
	RecordEvent(ev event.Event)

	// RecordAction 报告执行了一个动作。err 非 nil 表示动作失败，
	// 失败的动作同样要记——「为什么没发出去」正是要查的。
	RecordAction(ruleName string, a Action, tr Trigger, err error)
}

// nopSink 是未配置时的空实现。
//
// 用空实现而非到处判 nil：调用点散在热路径上，每处加一个 if
// 既啰嗦又容易漏。
type nopSink struct{}

func (nopSink) RecordEvent(event.Event)                     {}
func (nopSink) RecordAction(string, Action, Trigger, error) {}
