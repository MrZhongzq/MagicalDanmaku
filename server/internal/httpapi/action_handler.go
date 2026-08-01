package httpapi

import (
	"context"
	"net/http"
	"strings"
	"sync"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector"
)

// defaultBlockHours 是未指定时的禁言小时数。
const defaultBlockHours = 1

// BindingRuntime 是某个绑定在运行期的能力，由 run 注入。
//
// httpapi 只认这个接口，不依赖 bilibili 包——测试因此可以注入假实现，
// 不必真连 B 站。
type BindingRuntime interface {
	SendDanmaku(ctx context.Context, text string) error
	Block(ctx context.Context, uid string, hours int) error
	Unblock(ctx context.Context, uid string) error
	State() connector.State

	// Reload 用数据库里当前的配置重建这个绑定的规则引擎。
	//
	// **不重建连接**——用户改的只是规则，重连要重新握手、重新拉房间
	// 信息，还会丢掉这期间的弹幕。
	//
	// 新引擎构造失败时必须保持旧引擎继续跑并返回错误：保存了一份非法
	// 规则不该把机器人搞停。
	Reload(ctx context.Context) error
}

// runtimeRegistry 是绑定 ID 到运行期能力的映射。
//
// run 在启动完成后一次性写入，之后只读。用读写锁而非直接换 map，
// 是为了让「机器人还没启动完，网页就来发弹幕」这个窗口有确定行为。
type runtimeRegistry struct {
	mu sync.RWMutex
	m  map[int64]BindingRuntime
}

func newRuntimeRegistry() *runtimeRegistry {
	return &runtimeRegistry{m: make(map[int64]BindingRuntime)}
}

func (rr *runtimeRegistry) set(m map[int64]BindingRuntime) {
	rr.mu.Lock()
	rr.m = m
	rr.mu.Unlock()
}

func (rr *runtimeRegistry) get(id int64) (BindingRuntime, bool) {
	rr.mu.RLock()
	defer rr.mu.RUnlock()
	rt, ok := rr.m[id]
	return rt, ok
}

func (rr *runtimeRegistry) all() map[int64]BindingRuntime {
	rr.mu.RLock()
	defer rr.mu.RUnlock()
	out := make(map[int64]BindingRuntime, len(rr.m))
	for k, v := range rr.m {
		out[k] = v
	}
	return out
}

// SetRuntime 注入运行期能力。run 在启动完全部绑定后调用一次。
func (s *Server) SetRuntime(rt map[int64]BindingRuntime) { s.runtime.set(rt) }

// runtimeFor 取运行期能力，取不到时已写过 503 响应。
func (s *Server) runtimeFor(w http.ResponseWriter, bindingID int64, label string) (BindingRuntime, bool) {
	rt, ok := s.runtime.get(bindingID)
	if !ok {
		// 503 而非 404：绑定是存在的，只是当前没在跑
		respondError(w, http.StatusServiceUnavailable,
			"绑定 %s 当前未在运行，可能是机器人尚未启动、该绑定被停用，或改动后还没重启", label)
		return nil, false
	}
	return rt, true
}

func (s *Server) handleSendDanmaku(w http.ResponseWriter, r *http.Request) {
	b := bindingFrom(r.Context())

	var req struct {
		Text string `json:"text"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	text := strings.TrimSpace(req.Text)
	if text == "" {
		respondError(w, http.StatusUnprocessableEntity, "弹幕内容不能为空")
		return
	}

	rt, ok := s.runtimeFor(w, b.ID, b.Label())
	if !ok {
		return
	}
	// 超长会由 Actions 自动切分成多条依次发送，这里不做截断
	if err := rt.SendDanmaku(r.Context(), text); err != nil {
		s.log.Warn("手动发弹幕失败", "binding", b.Label(), "err", err)
		respondError(w, http.StatusBadGateway, "发送失败: %v", err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleBlockUser(w http.ResponseWriter, r *http.Request) {
	b := bindingFrom(r.Context())

	var req struct {
		UID   string `json:"uid"`
		Hours int    `json:"hours"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	uid := strings.TrimSpace(req.UID)
	if uid == "" {
		respondError(w, http.StatusUnprocessableEntity, "UID 不能为空")
		return
	}
	if req.Hours <= 0 {
		req.Hours = defaultBlockHours
	}

	rt, ok := s.runtimeFor(w, b.ID, b.Label())
	if !ok {
		return
	}
	if err := rt.Block(r.Context(), uid, req.Hours); err != nil {
		s.log.Warn("手动禁言失败", "binding", b.Label(), "uid", uid, "err", err)
		respondError(w, http.StatusBadGateway, "禁言失败: %v", err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"uid": uid, "hours": req.Hours})
}

func (s *Server) handleUnblockUser(w http.ResponseWriter, r *http.Request) {
	b := bindingFrom(r.Context())

	var req struct {
		UID string `json:"uid"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	uid := strings.TrimSpace(req.UID)
	if uid == "" {
		respondError(w, http.StatusUnprocessableEntity, "UID 不能为空")
		return
	}

	rt, ok := s.runtimeFor(w, b.ID, b.Label())
	if !ok {
		return
	}
	if err := rt.Unblock(r.Context(), uid); err != nil {
		s.log.Warn("手动解禁失败", "binding", b.Label(), "uid", uid, "err", err)
		respondError(w, http.StatusBadGateway, "解除禁言失败: %v", err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"uid": uid})
}

// handleReload 让这个绑定用数据库里当前的配置重建规则引擎。
//
// 显式触发而不是监听文件或轮询——用户的要求是「改完按保存才生效」。
func (s *Server) handleReload(w http.ResponseWriter, r *http.Request) {
	b := bindingFrom(r.Context())

	rt, ok := s.runtimeFor(w, b.ID, b.Label())
	if !ok {
		return
	}

	if err := rt.Reload(r.Context()); err != nil {
		// 规则非法是最常见的失败，属于调用者的输入问题；旧引擎仍在跑，
		// 机器人没有停。422 而不是 500，文案要带上具体原因——
		// 「哪条规则错了」正是操作者按下保存后要看的
		s.log.Warn("热重载失败", "binding", b.Label(), "err", err)
		respondError(w, http.StatusUnprocessableEntity,
			"重载失败，仍在用上一份配置运行: %v", err)
		return
	}

	// 重载成功，配置版本就不再是「待重载」了
	if h, err := s.CurrentConfigHash(r.Context()); err == nil {
		s.SetConfigHash(h)
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
