package httpapi

import (
	"sync"
	"testing"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

func danmakuEvent(text string) event.Event {
	return event.Event{
		ID:      event.NewID(),
		Type:    event.TypeDanmaku,
		Payload: event.Danmaku{Text: text},
	}
}

func TestHubDeliversToSubscriber(t *testing.T) {
	h := NewHub()
	ch, cancel := h.Subscribe(7)
	defer cancel()

	h.Publish(7, danmakuEvent("你好"))

	select {
	case ev := <-ch:
		if ev.Type != event.TypeDanmaku {
			t.Errorf("事件类型 = %s", ev.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("等待事件超时")
	}
}

// 每个绑定是独立的主题，甲房间的弹幕不该出现在乙房间的订阅里
func TestHubIsolatesBindings(t *testing.T) {
	h := NewHub()
	ch1, c1 := h.Subscribe(1)
	defer c1()
	ch2, c2 := h.Subscribe(2)
	defer c2()

	h.Publish(1, danmakuEvent("给绑定一的"))

	select {
	case <-ch1:
	case <-time.After(time.Second):
		t.Fatal("绑定一应收到事件")
	}
	select {
	case ev := <-ch2:
		t.Errorf("绑定二不该收到: %+v", ev)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestHubMultipleSubscribers(t *testing.T) {
	h := NewHub()
	ch1, c1 := h.Subscribe(7)
	defer c1()
	ch2, c2 := h.Subscribe(7)
	defer c2()

	h.Publish(7, danmakuEvent("广播"))

	for i, ch := range []<-chan event.Event{ch1, ch2} {
		select {
		case <-ch:
		case <-time.After(time.Second):
			t.Fatalf("第 %d 个订阅者没收到", i+1)
		}
	}
}

func TestHubCancelStopsDelivery(t *testing.T) {
	h := NewHub()
	ch, cancel := h.Subscribe(7)
	cancel()

	h.Publish(7, danmakuEvent("退订之后"))

	select {
	case _, ok := <-ch:
		if ok {
			t.Error("退订后不该再收到事件")
		}
	case <-time.After(50 * time.Millisecond):
	}

	if n := h.SubscriberCount(7); n != 0 {
		t.Errorf("退订后订阅者数 = %d, 期望 0", n)
	}
}

func TestHubCancelIsIdempotent(t *testing.T) {
	h := NewHub()
	_, cancel := h.Subscribe(7)
	cancel()
	cancel() // 不应 panic
}

// Publish 绝不能阻塞：它跑在机器人的事件循环上
func TestHubPublishNeverBlocks(t *testing.T) {
	h := NewHub()
	_, cancel := h.Subscribe(7) // 故意不读这个通道
	defer cancel()

	done := make(chan struct{})
	go func() {
		for i := 0; i < 10000; i++ {
			h.Publish(7, danmakuEvent("洪水"))
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Publish 阻塞了：它跑在机器人的事件循环上，绝不能阻塞")
	}
}

func TestHubPublishWithNoSubscribers(t *testing.T) {
	h := NewHub()
	h.Publish(7, danmakuEvent("没人听")) // 不应 panic
}

func TestHubConcurrentSubscribeAndPublish(t *testing.T) {
	h := NewHub()
	var wg sync.WaitGroup

	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				ch, cancel := h.Subscribe(int64(g))
				h.Publish(int64(g), danmakuEvent("并发"))
				select {
				case <-ch:
				default:
				}
				cancel()
			}
		}(g)
	}
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				h.Publish(int64(g), danmakuEvent("并发发布"))
			}
		}(g)
	}
	wg.Wait()
}
