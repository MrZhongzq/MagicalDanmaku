package rules

import "github.com/MrZhongzq/MagicalDanmaku/server/internal/event"

// TypeScheduled 是定时任务专用的伪事件类型。
//
// 定时触发也走与事件规则完全相同的匹配与执行路径，
// 因此需要一个不与真实事件冲突的类型标识。
const TypeScheduled event.Type = "scheduled"

// Trigger 是规则匹配的输入单元。
//
// 未经合并时 Events 长度为 1；经合并窗口聚合后为 N。
type Trigger struct {
	Type   event.Type     // 事件类型
	Events []event.Event  // 参与本次触发的原始事件
	Vars   map[string]any // 条件求值与模板渲染的唯一取值来源
}
