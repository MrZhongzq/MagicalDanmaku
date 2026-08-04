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

	// Blacklist/Unblacklist 是账号级拉黑/取消拉黑，与 Block/Unblock
	// （房间级禁言/解禁）是两条独立的能力——P5-6 的明确要求，见
	// connector.Actions.BlacklistUser 的注释。
	Blacklist(ctx context.Context, uid string) error
	Unblacklist(ctx context.Context, uid string) error
	// BlacklistStatus 回读当前是否已拉黑，用于让界面显示真实状态而不是
	// "发了请求所以大概成功了"。nickname 是尽力而为的自动回填，查不到
	// 时留空，不影响 blacklisted 的准确性。
	BlacklistStatus(ctx context.Context, uid string) (blacklisted bool, nickname string, err error)
	// Nickname 查询 uid 的昵称，用于「加入禁言名单」时自动回填。
	Nickname(ctx context.Context, uid string) (string, error)

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

// put 登记单个绑定，不影响表里的其余条目。
func (rr *runtimeRegistry) put(id int64, rt BindingRuntime) {
	rr.mu.Lock()
	if rr.m == nil {
		rr.m = make(map[int64]BindingRuntime)
	}
	rr.m[id] = rt
	rr.mu.Unlock()
}

// remove 摘除单个绑定，不影响表里的其余条目。
func (rr *runtimeRegistry) remove(id int64) {
	rr.mu.Lock()
	delete(rr.m, id)
	rr.mu.Unlock()
}

// SetRuntime 用整张表替换注册表。run 在启动完全部绑定后调用一次——
// 这一刻全部绑定的运行时都已就绪，一次性交付比逐个 PutRuntime 更直接。
func (s *Server) SetRuntime(rt map[int64]BindingRuntime) { s.runtime.set(rt) }

// PutRuntime 登记单个绑定的运行期能力，不影响其余绑定。
//
// 与 SetRuntime（启动时整表交付一次）是两条不同的路径：新增/启用一个
// 绑定发生在运行期，此时其余绑定已经在跑，绝不能用一次 SetRuntime
// 把它们全部替换掉——那样会把尚未来得及重新登记的绑定短暂地从表里
// 抹掉，造成误报的「未在运行」。
func (s *Server) PutRuntime(id int64, rt BindingRuntime) { s.runtime.put(id, rt) }

// RemoveRuntime 从注册表摘除单个绑定。停用/删除一个绑定时调用，
// 之后针对它的即时动作请求会如实收到「未在运行」而不是命中一个
// 已经拆除掉的运行时。
func (s *Server) RemoveRuntime(id int64) { s.runtime.remove(id) }

// BindingLifecycle 是绑定生命周期的管理能力：新增/启用一个绑定时建连接、
// 装配规则引擎、注册定时任务并登记进运行时注册表；停用/删除时反向拆除。
//
// httpapi 自己不知道怎么连 B 站——这由 cmd/magicd 的 runtimeManager 实现，
// 通过 SetBindingLifecycle 注入。绑定的增删启停 handler 只管把数据库
// 状态改对，然后调这个接口把「让改动在运行期生效」这件事交出去；
// 生产环境里它不应为 nil（httpapi.New 时机器人进程总会注入一个），
// 但测试可以不设，处理器据此判空跳过（见各 handler 里的 nil 检查）。
type BindingLifecycle interface {
	// StartBinding 为 bindingID 建立运行时。已经在跑的绑定重复调用是
	// 幂等的（直接返回 nil）。
	//
	// 失败（账号异常、网络不通、规则非法等）不应该连带回滚数据库改动：
	// 数据库状态是唯一真相，调用方只需要记日志，不能让用户的这次
	// 增/删/启/停操作因为运行时暂时没跟上而报错——这与「热重载失败时
	// 旧引擎照跑、不拖累其余绑定」是同一个原则。
	StartBinding(ctx context.Context, bindingID int64) error

	// StopBinding 反向拆除 bindingID 的运行时：停连接、注销定时任务、
	// 结算引擎未决的合并窗口、从注册表摘除。已经不在跑的绑定重复调用
	// 是幂等的（直接返回，不做任何事），因此不返回 error——调用方没有
	// 需要处理的失败态。
	StopBinding(ctx context.Context, bindingID int64)
}

// SetBindingLifecycle 注入绑定生命周期管理能力。run 在装配完成后调用一次。
func (s *Server) SetBindingLifecycle(lc BindingLifecycle) { s.lifecycle = lc }

// runtimeFor 取运行期能力，取不到时已写过 503 响应。
func (s *Server) runtimeFor(w http.ResponseWriter, bindingID int64, label string) (BindingRuntime, bool) {
	rt, ok := s.runtime.get(bindingID)
	if !ok {
		// 503 而非 404：绑定是存在的，只是当前没在跑
		respondError(w, http.StatusServiceUnavailable,
			"绑定 %s 当前未在运行，可能是机器人尚未启动，或该绑定被停用；"+
				"若只是改了规则，保存后点「重载」即可，不需要重启", label)
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

	// 重载成功，只更新**这一个**绑定的配置版本。绝不能调
	// s.CurrentConfigHash + s.SetConfigHash 去重算并整体写回全部
	// 绑定的哈希——那等于「按绑定重载却按全库重算」：只重载了 b，
	// 若把其余绑定的哈希也跟着刷新成「当前」，其余绑定明明还在跑
	// 旧引擎，却会被判定成不再 stale。
	if h, err := s.currentBindingHash(r.Context(), b); err == nil {
		s.SetBindingConfigHash(b.ID, h)
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
