package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/perm"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/store"
)

const ctxKeyBinding ctxKey = 1

// requirePerm 要求当前用户对 URL 里的 {binding} 拥有权限点 p。
//
// **授权判定只有这一处实现。** 处理器里不得再写权限判断——
// 多一处就多一处漏判的可能。
//
// 它同时负责解析并加载绑定，放进 context 供处理器取用，
// 这样处理器不必再查一次。
func (s *Server) requirePerm(p perm.Permission, h http.HandlerFunc) http.HandlerFunc {
	return s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		u := userFrom(r.Context())

		raw := r.PathValue("binding")
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			// 不合法的 ID 与不存在的绑定对客户端是同一件事
			respondError(w, http.StatusNotFound, "绑定 %q 不存在", raw)
			return
		}

		b, err := s.bindingByID(r.Context(), id)
		if err != nil {
			respondStoreError(w, err, "绑定不存在")
			return
		}

		ok, err := s.store.Can(r.Context(), u.ID, b.ID, p)
		if err != nil {
			respondStoreError(w, err, "")
			return
		}
		if !ok {
			// 403 会带上 b.Label()（账号名@房间号），对一个连这个绑定
			// 存不存在都不该知道的调用者来说，这个 403 本身就是泄漏。
			// 只有对该绑定已有可见性的调用者才配收到带标签的 403；
			// 完全没有可见性的调用者要收到和「绑定不存在」完全一样的 404，
			// 二者不可区分——设计文档 §5 的要求。
			visible, err := s.canSeeBinding(r.Context(), u, b)
			if err != nil {
				respondStoreError(w, err, "")
				return
			}
			if !visible {
				respondError(w, http.StatusNotFound, "绑定不存在")
				return
			}
			respondError(w, http.StatusForbidden, "你在 %s 上没有 %s 权限", b.Label(), p)
			return
		}

		h(w, r.WithContext(context.WithValue(r.Context(), ctxKeyBinding, b)))
	})
}

// bindingFrom 取出守卫加载好的绑定。
func bindingFrom(ctx context.Context) *store.Binding {
	b, _ := ctx.Value(ctxKeyBinding).(*store.Binding)
	return b
}

// bindingByID 按主键查绑定。
//
// P3 的 store 只提供了按「账号名+房间号」查的 GetBinding，
// 而 URL 里带的是主键，所以这里在 ListBindings 的结果里挑。
// 绑定数量是「账号数 × 房间数」，实际部署里几个到几十个，
// 全量拉一次再挑完全可以接受；等它成为瓶颈再加按主键查的 SQL。
func (s *Server) bindingByID(ctx context.Context, id int64) (*store.Binding, error) {
	bs, err := s.store.ListBindings(ctx)
	if err != nil {
		return nil, err
	}
	for i := range bs {
		if bs[i].ID == id {
			return &bs[i], nil
		}
	}
	return nil, fmt.Errorf("store: 绑定 %d 不存在: %w", id, store.ErrNotFound)
}

// canSeeBinding 判断调用者对某一个绑定是否有任意可见性。
//
// 可见 = 管理员，或该绑定所属账号的所有者，或在该绑定上有任意授权
// 记录（不论具体权限点是什么）。判定标准与 visibleBindings 完全
// 一致，因为二者回答的是同一个问题——只是这里只查一个绑定，不必
// 为了判断一个绑定就拉取并过滤全部绑定列表。
//
// requirePerm 用它决定 Can 返回 false 之后该回 404 还是 403：
// 完全不可见 → 404（与「绑定不存在」不可区分）；
// 可见但缺这一个权限点 → 403（对方已经知道这个绑定存在，不算泄漏）。
func (s *Server) canSeeBinding(ctx context.Context, u *store.User, b *store.Binding) (bool, error) {
	if u.IsAdmin {
		return true, nil
	}

	owned, err := s.ownedAccountNames(ctx, u)
	if err != nil {
		return false, err
	}
	if owned[b.AccountName] {
		return true, nil
	}

	ms, err := s.store.ListMemberships(ctx, u.Username)
	if err != nil {
		return false, err
	}
	for _, m := range ms {
		if m.BindingID == b.ID {
			return true, nil
		}
	}
	return false, nil
}

// visibleBindings 返回调用者能看到的绑定。
//
// 可见 = 管理员，或账号的所有者，或在该绑定上有任意授权。
//
// **列表接口必须用它过滤，不得返回全部再让前端隐藏。**
// 这是最容易漏且后果最重的一类越权。
func (s *Server) visibleBindings(ctx context.Context, u *store.User) ([]store.Binding, error) {
	all, err := s.store.ListBindings(ctx)
	if err != nil {
		return nil, err
	}
	if u.IsAdmin {
		return all, nil
	}

	owned, err := s.ownedAccountNames(ctx, u)
	if err != nil {
		return nil, err
	}

	granted := make(map[int64]bool)
	ms, err := s.store.ListMemberships(ctx, u.Username)
	if err != nil {
		return nil, err
	}
	for _, m := range ms {
		granted[m.BindingID] = true
	}

	out := make([]store.Binding, 0, len(all))
	for _, b := range all {
		if owned[b.AccountName] || granted[b.ID] {
			out = append(out, b)
		}
	}
	return out, nil
}

// ownedAccountNames 返回该用户拥有的账号名集合。
func (s *Server) ownedAccountNames(ctx context.Context, u *store.User) (map[string]bool, error) {
	accs, err := s.store.ListAccounts(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(accs))
	for _, a := range accs {
		if a.OwnerID == u.ID {
			out[a.Name] = true
		}
	}
	return out, nil
}

// visibleAccountNames 返回调用者能看到的账号名集合。
//
// 可见 = 管理员，或自己拥有，或自己在其某个绑定上有任意授权。
func (s *Server) visibleAccountNames(ctx context.Context, u *store.User) (map[string]bool, error) {
	if u.IsAdmin {
		accs, err := s.store.ListAccounts(ctx)
		if err != nil {
			return nil, err
		}
		out := make(map[string]bool, len(accs))
		for _, a := range accs {
			out[a.Name] = true
		}
		return out, nil
	}

	out, err := s.ownedAccountNames(ctx, u)
	if err != nil {
		return nil, err
	}
	ms, err := s.store.ListMemberships(ctx, u.Username)
	if err != nil {
		return nil, err
	}
	for _, m := range ms {
		out[m.AccountName] = true
	}
	return out, nil
}
