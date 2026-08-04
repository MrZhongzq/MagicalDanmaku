package httpapi

import (
	"context"
	"errors"
	"fmt"
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

	// LoginState 是最近一次登录态检测的结果（"valid"/"invalid"/"unknown"），
	// 由 cmd/magicd 里的定期检测循环写入。LoginCheckedAt 为 nil 表示
	// 该账号从未被检测过。
	LoginState     string  `json:"loginState"`
	LoginCheckedAt *string `json:"loginCheckedAt"`
}

func toAccountView(a *store.Account, callerID int64) accountView {
	v := accountView{
		ID:          a.ID,
		Name:        a.Name,
		UID:         a.UID,
		RateLimitMs: a.RateLimit.Milliseconds(),
		MaxLength:   a.MaxLength,
		OwnerID:     a.OwnerID,
		IsOwner:     a.OwnerID == callerID,
		CreatedAt:   a.CreatedAt.Format(timeLayout),
		LoginState:  a.LoginState,
	}
	if a.LoginCheckedAt != nil {
		s := a.LoginCheckedAt.Format(timeLayout)
		v.LoginCheckedAt = &s
	}
	return v
}

// qrStarter 抽出扫码登录的两个方法，便于测试注入假实现
// （真打 B 站接口的测试既慢又不可控）。
type qrStarter interface {
	Generate(ctx context.Context) (*auth.QRCode, error)
	Poll(ctx context.Context, key string) (*auth.PollResult, error)
}

// SetQRLogin 替换扫码登录实现。仅测试使用。
func (s *Server) SetQRLogin(l qrStarter) { s.qrLogin = l }

// LoginProbe 是扫码成功后立即探测一次账号登录态的能力，不必等后台
// 10 分钟一轮的检测循环才发现扫码没有真的成功（比如扫码那一刻账号
// 被风控、或者拿到的 Cookie 缺字段）。
//
// httpapi 自己不知道怎么打 B 站接口——具体判定逻辑（nav 接口、
// code=-101 代表未登录）留在 cmd/magicd 里已经写好且测试过的
// checkAccountLogin，通过 SetLoginProbe 注入，与 BindingLifecycle 是
// 同一种解耦方式。可能为 nil（测试环境通常不关心这一步），处理器判空
// 后跳过——扫码成功这个主流程不该被这一步缺失或探测失败拖累。
type LoginProbe interface {
	// ProbeNow 探测 accountName 对应账号的登录态并写库。
	ProbeNow(ctx context.Context, accountName string)
}

// SetLoginProbe 注入登录态立即检测能力。run 在装配完成后调用一次。
func (s *Server) SetLoginProbe(p LoginProbe) { s.loginProbe = p }

// AccountRuntimeUpdater 是账号参数（发送间隔、单条弹幕字数上限）保存后，
// 把改动同步给该账号当前正在跑的全部绑定的能力。
//
// 与 BindingLifecycle 管的不是同一件事：BindingLifecycle 决定"一个绑定
// 在不在跑"，这里管的是"账号级参数变了，已经在跑的绑定要不要跟着变"。
// 此前的已知局限（P5-1 报告记录过）是账号的字数上限/发送间隔只在绑定
// 装配那一刻（cmd/magicd/run.go 的 buildRoomRuntime）读一次，改了不
// 重启不生效，界面上也没有任何提示——用户会以为保存失败或者软件有
// bug。httpapi 自己不知道
// 怎么把新参数落到运行中的限流器/Actions 上，具体实现留在 cmd/magicd
// 的 runtimeManager，通过 SetAccountRuntimeUpdater 注入，与 LoginProbe/
// RoomStatusProbe 是同一种解耦方式：可能为 nil（测试环境通常不关心这
// 一步），处理器判空后跳过。
type AccountRuntimeUpdater interface {
	// UpdateAccountRuntime 把 accountName 当前保存在数据库里的最新参数
	// 同步给该账号名下全部正在跑的绑定。这个账号眼下没有任何绑定在跑
	// 不是错误——下次绑定启动时装配自然会读到数据库里的新值。
	UpdateAccountRuntime(ctx context.Context, accountName string)
}

