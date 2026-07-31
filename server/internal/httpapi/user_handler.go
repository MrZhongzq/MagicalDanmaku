package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/store"
)

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	us, err := s.store.ListUsers(r.Context())
	if err != nil {
		respondStoreError(w, err, "")
		return
	}
	out := make([]userView, 0, len(us))
	for i := range us {
		out = append(out, toUserView(&us[i]))
	}
	respondJSON(w, http.StatusOK, out)
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		IsAdmin  bool   `json:"isAdmin"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Username) == "" {
		respondError(w, http.StatusUnprocessableEntity, "用户名不能为空")
		return
	}
	if len(req.Password) < 8 {
		respondError(w, http.StatusUnprocessableEntity, "密码至少 8 个字符")
		return
	}

	u, err := s.store.CreateUser(r.Context(), req.Username, req.Password, req.IsAdmin)
	if err != nil {
		respondStoreError(w, err, "")
		return
	}
	respondJSON(w, http.StatusCreated, toUserView(u))
}

// handleChangePassword 改密码。
//
// 两条路径：自己改自己必须带旧密码；管理员改他人不需要。
// 两者都撤销该用户的全部会话——改了密码而旧会话还能用，等于没改。
func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	caller := userFrom(r.Context())
	target := r.PathValue("name")

	var req struct {
		OldPassword string `json:"oldPassword"`
		NewPassword string `json:"newPassword"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if len(req.NewPassword) < 8 {
		respondError(w, http.StatusUnprocessableEntity, "新密码至少 8 个字符")
		return
	}

	switch {
	case caller.IsAdmin:
		// 管理员改任何人的密码都不需要旧密码
	case caller.Username == target:
		if _, err := s.store.VerifyPassword(r.Context(), target, req.OldPassword); err != nil {
			if errors.Is(err, store.ErrBadCredentials) {
				respondError(w, http.StatusUnauthorized, "旧密码不正确")
				return
			}
			respondStoreError(w, err, "用户不存在")
			return
		}
	default:
		respondError(w, http.StatusForbidden, "只能修改自己的密码")
		return
	}

	if err := s.store.SetPassword(r.Context(), target, req.NewPassword); err != nil {
		respondStoreError(w, err, "用户 "+target+" 不存在")
		return
	}

	u, err := s.store.GetUserByName(r.Context(), target)
	if err != nil {
		respondStoreError(w, err, "用户 "+target+" 不存在")
		return
	}
	if _, err := s.store.DeleteUserSessions(r.Context(), u.ID); err != nil {
		respondStoreError(w, err, "")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	caller := userFrom(r.Context())
	target := r.PathValue("name")

	// 删掉自己会把系统锁死——可能一个管理员都不剩
	if caller.Username == target {
		respondError(w, http.StatusConflict, "不能删除自己")
		return
	}

	u, err := s.store.GetUserByName(r.Context(), target)
	if err != nil {
		respondStoreError(w, err, "用户 "+target+" 不存在")
		return
	}

	// accounts.owner_id 是 ON DELETE RESTRICT，还拥有账号的用户删不掉，
	// 避免留下无主的 Cookie。把这个约束翻译成人能看懂的话。
	if _, err := s.store.DeleteUser(r.Context(), u.ID); err != nil {
		if store.IsForeignKeyViolation(err) {
			respondError(w, http.StatusConflict,
				"用户 %s 还拥有 B 站账号，请先转移或删除这些账号", target)
			return
		}
		respondStoreError(w, err, "用户 "+target+" 不存在")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
