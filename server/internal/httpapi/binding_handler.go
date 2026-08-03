package httpapi

import (
	"context"
	"net/http"
	"strings"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/perm"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/store"
)

// bindingView 是绑定对外的表示。
//
// 带上调用者在这个绑定上的权限点，前端据此决定显示哪些按钮，
// 不必再逐个绑定发请求去问。
type bindingView struct {
	ID          int64    `json:"id"`
	AccountID   int64    `json:"accountId"`
	AccountName string   `json:"accountName"`
	RoomID      string   `json:"roomId"`
	Enabled     bool     `json:"enabled"`
	RuleCount   int      `json:"ruleCount"`
	Permissions []string `json:"permissions"`
}

// permissionSet 是调用者在各绑定上的权限点快照。
//
// 用一次 ListMemberships 建好，之后查任意绑定都不再碰数据库。
// 逐绑定逐权限点调 Can 是 7×绑定数 次往返，而 memberships 表里
// 一行就带着该绑定的整组权限点，本来就不需要问七次。
//
// 判定顺序与 guard.go 的 canSeeBinding 一致：管理员 → 账号所有者
// → 授权行。三处必须同步，否则会出现「列表说你没权限、请求却成了」
// 这种比报错更难查的不一致。
type permissionSet struct {
	admin     bool
	owned     map[string]bool // 调用者拥有的账号名
	byBinding map[int64][]string
}

// callerPermissions 拉取调用者的全部授权，建成按绑定主键索引的快照。
func (s *Server) callerPermissions(ctx context.Context, u *store.User) (*permissionSet, error) {
	ps := &permissionSet{admin: u.IsAdmin, byBinding: map[int64][]string{}}
	if u.IsAdmin {
		// 管理员绕过全部检查，不必查——他在 memberships 表里本来就没有行
		return ps, nil
	}

	owned, err := s.ownedAccountNames(ctx, u)
	if err != nil {
		return nil, err
	}
	ps.owned = owned

	ms, err := s.store.ListMemberships(ctx, u.Username)
	if err != nil {
		return nil, err
	}
	for _, m := range ms {
		ps.byBinding[m.BindingID] = perm.Strings(m.Permissions)
	}
	return ps, nil
}

// of 返回调用者在某绑定上的权限点。
//
// 永远返回非 nil 切片：JSON 里要出 [] 而不是 null，前端拿到 null
// 做 .includes() 会直接抛异常。
func (ps *permissionSet) of(b *store.Binding) []string {
	if ps.admin {
		return perm.Strings(perm.All())
	}

	// 所有者与授权行是**并集**，不是二选一。store.Can 的 SQL 是三条
	// OR，这里若命中 owned 就提前 return 会漏掉 byBinding——而
	// 「所有者 + 显式授予 member:manage」正是 Task 8b 裁决之后的
	// 标准配置。漏掉的表现是「列表说你没权限、请求却成了」。
	set := make(map[string]bool)
	if ps.owned[b.AccountName] {
		// 与 store.Can 同一条规则（定义在 perm.OwnerBypass）：所有者
		// 凭所有权拿全部权限点减去 member:manage——把第三方拉进授权
		// 体系是管理员级别的决定，不是账号所有权的附带品。
		for _, p := range perm.All() {
			if perm.OwnerBypass(p) {
				set[string(p)] = true
			}
		}
	}
	for _, p := range ps.byBinding[b.ID] {
		set[p] = true
	}

	// 按 perm.All() 的声明顺序输出，保证响应稳定、可测试
	out := make([]string, 0, len(set))
	for _, p := range perm.All() {
		if set[string(p)] {
			out = append(out, string(p))
		}
	}
	return out
}

func (s *Server) handleListBindings(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())

	bs, err := s.visibleBindings(r.Context(), u)
	if err != nil {
		respondStoreError(w, err, "")
		return
	}

	ps, err := s.callerPermissions(r.Context(), u)
	if err != nil {
		respondStoreError(w, err, "")
		return
	}

	out := make([]bindingView, 0, len(bs))
	for _, b := range bs {
		// 规则条数是每个绑定一次查询。绑定数是「账号数 × 房间数」，
		// 实际部署里几个到几十个，与 bindingByID 全量拉列表是同一个
		// 量级判断；等它成为瓶颈再加一次性 GROUP BY 计数。
		rs, err := s.store.ListRules(r.Context(), b.ID)
		if err != nil {
			respondStoreError(w, err, "")
			return
		}
		out = append(out, bindingView{
			ID:          b.ID,
			AccountID:   b.AccountID,
			AccountName: b.AccountName,
			RoomID:      b.RoomID,
			Enabled:     b.Enabled,
			RuleCount:   len(rs),
			Permissions: ps.of(&b),
		})
	}
	respondJSON(w, http.StatusOK, out)
}

