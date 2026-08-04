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

	// IsOwner 是调用者是不是这个绑定所属账号的所有者（管理员也为
	// true）。拉黑（P5-6）是账号级操作，走 isAccountOwner 而不是权限点，
	// 前端靠这个字段决定显不显示「拉黑」区，不要用 AccountName 去跟
	// 别处的账号列表比对——那是在前端重算一遍账号所有权判定，必然漂。
	IsOwner bool `json:"isOwner"`

	// LiveStatus 是最近一次直播间开播状态检测的结果（"living"/"offline"/
	// "unknown"），由 cmd/magicd 的心跳循环（60 秒一次）或新增绑定时的
	// 立即检测写入。LiveCheckedAt 为 nil 表示这个绑定从未被检测过。
	//
	// **"unknown" 不等于"未开播"**：探测本身失败（网络不通、被风控）
	// 也会落在这一档，与"确认未开播"是完全不同的两件事，前端不能把它
	// 当作 offline 的同义词显示，理由与账号登录态的 unknown 完全一致。
	LiveStatus    string  `json:"liveStatus"`
	LiveCheckedAt *string `json:"liveCheckedAt"`
	// AnchorUID/AnchorName 是主播身份——AnchorUID 是主播 UID，不是
	// RoomID（房间号）；两者探测成功前都是空串。
	AnchorUID  string `json:"anchorUid"`
	AnchorName string `json:"anchorName"`
}

// toBindingView 是 bindingView 唯一的构造入口，保证 handleListBindings
// 与 handleCreateBinding 对同一批字段的映射逻辑不会走岔。
func toBindingView(b *store.Binding, ruleCount int, permissions []string, isOwner bool) bindingView {
	v := bindingView{
		ID: b.ID, AccountID: b.AccountID, AccountName: b.AccountName,
		RoomID: b.RoomID, Enabled: b.Enabled, RuleCount: ruleCount,
		Permissions: permissions,
		IsOwner:     isOwner,
		LiveStatus:  b.LiveStatus,
		AnchorUID:   b.AnchorUID,
		AnchorName:  b.AnchorName,
	}
	if b.LiveCheckedAt != nil {
		s := b.LiveCheckedAt.Format(timeLayout)
		v.LiveCheckedAt = &s
	}
	return v
}

// RoomStatusProbe 是新增绑定后立即探测一次直播间开播状态与主播身份
// 的能力，不必等 60 秒心跳。
//
// httpapi 自己不知道怎么打 B 站接口——具体探测逻辑（走哪个接口、字段
// 路径）留在 cmd/magicd 里，通过 SetRoomStatusProbe 注入，与
// BindingLifecycle/LoginProbe 是同一种解耦方式。可能为 nil（测试环境
// 通常不关心这一步），处理器判空后跳过——加绑定这个主流程不该被这一步
// 缺失或探测失败拖累。
type RoomStatusProbe interface {
	// ProbeNow 探测 bindingID 对应的直播间并写库。
	ProbeNow(ctx context.Context, bindingID int64)
}

// SetRoomStatusProbe 注入直播间状态立即检测能力。run 在装配完成后调用一次。
func (s *Server) SetRoomStatusProbe(p RoomStatusProbe) { s.roomStatusProbe = p }

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

// isOwner 判断调用者是不是 b 所属账号的所有者（管理员也算）。
//
// 与 of() 判断权限点是两条独立的轴：拉黑走账号所有权，不走权限点，
// 前端要能分别渲染「有 user:block 但不是所有者」（能禁言不能拉黑）
// 与「是所有者」（都能）两种状态，缺一个字段就只能靠权限点列表反推，
// 而反推等于在前端重新实现一遍 isAccountOwner 的判定，必然漂。
func (ps *permissionSet) isOwner(b *store.Binding) bool {
	return ps.admin || ps.owned[b.AccountName]
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
		out = append(out, toBindingView(&b, len(rs), ps.of(&b), ps.isOwner(&b)))
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

	// 立即探测一次直播间状态，不必等 60 秒心跳——用户添加房间号的这一刻
	// 正是最想确认"有没有加错房间"的时候。门槛与上面的 StartBinding
	// 一致（只对启用的绑定做），道理相同：重复点一下按钮落在一个已被
	// 手动停用的绑定上时，不该悄悄替它探测。同步调用（不起 goroutine），
	// 这样探测成功时下面重新查到的 b 才能带着最新结果一起返回给前端，
	// 不必让用户刷新页面才看到。
	if s.roomStatusProbe != nil && b.Enabled {
		s.roomStatusProbe.ProbeNow(r.Context(), b.ID)
		if fresh, err := s.store.GetBindingByID(r.Context(), b.ID); err == nil {
			b = fresh
		} else {
			// 绑定不该在这几行之间消失；查不到就沿用探测前的 b，
			// 响应里的开播状态字段会是探测前的旧值——比让整个请求
			// 报错更合理，绑定本身已经创建成功了。
			s.log.Warn("立即检测后重新查询绑定失败", "binding", b.Label(), "err", err)
		}
	}

	ps, err := s.callerPermissions(r.Context(), u)
	if err != nil {
		respondStoreError(w, err, "")
		return
	}
	respondJSON(w, http.StatusCreated, toBindingView(b, 0, ps.of(b), ps.isOwner(b)))
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
