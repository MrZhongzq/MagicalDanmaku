package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sync"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules/spec"
)

// configHash 记录当前"已保存但可能还没重载"的配置版本。
//
// 热重载是显式触发的（保存后手动点重载），不是文件监听或轮询——
// 界面必须诚实地告诉用户：当前跑的引擎，是否与数据库里最新保存的
// 配置一致。
type configHash struct {
	mu sync.RWMutex
	h  string
}

func (c *configHash) set(h string) {
	c.mu.Lock()
	c.h = h
	c.mu.Unlock()
}

func (c *configHash) get() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.h
}

// SetConfigHash 记下机器人当前运行所依据的配置版本。
// run 在装配完成后、以及每次热重载成功后调用。
func (s *Server) SetConfigHash(h string) { s.cfgHash.set(h) }

// CurrentConfigHash 算当前数据库里配置的哈希。
//
// 只取影响运行行为的部分：绑定的启停、规则体、冷却组。
// 账号的 Cookie 不进哈希——换 Cookie 不需要重载（下次请求就用新的）。
func (s *Server) CurrentConfigHash(ctx context.Context) (string, error) {
	bindings, err := s.store.ListBindings(ctx)
	if err != nil {
		return "", err
	}

	type bindingShape struct {
		ID       int64            `json:"id"`
		Enabled  bool             `json:"enabled"`
		Rules    []spec.Rule      `json:"rules"`
		Cooldown map[string]int64 `json:"cooldown"`
	}
	shapes := make([]bindingShape, 0, len(bindings))

	for _, b := range bindings {
		recs, err := s.store.ListRules(ctx, b.ID)
		if err != nil {
			return "", err
		}
		rs := make([]spec.Rule, 0, len(recs))
		for _, rec := range recs {
			rs = append(rs, rec.Spec)
		}

		groups, err := s.store.CooldownGroups(ctx, b.ID)
		if err != nil {
			return "", err
		}
		cd := make(map[string]int64, len(groups))
		for k, v := range groups {
			cd[k] = v.Milliseconds()
		}

		shapes = append(shapes, bindingShape{
			ID: b.ID, Enabled: b.Enabled, Rules: rs, Cooldown: cd,
		})
	}

	// ListBindings 已按 (account_id, binding_id) 排序，顺序稳定
	raw, err := json.Marshal(shapes)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// runtimeBindingView 是一个绑定的运行状态。
type runtimeBindingView struct {
	ID          int64  `json:"id"`
	AccountName string `json:"accountName"`
	RoomID      string `json:"roomId"`
	Enabled     bool   `json:"enabled"`
	Running     bool   `json:"running"`
	State       string `json:"state"`
}

// handleRuntimeMeta 报告每个可见绑定的运行状态，以及配置是否已过时
// （数据库里保存的配置与机器人当前跑的引擎不一致，需要点「重载」）。
func (s *Server) handleRuntimeMeta(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())

	bs, err := s.visibleBindings(r.Context(), u)
	if err != nil {
		respondStoreError(w, err, "")
		return
	}

	rts := s.runtime.all()
	views := make([]runtimeBindingView, 0, len(bs))
	for _, b := range bs {
		v := runtimeBindingView{
			ID: b.ID, AccountName: b.AccountName, RoomID: b.RoomID,
			Enabled: b.Enabled, State: "not_running",
		}
		if rt, ok := rts[b.ID]; ok {
			v.Running = true
			v.State = string(rt.State())
		}
		views = append(views, v)
	}

	current, err := s.CurrentConfigHash(r.Context())
	if err != nil {
		respondStoreError(w, err, "")
		return
	}
	started := s.cfgHash.get()

	respondJSON(w, http.StatusOK, map[string]any{
		// configStale 为真时，界面应提示「有已保存但还没重载的改动」
		"configStale": started != "" && started != current,
		"bindings":    views,
	})
}
