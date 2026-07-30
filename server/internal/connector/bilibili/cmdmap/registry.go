// Package cmdmap 负责把 B 站直播 CMD 消息映射为归一化事件。
//
// 每个 CMD 的映射逻辑放在独立文件中，通过 init() 注册到全局表，
// 新增 CMD 只需新增文件，无需修改既有代码。
package cmdmap

import (
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

// Context 是映射过程所需的上下文。
type Context struct {
	RoomID     string    // 当前直播间号
	ReceivedAt time.Time // 本地接收时间
}

// Mapper 把一条原始 CMD JSON 映射为零个或多个归一化事件。
// 返回空切片表示该消息被有意忽略。
type Mapper func(ctx Context, raw json.RawMessage) ([]event.Event, error)

var (
	mu       sync.RWMutex
	registry = make(map[string]Mapper)
)

// Register 注册一个 CMD 的映射函数。重复注册会覆盖旧值。
// 约定在各 CMD 文件的 init() 中调用。
func Register(cmd string, m Mapper) {
	mu.Lock()
	defer mu.Unlock()
	registry[cmd] = m
}

// CommandOf 提取消息的 CMD 名，并剥离形如 ":4:0:2:2:2:0" 的后缀。
func CommandOf(raw json.RawMessage) string {
	var probe struct {
		Cmd string `json:"cmd"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return ""
	}
	if i := strings.IndexByte(probe.Cmd, ':'); i >= 0 {
		return probe.Cmd[:i]
	}
	return probe.Cmd
}

// Map 把一条原始 CMD JSON 映射为归一化事件。
// 未注册的 CMD 一律产出 TypeUnknown 事件，绝不丢弃。
func Map(ctx Context, raw json.RawMessage) ([]event.Event, error) {
	name := CommandOf(raw)

	mu.RLock()
	m, ok := registry[name]
	mu.RUnlock()

	if !ok {
		return mapUnknown(ctx, name, raw), nil
	}
	return m(ctx, raw)
}

// NewEvent 构造一个填好公共字段的事件。
// ts 为零值时回落到 ctx.ReceivedAt。
func NewEvent(ctx Context, t event.Type, ts time.Time, p event.Payload, raw json.RawMessage) event.Event {
	if ts.IsZero() {
		ts = ctx.ReceivedAt
	}
	// 复制一份 raw，避免上游复用底层缓冲区导致数据被覆写。
	rawCopy := make(json.RawMessage, len(raw))
	copy(rawCopy, raw)

	return event.Event{
		ID:         event.NewID(),
		RoomID:     ctx.RoomID,
		Platform:   event.PlatformBilibili,
		Type:       t,
		Timestamp:  ts,
		ReceivedAt: ctx.ReceivedAt,
		Payload:    p,
		Raw:        rawCopy,
	}
}
