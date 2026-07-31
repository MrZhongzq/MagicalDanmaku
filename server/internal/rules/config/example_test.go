package config

import "testing"

// TestExampleConfigLoads 保证仓库根的示例配置始终可被加载。
//
// 示例配置是使用者的起点，若随模型演化而失效却无人察觉，
// 新用户第一步就会卡住。
func TestExampleConfigLoads(t *testing.T) {
	c, err := Load("../../../../config.example.yaml")
	if err != nil {
		t.Fatalf("示例配置应当可以加载: %v", err)
	}

	if len(c.Accounts) < 2 {
		t.Fatalf("示例应演示多账号场景，实际 %d 个账号", len(c.Accounts))
	}

	bs := c.Bindings()
	if len(bs) < 2 {
		t.Fatalf("示例应演示多绑定，实际 %d 个", len(bs))
	}

	// 示例演示的是职责分工：两个账号连同一个直播间，规则各不相同
	if bs[0].RoomID != bs[1].RoomID {
		t.Errorf("前两个绑定应指向同一直播间以演示职责分工，实际 %s 与 %s",
			bs[0].RoomID, bs[1].RoomID)
	}
	if bs[0].AccountName == bs[1].AccountName {
		t.Error("前两个绑定应属于不同账号")
	}

	// 禁言规则默认必须是关闭的——误禁言真实观众无法自动撤销
	for _, b := range bs {
		for _, r := range b.Rules {
			for _, a := range r.Do {
				if a.Type == "block" && r.Enabled {
					t.Errorf("示例中的禁言规则 %q 必须默认关闭", r.Name)
				}
			}
		}
	}
}
