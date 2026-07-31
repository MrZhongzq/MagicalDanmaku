package httpapi

import (
	"errors"
	"net/http"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/perm"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/store"
)

// userView 是用户对外的表示。刻意不含任何凭据字段。
type userView struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	IsAdmin   bool   `json:"isAdmin"`
	CreatedAt string `json:"createdAt"`
}

func toUserView(u *store.User) userView {
	return userView{
		ID:        u.ID,
		Username:  u.Username,
		IsAdmin:   u.IsAdmin,
		CreatedAt: u.CreatedAt.Format(timeLayout),
	}
}

// timeLayout 是本 API 对外的时间格式，全包统一。
const timeLayout = "2006-01-02T15:04:05Z07:00"

// membershipView 是一条授权对外的表示。
type membershipView struct {
	BindingID   int64    `json:"bindingId"`
	AccountName string   `json:"accountName"`
	RoomID      string   `json:"roomId"`
	Permissions []string `json:"permissions"`
}

// handleLogin 校验密码并种下会话 Cookie。
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	u, err := s.store.VerifyPassword(r.Context(), req.Username, req.Password)
	if err != nil {
		if errors.Is(err, store.ErrBadCredentials) {
			// 用户不存在与密码错误返回完全一致的响应：
			// 区分开来，这个接口就成了用户名枚举器
			respondError(w, http.StatusUnauthorized, "用户名或密码错误")
			return
		}
		respondStoreError(w, err, "用户不存在")
		return
	}

	token, err := s.store.CreateSession(r.Context(), u.ID, s.opts.SessionTTL, r.UserAgent())
	if err != nil {
		respondStoreError(w, err, "")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,                 // 阻断 XSS 读取
		SameSite: http.SameSiteLaxMode, // 本项目替代 CSRF token 的前提
		Secure:   s.opts.SecureCookie,
		MaxAge:   int(s.opts.SessionTTL.Seconds()),
	})
	respondJSON(w, http.StatusOK, toUserView(u))
}

// handleLogout 撤销当前会话并清掉 Cookie。
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookieName); err == nil && c.Value != "" {
		if err := s.store.DeleteSession(r.Context(), c.Value); err != nil {
			respondStoreError(w, err, "")
			return
		}
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.opts.SecureCookie,
		MaxAge:   -1,
	})
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleMe 返回当前用户及其全部授权，前端据此决定显示哪些入口。
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())

	ms, err := s.store.ListMemberships(r.Context(), u.Username)
	if err != nil {
		respondStoreError(w, err, "")
		return
	}

	// 空数组而非 null：前端不该被迫判空
	views := make([]membershipView, 0, len(ms))
	for _, m := range ms {
		views = append(views, membershipView{
			BindingID:   m.BindingID,
			AccountName: m.AccountName,
			RoomID:      m.RoomID,
			Permissions: perm.Strings(m.Permissions),
		})
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"user":        toUserView(u),
		"memberships": views,
	})
}