// handleCreateBinding 给账号加一个直播间。
//
// 授权：账号所有者或管理员。这不挂在已有绑定上（绑定还不存在），
// 所以不走 requirePerm。
func (s *Server) handleCreateBinding(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())

	var req struct {
		AccountName string `json:"accountName"`
		RoomID      string `json:"roomId"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	roomID := strings.TrimSpace(req.RoomID)
	if roomID == "" {
		respondError(w, http.StatusUnprocessableEntity, "房间号不能为空")
		return
	}

	acc, err := s.store.GetAccountByName(r.Context(), req.AccountName)
	if err != nil {
		respondStoreError(w, err, "账号 "+req.AccountName+" 不存在")
		return
	}
	if !s.isAccountOwner(u, acc) {
		respondError(w, http.StatusNotFound, "账号 %s 不存在", req.AccountName)
		return
	}

	// UpsertBinding 是幂等的：重复点一下按钮不该看到红色报错
	b, err := s.store.UpsertBinding(r.Context(), acc.ID, roomID)
	if err != nil {
		respondStoreError(w, err, "")
		return
	}

	// 只有真正启用的绑定才起运行时：UpsertBinding 不改动已有绑定的
	// enabled（幂等分支落在一个被用户手动停用的绑定上时 b.Enabled 为
	// 假），这时不该把它悄悄重新拉起。
	if s.lifecycle != nil && b.Enabled {
		if err := s.lifecycle.StartBinding(r.Context(), b.ID); err != nil {
			// 数据库状态已经改对，运行时没跟上不该让这次请求报错——
			// 用户看到的应该是「绑定加成功了」，账号异常/网络问题
			// 从日志和 /api/meta/runtime 的连接状态里看得出来。
			s.log.Warn("绑定已创建，但装配运行时失败", "binding", b.Label(), "err", err)
		}
	}

	ps, err := s.callerPermissions(r.Context(), u)
	if err != nil {
		respondStoreError(w, err, "")
		return
	}
	respondJSON(w, http.StatusCreated, bindingView{
		ID: b.ID, AccountID: b.AccountID, AccountName: b.AccountName,
		RoomID: b.RoomID, Enabled: b.Enabled, RuleCount: 0,
		Permissions: ps.of(b),
	})
}

// handlePatchBinding 启停绑定。守卫是 rule:write。
func (s *Server) handlePatchBinding(w http.ResponseWriter, r *http.Request) {
	b := bindingFrom(r.Context())

	var req struct {
		Enabled *bool `json:"enabled"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Enabled == nil {
		respondError(w, http.StatusUnprocessableEntity, "请提供 enabled 字段")
		return
	}

	if err := s.store.SetBindingEnabled(r.Context(), b.AccountName, b.RoomID, *req.Enabled); err != nil {
		respondStoreError(w, err, "绑定不存在")
		return
	}

	// 数据库状态已经改对，让运行时跟上——这正是 P5-1 要修的问题：
	// 启停过去只改库，不重启进程不生效。
	if s.lifecycle != nil {
		if *req.Enabled {
			if err := s.lifecycle.StartBinding(r.Context(), b.ID); err != nil {
				s.log.Warn("绑定已启用，但装配运行时失败", "binding", b.Label(), "err", err)
			}
		} else {
			s.lifecycle.StopBinding(r.Context(), b.ID)
		}
	}
	respondJSON(w, http.StatusOK, map[string]any{"enabled": *req.Enabled})
}

// handleDeleteBinding 删除绑定及其全部规则、冷却组、KV 与授权。
//
// 授权：账号所有者或管理员。**有 rule:write 也不够**——删绑定会带走
// 全部规则与授权，是账号所有权级别的操作，不是编辑规则级别的。
func (s *Server) handleDeleteBinding(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())

	id, err := parseBindingID(r)
	if err != nil {
		respondError(w, http.StatusNotFound, "绑定不存在")
		return
	}
	b, err := s.bindingByID(r.Context(), id)
	if err != nil {
		respondStoreError(w, err, "绑定不存在")
		return
	}

	acc, err := s.store.GetAccountByName(r.Context(), b.AccountName)
	if err != nil {
		respondStoreError(w, err, "账号不存在")
		return
	}
	if !s.isAccountOwner(u, acc) {
		// 与 requirePerm 的判定完全一致：这条路径走的是 requireAuth，
		// 没有守卫替它做可见性判断，所以必须自己做。无条件回 403 的话，
		// 拿绑定 ID 从 1 递增试一遍就能枚举出部署里有哪些绑定——
		// 「不存在」与「存在但不归你」必须不可区分。
		visible, err := s.canSeeBinding(r.Context(), u, b)
		if err != nil {
			respondStoreError(w, err, "")
			return
		}
		if !visible {
			respondError(w, http.StatusNotFound, "绑定不存在")
			return
		}
		// 可见但不是所有者 → 403：对方已经知道这个绑定存在，不算泄漏
		respondError(w, http.StatusForbidden, "只有账号所有者能删除绑定")
		return
	}

	if err := s.store.DeleteBinding(r.Context(), b.AccountName, b.RoomID); err != nil {
		respondStoreError(w, err, "绑定不存在")
		return
	}

	// 删库成功之后立刻拆运行时——不拆的话会悬挂着一个数据库里已经查不到
	// 的绑定：定时任务、连接、goroutine 全部悬空，且再也没有任何 API
	// 路径能摸到它、清理它，只能重启进程。
	if s.lifecycle != nil {
		s.lifecycle.StopBinding(r.Context(), b.ID)
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
