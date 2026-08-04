package httpapi

import (
	"net/http"
	"strings"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/store"
)

// blockedView 是禁言名单一条记录对外的表示。
type blockedView struct {
	ID        int64  `json:"id"`
	UID       string `json:"uid"`
	Username  string `json:"username"`
	Reason    string `json:"reason"`
	CreatedBy *int64 `json:"createdBy"` // 操作人被删除后为 null，名单本身保留
	CreatedAt string `json:"createdAt"`
}

func toBlockedView(b *store.BlockedUser) blockedView {
	return blockedView{
		ID:        b.ID,
		UID:       b.UID,
		Username:  b.Username,
		Reason:    b.Reason,
		CreatedBy: b.CreatedBy,
		CreatedAt: b.CreatedAt.Format(timeLayout),
	}
}

func (s *Server) handleListBlockList(w http.ResponseWriter, r *http.Request) {
	b := bindingFrom(r.Context())

	list, err := s.store.ListBlockList(r.Context(), b.ID)
	if err != nil {
		respondStoreError(w, err, "")
		return
	}
	out := make([]blockedView, 0, len(list))
	for i := range list {
		out = append(out, toBlockedView(&list[i]))
	}
	respondJSON(w, http.StatusOK, out)
}

func (s *Server) handleAddToBlockList(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	b := bindingFrom(r.Context())

	var req struct {
		UID      string `json:"uid"`
		Username string `json:"username"`
		Reason   string `json:"reason"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	uid := strings.TrimSpace(req.UID)
	if uid == "" {
		respondError(w, http.StatusUnprocessableEntity, "UID 不能为空")
		return
	}

	// 昵称自动按 UID 查——前端不再要求手填（P5-6：「昵称应由 UID 自动
	// 获取」）。尽力而为：查不到就留空，不能让「查昵称失败」拖累「加入
	// 名单」这个主流程本身。只在没有运行时（机器人没跑）或查询本身失败
	// 时静默降级，不占用 runtimeFor 的 503 响应路径——那是给真正的即时
	// 动作用的，「加入名单」是纯本地存储操作，不该因为机器人没启动而
	// 整体失败。
	username := strings.TrimSpace(req.Username)
	if username == "" {
		if rt, ok := s.runtime.get(b.ID); ok {
			if name, err := rt.Nickname(r.Context(), uid); err != nil {
				s.log.Warn("加入禁言名单时自动查询昵称失败，昵称留空",
					"binding", b.Label(), "uid", uid, "err", err)
			} else {
				username = name
			}
		}
	}

	if err := s.store.AddToBlockList(r.Context(), b.ID, uid, username, req.Reason, &u.ID); err != nil {
		respondStoreError(w, err, "")
		return
	}
	respondJSON(w, http.StatusCreated, map[string]string{"uid": uid})
}

func (s *Server) handleRemoveFromBlockList(w http.ResponseWriter, r *http.Request) {
	b := bindingFrom(r.Context())
	uid := r.PathValue("uid")

	if err := s.store.RemoveFromBlockList(r.Context(), b.ID, uid); err != nil {
		respondStoreError(w, err, "UID "+uid+" 不在禁言名单里")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ---- 账号级拉黑（P5-6）----
//
// 与上面的禁言名单（本地维护、房间级）以及 action_handler.go 的
// handleBlockUser/handleUnblockUser（B 站禁言接口、房间级）都不是同一
// 件事：拉黑调的是 x/relation/modify，账号级，与直播间无关。守卫用
// requireBindingOwner（账号所有者或管理员），不是 requirePerm(user:block)——
// 这是本任务在权限模型上的核心决定，见 guard.go 里 requireBindingOwner
// 的注释。

func (s *Server) handleBlacklistUser(w http.ResponseWriter, r *http.Request) {
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
	if err := rt.Blacklist(r.Context(), uid); err != nil {
		// 拉黑是不可逆的对外操作，失败原因要原样透出——「操作失败，
		// 请重试」对这类请求没有意义，B 站侧的具体原因才有用。
		s.log.Warn("手动拉黑失败", "binding", b.Label(), "uid", uid, "err", err)
		respondError(w, http.StatusBadGateway, "拉黑失败: %v", err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"uid": uid})
}

func (s *Server) handleUnblacklistUser(w http.ResponseWriter, r *http.Request) {
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
	if err := rt.Unblacklist(r.Context(), uid); err != nil {
		s.log.Warn("手动解除拉黑失败", "binding", b.Label(), "uid", uid, "err", err)
		respondError(w, http.StatusBadGateway, "解除拉黑失败: %v", err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"uid": uid})
}

// blacklistStatusView 是拉黑状态回读的响应体。
type blacklistStatusView struct {
	UID         string `json:"uid"`
	Blacklisted bool   `json:"blacklisted"`
	// Nickname 是尽力而为的自动回填，查不到时留空——不能让昵称查询失败
	// 拖累状态回读本身。
	Nickname string `json:"nickname"`
}

// handleBlacklistStatus 是"白捡"的状态回读：GET x/space/wbi/acc/relation
// 的 attribute==128 即已拉黑。只读查询，不改变任何状态，因此用 GET——
// 与"一切改变状态的接口不得用 GET"不冲突。
func (s *Server) handleBlacklistStatus(w http.ResponseWriter, r *http.Request) {
	b := bindingFrom(r.Context())
	uid := strings.TrimSpace(r.URL.Query().Get("uid"))
	if uid == "" {
		respondError(w, http.StatusUnprocessableEntity, "UID 不能为空")
		return
	}

	rt, ok := s.runtimeFor(w, b.ID, b.Label())
	if !ok {
		return
	}
	blacklisted, nickname, err := rt.BlacklistStatus(r.Context(), uid)
	if err != nil {
		s.log.Warn("查询拉黑状态失败", "binding", b.Label(), "uid", uid, "err", err)
		respondError(w, http.StatusBadGateway, "查询拉黑状态失败: %v", err)
		return
	}
	respondJSON(w, http.StatusOK, blacklistStatusView{
		UID: uid, Blacklisted: blacklisted, Nickname: nickname,
	})
}
