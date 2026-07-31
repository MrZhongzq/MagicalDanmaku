package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/auth"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/store"
)

// accountView 是 B 站账号对外的表示。
//
// **刻意不含 cookie 字段。** Cookie 等同于账号密码，绝不出现在任何
// 响应体里——这是本项目的硬性约束，有测试直接在原始 JSON 里搜
// "SESSDATA" 来守它。
type accountView struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	UID         string `json:"uid"`
	RateLimitMs int64  `json:"rateLimitMs"`
	MaxLength   int    `json:"maxLength"`
	OwnerID     int64  `json:"ownerId"`
	IsOwner     bool   `json:"isOwner"`
	CreatedAt   string `json:"createdAt"`
}

func toAccountView(a *store.Account, callerID int64) accountView {
	return accountView{
		ID:          a.ID,
		Name:        a.Name,
		UID:         a.UID,
		RateLimitMs: a.RateLimit.Milliseconds(),
		MaxLength:   a.MaxLength,
		OwnerID:     a.OwnerID,
		IsOwner:     a.OwnerID == callerID,
		CreatedAt:   a.CreatedAt.Format(timeLayout),
	}
}

// qrStarter 抽出扫码登录的两个方法，便于测试注入假实现
// （真打 B 站接口的测试既慢又不可控）。
type qrStarter interface {
	Generate(ctx context.Context) (*auth.QRCode, error)
	Poll(ctx context.Context, key string) (*auth.PollResult, error)
}

// SetQRLogin 替换扫码登录实现。仅测试使用。
func (s *Server) SetQRLogin(l qrStarter) { s.qrLogin = l }

func (s *Server) handleListAccounts(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())

	visible, err := s.visibleAccountNames(r.Context(), u)
	if err != nil {
		respondStoreError(w, err, "")
		return
	}
	all, err := s.store.ListAccounts(r.Context())
	if err != nil {
		respondStoreError(w, err, "")
		return
	}

	out := make([]accountView, 0, len(all))
	for i := range all {
		if visible[all[i].Name] {
			out = append(out, toAccountView(&all[i], u.ID))
		}
	}
	respondJSON(w, http.StatusOK, out)
}

