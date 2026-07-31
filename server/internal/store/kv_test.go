package store

import (
	"context"
	"testing"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules"
)

func TestBindingStorageImplementsRulesStorage(t *testing.T) {
	// 编译期断言：脚本沙箱注入的就是这个接口
	var _ rules.Storage = (*BindingStorage)(nil)
}

func TestBindingStorageSetAndGet(t *testing.T) {
	s := testStore(t)
	bid := mustBinding(t, s, "小号", "123")
	st := s.BindingStorage(bid)

	st.Set("累计礼物", "42")

	v, ok := st.Get("累计礼物")
	if !ok {
		t.Fatal("刚写入的键应读得到")
	}
	if v != "42" {
		t.Errorf("值 = %q, 期望 42", v)
	}
}

func TestBindingStorageGetMissingReturnsFalse(t *testing.T) {
	s := testStore(t)
	bid := mustBinding(t, s, "小号", "123")
	st := s.BindingStorage(bid)

	v, ok := st.Get("没写过的键")
	if ok {
		t.Error("未写过的键应返回 false")
	}
	if v != "" {
		t.Errorf("未写过的键应返回空串，实际 %q", v)
	}
}

func TestBindingStorageSetOverwrites(t *testing.T) {
	s := testStore(t)
	bid := mustBinding(t, s, "小号", "123")
	st := s.BindingStorage(bid)

	st.Set("k", "旧")
	st.Set("k", "新")

	v, _ := st.Get("k")
	if v != "新" {
		t.Errorf("值 = %q, 期望「新」", v)
	}
}

// 每个绑定有独立的命名空间，小号在甲房间写的键不该被乙房间读到
func TestBindingStorageIsolatedPerBinding(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	accID := mustAccount(t, s, "小号")

	b1, err := s.UpsertBinding(ctx, accID, "甲")
	if err != nil {
		t.Fatalf("创建绑定甲报错: %v", err)
	}
	b2, err := s.UpsertBinding(ctx, accID, "乙")
	if err != nil {
		t.Fatalf("创建绑定乙报错: %v", err)
	}

	s.BindingStorage(b1.ID).Set("计数", "10")

	if v, ok := s.BindingStorage(b2.ID).Get("计数"); ok {
		t.Errorf("绑定乙不该读到绑定甲的键，实际读到 %q", v)
	}
}

// storage 是脚本用的，写失败不能让脚本崩掉——记日志并继续
func TestBindingStorageSurvivesWriteFailure(t *testing.T) {
	s := testStore(t)
	// 绑定不存在，外键会挡住写入
	st := s.BindingStorage(999999)

	st.Set("k", "v") // 不应 panic

	if _, ok := st.Get("k"); ok {
		t.Error("写入失败后不该读到值")
	}
}
