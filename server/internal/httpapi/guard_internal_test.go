package httpapi

import (
	"testing"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/store"
)

// isAccountOwner 是账号所有权判定的唯一实现，直接判断所有权的处理器
// （扫码起始、PATCH 账号、DELETE 账号、建绑定、删绑定，以及 P5-6 新增的
// requireBindingOwner——账号级拉黑走的就是这条判定）都该收口到它，
// 不再各自重复 `!u.IsAdmin && acc.OwnerID != u.ID`。
func TestIsAccountOwner(t *testing.T) {
	s := &Server{}

	tests := []struct {
		name string
		u    *store.User
		acc  *store.Account
		want bool
	}{
		{"管理员总是放行，即便不是所有者",
			&store.User{ID: 1, IsAdmin: true}, &store.Account{OwnerID: 2}, true},
		{"所有者本人放行",
			&store.User{ID: 2}, &store.Account{OwnerID: 2}, true},
		{"既非管理员也非所有者，拒绝",
			&store.User{ID: 3}, &store.Account{OwnerID: 2}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := s.isAccountOwner(tt.u, tt.acc); got != tt.want {
				t.Errorf("isAccountOwner() = %v, 期望 %v", got, tt.want)
			}
		})
	}
}