// handleQRCodeStart 开始一次扫码登录。
func (s *Server) handleQRCodeStart(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())

	var req struct {
		Name string `json:"name"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		respondError(w, http.StatusUnprocessableEntity, "账号名不能为空")
		return
	}

	// 若账号已存在，只有能管它的人才能换 Cookie
	if existing, err := s.store.GetAccountByName(r.Context(), name); err == nil {
		if !u.IsAdmin && existing.OwnerID != u.ID {
			// 返回 404 而非 403：403 加上「不属于你」的文案等于告诉调用者
			// 这个账号名被别人占用了，任何登录用户拿任意名字试一次就能探测。
			// 与 handlePatchAccount / handleDeleteAccount 保持一致。
			//
			// 这道拦截本身不能删：saveScannedAccount 在落库时不做归属校验，
			// 只看账号名是否存在就直接换 Cookie，这里是唯一的关口。
			respondError(w, http.StatusNotFound, "账号 %s 不存在", name)
			return
		}
	} else if !errors.Is(err, store.ErrNotFound) {
		respondStoreError(w, err, "")
		return
	}

	qr, err := s.qrLogin.Generate(r.Context())
	if err != nil {
		s.log.Error("生成二维码失败", "err", err)
		respondError(w, http.StatusBadGateway, "向 B 站请求二维码失败，请稍后重试")
		return
	}

	s.qrs.purgeExpired()
	s.qrs.put(qr.Key, qrPending{AccountName: name, UserID: u.ID})

	respondJSON(w, http.StatusOK, map[string]string{
		"key": qr.Key,
		"url": qr.URL,
	})
}

// handleQRCodePoll 轮询一次扫码状态。成功时在服务端完成建号或换 Cookie。
//
// 用 POST 而非 GET：它会改变服务端状态（成功时建账号）。
func (s *Server) handleQRCodePoll(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	key := r.PathValue("key")

	pending, ok := s.qrs.take(key)
	if !ok {
		respondError(w, http.StatusNotFound, "扫码会话不存在或已过期，请重新发起")
		return
	}
	// 别人发起的扫码，拿着 key 也换不出账号
	if pending.UserID != u.ID {
		respondError(w, http.StatusNotFound, "扫码会话不存在或已过期，请重新发起")
		return
	}

	res, err := s.qrLogin.Poll(r.Context(), key)
	if err != nil {
		s.log.Error("轮询扫码状态失败", "err", err)
		respondError(w, http.StatusBadGateway, "向 B 站查询扫码状态失败，请稍后重试")
		return
	}

	switch res.Status {
	case auth.PollSuccess:
		// Cookie 在服务端落库，绝不回传给浏览器
		if err := s.saveScannedAccount(r.Context(), pending, res.Cookie); err != nil {
			respondStoreError(w, err, "")
			return
		}
		s.qrs.delete(key)
		respondJSON(w, http.StatusOK, map[string]string{
			"status":  "success",
			"account": pending.AccountName,
		})
	case auth.PollExpired:
		s.qrs.delete(key)
		respondJSON(w, http.StatusOK, map[string]string{"status": "expired"})
	default:
		respondJSON(w, http.StatusOK, map[string]string{"status": string(res.Status)})
	}
}

// saveScannedAccount 把扫到的 Cookie 落库：账号已存在就换 Cookie，否则新建。
func (s *Server) saveScannedAccount(ctx context.Context, p qrPending, cookie string) error {
	sess, err := auth.ParseSession(cookie)
	if err != nil {
		return err
	}

	if _, err := s.store.GetAccountByName(ctx, p.AccountName); err == nil {
		return s.store.UpdateAccountCookie(ctx, p.AccountName, cookie, sess.UID)
	} else if !errors.Is(err, store.ErrNotFound) {
		return err
	}

	_, err = s.store.CreateAccount(ctx, store.AccountInput{
		Name:    p.AccountName,
		UID:     sess.UID,
		Cookie:  cookie,
		OwnerID: p.UserID,
	})
	return err
}

// handlePatchAccount 改账号的限流与字数上限。
//
// 授权：所有者或管理员。这不挂在绑定上，所以不走 requirePerm。
func (s *Server) handlePatchAccount(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	name := r.PathValue("name")

	acc, err := s.store.GetAccountByName(r.Context(), name)
	if err != nil {
		respondStoreError(w, err, "账号 "+name+" 不存在")
		return
	}
	if !u.IsAdmin && acc.OwnerID != u.ID {
		// 不是所有者就当作不存在，避免被用来探测别人有哪些账号
		respondError(w, http.StatusNotFound, "账号 %s 不存在", name)
		return
	}

	var req struct {
		RateLimitMs *int64 `json:"rateLimitMs"`
		MaxLength   *int   `json:"maxLength"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	in := store.AccountInput{
		Name:      acc.Name,
		UID:       acc.UID,
		Cookie:    acc.Cookie,
		RateLimit: acc.RateLimit,
		MaxLength: acc.MaxLength,
		OwnerID:   acc.OwnerID,
	}
	if req.RateLimitMs != nil {
		if *req.RateLimitMs < 0 {
			respondError(w, http.StatusUnprocessableEntity, "发送间隔不能为负")
			return
		}
		in.RateLimit = time.Duration(*req.RateLimitMs) * time.Millisecond
	}
	if req.MaxLength != nil {
		if *req.MaxLength < 1 || *req.MaxLength > 40 {
			respondError(w, http.StatusUnprocessableEntity,
				"字数上限须在 1 到 40 之间（B 站单条弹幕上限是 40）")
			return
		}
		in.MaxLength = *req.MaxLength
	}

	updated, err := s.store.UpsertAccount(r.Context(), in)
	if err != nil {
		respondStoreError(w, err, "")
		return
	}
	respondJSON(w, http.StatusOK, toAccountView(updated, u.ID))
}

func (s *Server) handleDeleteAccount(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	name := r.PathValue("name")

	acc, err := s.store.GetAccountByName(r.Context(), name)
	if err != nil {
		respondStoreError(w, err, "账号 "+name+" 不存在")
		return
	}
	if !u.IsAdmin && acc.OwnerID != u.ID {
		respondError(w, http.StatusNotFound, "账号 %s 不存在", name)
		return
	}

	if err := s.store.DeleteAccount(r.Context(), name); err != nil {
		respondStoreError(w, err, "账号 "+name+" 不存在")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
