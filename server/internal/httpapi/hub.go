package httpapi

import (
	"sync"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

// subscriberBuffer 是每个订阅者的缓冲深度。
//
// 256 条足够覆盖网络抖动与浏览器渲染的短暂落后；再满就丢弃，
// 不回压到机器人的事件循环上。
const subscriberBuffer = 256

// Hub 把机器人收到的事件扇出给 SSE 订阅者。
//
// **Publish 永不阻塞。** 订阅者的通道满了就丢弃它的这条事件——
// 网页看丢一条弹幕可以接受，拖慢规则处理不行。这与 ActivityWriter
// 的丢弃策略同源，理由也一样。
type Hub struct {
	mu     sync.RWMutex
	nextID int64
	// topics[bindingID][subscriberID] = 该订阅者的通道
	topics map[int64]map[int64]chan event.Event
}

// NewHub 创建事件中枢。
func NewHub() *Hub {
	return &Hub{topics: make(map[int64]map[int64]chan event.Event)}
}

// Publish 把事件扇出给某个绑定的全部订阅者。永不阻塞。
func (h *Hub) Publish(bindingID int64, ev event.Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, ch := range h.topics[bindingID] {
		select {
		case ch <- ev:
		default:
			// 该订阅者跟不上，丢弃它的这条。不记日志：
			// 一个卡住的浏览器会瞬间刷屏，日志本身就成了新问题。
		}
	}
}

// Subscribe 订阅某个绑定的事件流，返回通道与退订函数。
//
// 退订函数可重复调用，且必须被调用——否则订阅者会一直留在表里，
// Publish 每次都往一个没人读的通道里塞。
func (h *Hub) Subscribe(bindingID int64) (<-chan event.Event, func()) {
	ch := make(chan event.Event, subscriberBuffer)

	h.mu.Lock()
	h.nextID++
	id := h.nextID
	if h.topics[bindingID] == nil {
		h.topics[bindingID] = make(map[int64]chan event.Event)
	}
	h.topics[bindingID][id] = ch
	h.mu.Unlock()

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			h.mu.Lock()
			if subs := h.topics[bindingID]; subs != nil {
				delete(subs, id)
				if len(subs) == 0 {
					delete(h.topics, bindingID)
				}
			}
			h.mu.Unlock()
			// 关闭通道让读端的 range 结束。在持锁删除之后关，
			// 保证 Publish 不可能往已关闭的通道里发。
			close(ch)
		})
	}
	return ch, cancel
}

// SubscriberCount 返回某绑定当前的订阅者数，供监控与测试使用。
func (h *Hub) SubscriberCount(bindingID int64) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.topics[bindingID])
}