// SetAccountRuntimeUpdater 注入账号运行参数热传播能力。run 在装配完成
// 后调用一次。
func (s *Server) SetAccountRuntimeUpdater(u AccountRuntimeUpdater) { s.accountRuntimeUpdater = u }

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
		if !s.isAccountOwner(u, existing) {
			// 返回 404 而非 403：403 加上「不属于你」的文案等于告诉调用者
			// 这个账号名被别人占用了，任何登录用户拿任意名字试一次就能探测。
			// 与 handlePatchAccount / handleDeleteAccount 保持一致。
			//
			// 这道拦截挡的是「发起扫码那一刻」的状态，是 TOCTOU 的前半段；
			// 落库前 saveScannedAccount 还会重查一次——发起扫码时账号还
			// 不存在，等扫完它可能已经被别人建出来了，这道检查看不到那个。
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
	s.qrs.put(qr.Key, qrPending{AccountName: name, UserID: u.ID, IsAdmin: u.IsAdmin})

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
			// 文案与 handleQRCodeStart 的 404 一致：二者是同一件事的
			// TOCTOU 两端，调用者不该看出区别。
			respondStoreError(w, err, "账号 "+pending.AccountName+" 不存在")
			return
		}
		s.qrs.delete(key)

		// 立即探测一次登录态，不必等后台 10 分钟一轮的检测循环——用户
		// 刚扫完码，此刻正是最该马上知道结果的时候。同步调用（而不是
		// 起个 goroutine 异步做）：探测本身很轻量（一次 nav 请求），
		// 同步等它做完，才能保证响应返回时账号的登录态已经是最新的，
		// 而不是"响应说成功了，但状态要过一会儿才更新"。
		if s.loginProbe != nil {
			s.loginProbe.ProbeNow(r.Context(), pending.AccountName)
		}

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

	if acc, err := s.store.GetAccountByName(ctx, p.AccountName); err == nil {
		// 必须在落库前重查一次归属。handleQRCodeStart 那道检查看到的是
		// 至多 qrTTL(3 分钟) 之前的状态：发起扫码时账号还不存在，等扫完
		// 它可能已经被别人建出来了。不重查的话，攻击者对一个可猜的账号名
		// 发起扫码就能把自己的 Cookie 写进受害者的账号行——owner_id 不变，
		// 受害者的机器人从此以攻击者的 B 站身份发言。
		if !s.isAccountOwner(&store.User{ID: p.UserID, IsAdmin: p.IsAdmin}, acc) {
			return fmt.Errorf("账号 %s 不存在: %w", p.AccountName, store.ErrNotFound)
		}
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
	if !s.isAccountOwner(u, acc) {
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

	// 保存成功才通知——校验失败的早返回路径没有真的改动数据库，通知了
	// 也只是让 runtimeManager 白读一份跟改之前完全一样的配置。
	if s.accountRuntimeUpdater != nil {
		s.accountRuntimeUpdater.UpdateAccountRuntime(r.Context(), updated.Name)
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
	if !s.isAccountOwner(u, acc) {
		respondError(w, http.StatusNotFound, "账号 %s 不存在", name)
		return
	}

	// 删账号前先摸一遍它名下有哪些绑定，供下面拆运行时用——
	// DeleteAccount 会带着 ON DELETE CASCADE 把这些绑定行一并删掉，
	// 到那时候就再也查不出它们的 ID 了。
	var affected []int64
	if s.lifecycle != nil {
		bs, err := s.store.ListBindings(r.Context())
		if err != nil {
			respondStoreError(w, err, "")
			return
		}
		for _, b := range bs {
			if b.AccountID == acc.ID {
				affected = append(affected, b.ID)
			}
		}
	}

	if err := s.store.DeleteAccount(r.Context(), name); err != nil {
		respondStoreError(w, err, "账号 "+name+" 不存在")
		return
	}

	// 删库级联删掉的绑定不会自动摘运行时——不摘的话，这些绑定的连接/
	// goroutine/定时任务会变成永远没有任何 API 路径能摸到的悬挂资源。
	if s.lifecycle != nil {
		for _, id := range affected {
			s.lifecycle.StopBinding(r.Context(), id)
		}
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
