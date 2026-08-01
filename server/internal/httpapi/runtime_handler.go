package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sync"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules/spec"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/store"
)

// configHashes 记录**每个绑定各自**"已保存但可能还没重载"的配置版本。
//
// 热重载是显式触发的、按绑定触发的（保存后手动点某一个绑定的重载），
// 不是文件监听或轮询——界面必须诚实地告诉用户：具体是哪个绑定当前
// 跑的引擎与数据库里最新保存的配置不一致。
//
// 这必须是按绑定的映射而不是一个全局标量：改了 A、B 两个绑定后只
// 重载 A，若哈希是全库合成的一个值，A 重载后写回的新哈希会把 B 的
// 改动也一并标记成"已生效"——B 的引擎其实完全没变，仍在跑旧规则，
// 界面却不再有任何提示。
type configHashes struct {
	mu sync.RWMutex
	m  map[int64]string
}

// setAll 整体替换全部绑定的哈希基线。
//
// 只应在“此刻全部绑定的运行引擎确实都对应数据库当前状态”时调用——
// 也就是 run 启动完成的那一刻。热重载成功后必须用 setOne，
// 绝不能顺手调这个：那等于把「按绑定重载」又变回「按全库重算」。
func (c *configHashes) setAll(m map[int64]string) {
	c.mu.Lock()
	c.m = m
	c.mu.Unlock()
}

// setOne 只更新一个绑定的哈希基线。
func (c *configHashes) setOne(id int64, h string) {
	c.mu.Lock()
	if c.m == nil {
		c.m = make(map[int64]string)
	}
	c.m[id] = h
	c.mu.Unlock()
}

func (c *configHashes) get(id int64) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	h, ok := c.m[id]
	return h, ok
}

// SetConfigHash 记下机器人当前运行所依据的配置版本，逐绑定给出。
// run 在装配完成后调用一次：此时每个绑定的运行引擎都恰好对应数据库
// 当前状态。
func (s *Server) SetConfigHash(m map[int64]string) { s.cfgHash.setAll(m) }

// SetBindingConfigHash 只更新一个绑定的配置版本。
//
// handleReload 重载成功后调用它，而不是 SetConfigHash：按绑定重载，
// 就该按绑定更新版本号，不能碰其余绑定的基线。
func (s *Server) SetBindingConfigHash(bindingID int64, h string) {
	s.cfgHash.setOne(bindingID, h)
}

// bindingConfigShape 是参与哈希的字段：只取影响运行行为的部分。
// 账号的 Cookie 不进哈希——换 Cookie 不需要重载（下次请求就用新的）。
type bindingConfigShape struct {
	ID       int64            `json:"id"`
	Enabled  bool             `json:"enabled"`
	Rules    []spec.Rule      `json:"rules"`
	Cooldown map[string]int64 `json:"cooldown"`
}

// currentBindingHash 算**一个**绑定当前在数据库里的配置哈希。
//
// 按绑定单独算，供 handleReload 在只重载了这一个绑定之后，只更新
// 这一个绑定自己的版本号用——避免「按绑定重载却按全库重算」。
func (s *Server) currentBindingHash(ctx context.Context, b *store.Binding) (string, error) {
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

	raw, err := json.Marshal(bindingConfigShape{ID: b.ID, Enabled: b.Enabled, Rules: rs, Cooldown: cd})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// CurrentConfigHash 算数据库里当前配置的哈希，按 bindingID 分别给出。
func (s *Server) CurrentConfigHash(ctx context.Context) (map[int64]string, error) {
	bindings, err := s.store.ListBindings(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]string, len(bindings))
	for i := range bindings {
		h, err := s.currentBindingHash(ctx, &bindings[i])
		if err != nil {
			return nil, err
		}
		out[bindings[i].ID] = h
	}
	return out, nil
}

// runtimeBindingView 是一个绑定的运行状态。
type runtimeBindingView struct {
	ID          int64  `json:"id"`
	AccountName string `json:"accountName"`
	RoomID      string `json:"roomId"`
	Enabled     bool   `json:"enabled"`
	Running     bool   `json:"running"`
	State       string `json:"state"`

	// ConfigStale 为真时，界面应提示"这个绑定有已保存但还没重载的改动"。
	// 必须逐绑定给出：只有一个全局标量的话，指不出具体该重载哪一个。
	ConfigStale bool `json:"configStale"`
}

// handleRuntimeMeta 报告每个可见绑定的运行状态，以及配置是否已过时
// （数据库里保存的配置与机器人当前跑的引擎不一致，需要点该绑定的
// 「重载」）。
//
// 顶层 configStale 是"任意一个可见绑定 stale"的汇总，供前端显示一个
// 总的提示角标；具体该重载哪一个，看每个绑定自己的 configStale。
func (s *Server) handleRuntimeMeta(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())

	bs, err := s.visibleBindings(r.Context(), u)
	if err != nil {
		respondStoreError(w, err, "")
		return
	}

	current, err := s.CurrentConfigHash(r.Context())
	if err != nil {
		respondStoreError(w, err, "")
		return
	}

	rts := s.runtime.all()
	views := make([]runtimeBindingView, 0, len(bs))
	anyStale := false
	for _, b := range bs {
		v := runtimeBindingView{
			ID: b.ID, AccountName: b.AccountName, RoomID: b.RoomID,
			Enabled: b.Enabled, State: "not_running",
		}
		if rt, ok := rts[b.ID]; ok {
			v.Running = true
			v.State = string(rt.State())
		}
		if started, ok := s.cfgHash.get(b.ID); ok && started != current[b.ID] {
			v.ConfigStale = true
			anyStale = true
		}
		views = append(views, v)
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"configStale": anyStale,
		"bindings":    views,
	})
}
