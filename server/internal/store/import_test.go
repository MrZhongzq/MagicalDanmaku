package store

import (
	"context"
	"testing"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/rules/spec"
)

func sampleImport() []ImportAccount {
	return []ImportAccount{{
		Name: "主播号", Cookie: "SESSDATA=a", RateLimit: 1500 * time.Millisecond, MaxLength: 40,
		Rooms: []ImportRoom{{
			RoomID:         "1706666491",
			CooldownGroups: map[string]time.Duration{"moderation": 5 * time.Second},
			Rules: []spec.Rule{
				{Name: "礼物流水", On: []string{"gift"}, Do: []spec.Action{{Type: "log"}}},
			},
		}},
	}, {
		Name: "小号", Cookie: "SESSDATA=b", RateLimit: 1500 * time.Millisecond,
		Rooms: []ImportRoom{{
			RoomID: "1706666491",
			Rules: []spec.Rule{
				{Name: "进场欢迎", On: []string{"user_enter"},
					Do: []spec.Action{{Type: "danmaku", Template: []string{"欢迎"}}}},
				{Name: "礼物答谢", On: []string{"gift"},
					Do: []spec.Action{{Type: "danmaku", Template: []string{"谢谢"}}}},
			},
		}, {
			RoomID: "22222222",
			Rules: []spec.Rule{
				{Name: "打招呼", On: []string{"user_enter"},
					Do: []spec.Action{{Type: "danmaku", Template: []string{"你好"}}}},
			},
		}},
	}}
}

func TestImportConfigCreatesEverything(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	owner := mustUser(t, s, "张三")

	res, err := s.ImportConfig(ctx, owner, sampleImport())
	if err != nil {
		t.Fatalf("导入报错: %v", err)
	}
	if res.Accounts != 2 || res.Bindings != 3 || res.Rules != 4 {
		t.Errorf("统计 = %+v, 期望 2 账号 / 3 绑定 / 4 规则", res)
	}

	cfgs, err := s.LoadRunConfig(ctx)
	if err != nil {
		t.Fatalf("载入运行配置报错: %v", err)
	}
	if len(cfgs) != 3 {
		t.Fatalf("运行单元数 = %d, 期望 3", len(cfgs))
	}
}

// 同一份 YAML 导两次，结果必须一致
func TestImportConfigIsIdempotent(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	owner := mustUser(t, s, "张三")

	if _, err := s.ImportConfig(ctx, owner, sampleImport()); err != nil {
		t.Fatalf("首次导入报错: %v", err)
	}
	res, err := s.ImportConfig(ctx, owner, sampleImport())
	if err != nil {
		t.Fatalf("二次导入报错: %v", err)
	}
	if res.Accounts != 2 || res.Bindings != 3 || res.Rules != 4 {
		t.Errorf("二次导入统计 = %+v", res)
	}

	as, err := s.ListAccounts(ctx)
	if err != nil {
		t.Fatalf("列出账号报错: %v", err)
	}
	if len(as) != 2 {
		t.Errorf("账号数 = %d, 期望 2（不该翻倍）", len(as))
	}
	bs, err := s.ListBindings(ctx)
	if err != nil {
		t.Fatalf("列出绑定报错: %v", err)
	}
	if len(bs) != 3 {
		t.Errorf("绑定数 = %d, 期望 3（不该翻倍）", len(bs))
	}
}

// YAML 里删掉的规则，重新导入后库里也该没有
func TestImportConfigDropsRemovedRules(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	owner := mustUser(t, s, "张三")

	if _, err := s.ImportConfig(ctx, owner, sampleImport()); err != nil {
		t.Fatalf("首次导入报错: %v", err)
	}

	trimmed := sampleImport()
	trimmed[1].Rooms[0].Rules = trimmed[1].Rooms[0].Rules[:1] // 小号删掉「礼物答谢」
	if _, err := s.ImportConfig(ctx, owner, trimmed); err != nil {
		t.Fatalf("二次导入报错: %v", err)
	}

	b, err := s.GetBinding(ctx, "小号", "1706666491")
	if err != nil {
		t.Fatalf("查询绑定报错: %v", err)
	}
	rs, err := s.ListRules(ctx, b.ID)
	if err != nil {
		t.Fatalf("列出规则报错: %v", err)
	}
	if len(rs) != 1 || rs[0].Name != "进场欢迎" {
		t.Errorf("删掉的规则应消失，实际 %+v", rs)
	}
}

// 导入不该把用户手动停用的绑定又打开
func TestImportConfigPreservesDisabledBinding(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	owner := mustUser(t, s, "张三")

	if _, err := s.ImportConfig(ctx, owner, sampleImport()); err != nil {
		t.Fatalf("首次导入报错: %v", err)
	}
	if err := s.SetBindingEnabled(ctx, "小号", "22222222", false); err != nil {
		t.Fatalf("停用绑定报错: %v", err)
	}
	if _, err := s.ImportConfig(ctx, owner, sampleImport()); err != nil {
		t.Fatalf("二次导入报错: %v", err)
	}

	b, err := s.GetBinding(ctx, "小号", "22222222")
	if err != nil {
		t.Fatalf("查询绑定报错: %v", err)
	}
	if b.Enabled {
		t.Error("导入不该把手动停用的绑定又打开")
	}
}

func TestImportConfigRejectsInvalidRule(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	owner := mustUser(t, s, "张三")

	bad := sampleImport()
	bad[0].Rooms[0].Rules[0].On = []string{"没有这种事件"}

	if _, err := s.ImportConfig(ctx, owner, bad); err == nil {
		t.Fatal("含非法规则的导入应失败")
	}
}

func TestImportConfigSetsCooldownGroups(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	owner := mustUser(t, s, "张三")

	if _, err := s.ImportConfig(ctx, owner, sampleImport()); err != nil {
		t.Fatalf("导入报错: %v", err)
	}

	b, err := s.GetBinding(ctx, "主播号", "1706666491")
	if err != nil {
		t.Fatalf("查询绑定报错: %v", err)
	}
	g, err := s.CooldownGroups(ctx, b.ID)
	if err != nil {
		t.Fatalf("读取冷却组报错: %v", err)
	}
	if g["moderation"] != 5*time.Second {
		t.Errorf("冷却组 = %v", g)
	}
}
