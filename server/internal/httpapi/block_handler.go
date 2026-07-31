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

	if err := s.store.AddToBlockList(r.Context(), b.ID, uid, req.Username, req.Reason, &u.ID); err != nil {
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
