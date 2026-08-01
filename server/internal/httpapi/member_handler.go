package httpapi

import (
	"net/http"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/perm"
)

// memberView 是一条授权对外的表示。
type memberView struct {
	Username    string   `json:"username"`
	Permissions []string `json:"permissions"`
}

func (s *Server) handleListMembers(w http.ResponseWriter, r *http.Request) {
	b := bindingFrom(r.Context())

	ms, err := s.store.ListBindingMembers(r.Context(), b.AccountName, b.RoomID)
	if err != nil {
		respondStoreError(w, err, "绑定不存在")
		return
	}
	out := make([]memberView, 0, len(ms))
	for _, m := range ms {
		out = append(out, memberView{
			Username:    m.Username,
			Permissions: perm.Strings(m.Permissions),
		})
	}
	respondJSON(w, http.StatusOK, out)
}

// handleGrantMember 授予权限点。
//
// **替换而非累加**：重新授权的语义是「设定为这些」，累加会让人以为
// 撤掉了某项其实还在。这是 P3 store.Grant 的既定语义，这里如实透传。
func (s *Server) handleGrantMember(w http.ResponseWriter, r *http.Request) {
	b := bindingFrom(r.Context())
	username := r.PathValue("username")

	var req struct {
		Permissions []string `json:"permissions"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if len(req.Permissions) == 0 {
		// 空列表语义含糊：是撤销还是没填？要求显式走 DELETE
		respondError(w, http.StatusUnprocessableEntity,
			"权限点列表为空。撤销授权请用 DELETE")
		return
	}

	ps := make([]perm.Permission, 0, len(req.Permissions))
	for _, raw := range req.Permissions {
		p, err := perm.Parse(raw)
		if err != nil {
			// perm.Parse 的错误信息里已经列出了全部合法值
			respondError(w, http.StatusUnprocessableEntity, "%v", err)
			return
		}
		ps = append(ps, p)
	}

	if err := s.store.Grant(r.Context(), username, b.AccountName, b.RoomID, ps); err != nil {
		respondStoreError(w, err, "用户 "+username+" 不存在")
		return
	}
	respondJSON(w, http.StatusOK, memberView{
		Username:    username,
		Permissions: perm.Strings(ps),
	})
}

func (s *Server) handleRevokeMember(w http.ResponseWriter, r *http.Request) {
	b := bindingFrom(r.Context())
	username := r.PathValue("username")

	if err := s.store.Revoke(r.Context(), username, b.AccountName, b.RoomID); err != nil {
		respondStoreError(w, err, username+" 在该绑定上没有授权记录")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
