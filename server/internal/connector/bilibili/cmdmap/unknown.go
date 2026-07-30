package cmdmap

import (
	"encoding/json"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

// mapUnknown 为未注册的 CMD 产出兜底事件。
//
// 原项目遇到未知 CMD 是打日志丢弃，导致 B 站上线新功能后
// 用户必须等待客户端发版。这里改为照常投递，用户脚本可自行处理。
func mapUnknown(ctx Context, name string, raw json.RawMessage) []event.Event {
	return []event.Event{
		NewEvent(ctx, event.TypeUnknown, ctx.ReceivedAt, event.Unknown{Command: name}, raw),
	}
}
