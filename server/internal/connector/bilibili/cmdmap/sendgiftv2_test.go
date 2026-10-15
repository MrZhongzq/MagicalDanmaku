package cmdmap

import (
	"encoding/base64"
	"encoding/json"
	"math"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

// buildSendGiftV2 构造一条 SEND_GIFT_V2 的 JSON 消息。
func buildSendGiftV2(pb []byte) json.RawMessage {
	payload := map[string]any{
		"cmd":  "SEND_GIFT_V2",
		"data": map[string]any{"dmscore": 140, "pb": base64.StdEncoding.EncodeToString(pb)},
	}
	raw, _ := json.Marshal(payload)
	return raw
}

// giftItemV2 构造 gift_list 里的一份礼物，字段号与真实报文一致。
func giftItemV2(id uint64, name string, num, price, totalCoin, ts uint64) []byte {
	b := pbVarint(nil, 1, id)
	b = pbString(b, 2, name)
	b = pbVarint(b, 3, num)
	b = pbVarint(b, 4, 1) // demarcation，未使用
	b = pbVarint(b, 5, price)
	b = pbVarint(b, 6, price) // discount_price，未使用
	b = pbVarint(b, 7, totalCoin)
	b = pbString(b, 8, "gold")
	b = pbString(b, 9, "4828266350818586624") // tid
	b = pbVarint(b, 10, ts)
	b = pbString(b, 12, "batch:gift:combo_id:10001:20002:31164:1792092833.9243")
	// magnification 是 fixed32 浮点，真实报文里恒为 1.0，用来覆盖未知线型的跳过。
	b = protowire.AppendTag(b, 16, protowire.Fixed32Type)
	b = protowire.AppendFixed32(b, math.Float32bits(1))
	b = pbString(b, 18, "投喂")
	b = pbMessage(b, 29, pbVarint(pbString(nil, 1, "主播甲"), 2, 20002)) // receive_user_info
	return b
}

func TestSendGiftV2Basic(t *testing.T) {
	medal := pbVarint(nil, 1, 20002)    // target_id
	medal = pbString(medal, 3, "主播甲")   // anchor_uname
	medal = pbVarint(medal, 4, 30003)   // anchor_roomid
	medal = pbVarint(medal, 5, 21)      // medal_level
	medal = pbString(medal, 6, "甲粉")    // medal_name
	medal = pbVarint(medal, 7, 1725515) // medal_color，未使用
	medal = pbVarint(medal, 11, 1)      // is_lighted
	medal = pbVarint(medal, 12, 3)      // guard_level

	pb := pbVarint(nil, 1, 10001)
	pb = pbString(pb, 2, "送礼用户")
	pb = pbString(pb, 3, "https://i0.hdslb.com/bfs/face/v2gift.jpg")
	pb = pbString(pb, 4, "#FF7C28") // name_color，未使用
	pb = pbVarint(pb, 5, 3)         // guard_level
	pb = pbMessage(pb, 8, medal)
	pb = pbMessage(pb, 10, giftItemV2(31164, "粉丝团灯牌", 2, 100, 200, 1792092833))
	pb = pbVarint(pb, 11, 1)                     // switch
	pb = pbMessage(pb, 13, pbVarint(nil, 1, 32)) // wealth_info.level

	evs, err := Map(testCtx(), buildSendGiftV2(pb))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	if len(evs) != 1 || evs[0].Type != event.TypeGift {
		t.Fatalf("应产出 1 个 gift 事件，实际 %+v", evs)
	}

	g := evs[0].Payload.(event.Gift)
	if g.User.UID != "10001" || g.User.Username != "送礼用户" {
		t.Errorf("User = %+v", g.User)
	}
	if g.User.AvatarURL != "https://i0.hdslb.com/bfs/face/v2gift.jpg" {
		t.Errorf("AvatarURL = %q", g.User.AvatarURL)
	}
	if g.User.GuardLevel != event.GuardCaptain {
		t.Errorf("GuardLevel = %d, 期望 3", g.User.GuardLevel)
	}
	if g.User.WealthLevel != 32 {
		t.Errorf("WealthLevel = %d, 期望 32", g.User.WealthLevel)
	}
	want := event.Medal{Name: "甲粉", Level: 21, AnchorUID: "20002", AnchorName: "主播甲",
		RoomID: "30003", GuardLevel: 3, IsLighted: true}
	if g.User.Medal == nil || *g.User.Medal != want {
		t.Errorf("Medal = %+v, 期望 %+v", g.User.Medal, want)
	}
	if g.GiftID != 31164 || g.GiftName != "粉丝团灯牌" || g.Count != 2 {
		t.Errorf("礼物 = %d/%q/x%d", g.GiftID, g.GiftName, g.Count)
	}
	if g.CoinType != "gold" || g.Price != 100 || g.TotalCoin != 200 {
		t.Errorf("金额 = %s price=%d total=%d", g.CoinType, g.Price, g.TotalCoin)
	}
	if g.Action != "投喂" {
		t.Errorf("Action = %q", g.Action)
	}
	if g.BlindBox != nil {
		t.Errorf("普通礼物的 BlindBox 应为 nil，实际 %+v", g.BlindBox)
	}
	if got := evs[0].Timestamp.Unix(); got != 1792092833 {
		t.Errorf("Timestamp = %d", got)
	}
}

// TestSendGiftV2BlindBoxMultiItems 钉住两件事：盲盒爆出时一条广播带多份
// 礼物，每份各产出一个 gift 事件；盲盒信息整条广播共用一份。
func TestSendGiftV2BlindBoxMultiItems(t *testing.T) {
	blind := pbVarint(nil, 1, 139)    // blind_gift_config_id
	blind = pbVarint(blind, 2, 32251) // original_gift_id
	blind = pbString(blind, 3, "心动盲盒")
	blind = pbString(blind, 5, "爆出")
	blind = pbVarint(blind, 6, 15000) // original_gift_price；真实报文里没有 gift_tip_price(7)

	pb := pbVarint(nil, 1, 10001)
	pb = pbString(pb, 2, "开盒用户")
	pb = pbMessage(pb, 9, blind)
	pb = pbMessage(pb, 10, giftItemV2(32126, "棉花糖", 1, 9000, 15000, 1792092614))
	pb = pbMessage(pb, 10, giftItemV2(32125, "电影票", 3, 2000, 45000, 1792092616))
	pb = pbVarint(pb, 11, 1)

	evs, err := Map(testCtx(), buildSendGiftV2(pb))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	if len(evs) != 2 {
		t.Fatalf("两份礼物应产出 2 个事件，实际 %d", len(evs))
	}

	cases := []struct {
		name         string
		count, price int64
	}{{"棉花糖", 1, 9000}, {"电影票", 3, 2000}}
	for i, tc := range cases {
		g := evs[i].Payload.(event.Gift)
		if g.User.UID != "10001" || g.User.Username != "开盒用户" {
			t.Errorf("第 %d 份 User = %+v", i, g.User)
		}
		if g.GiftName != tc.name || g.Count != tc.count || g.Price != tc.price {
			t.Errorf("第 %d 份 = %q x%d price=%d", i, g.GiftName, g.Count, g.Price)
		}
		if g.BlindBox == nil {
			t.Fatalf("第 %d 份 BlindBox 不应为 nil", i)
		}
		if g.BlindBox.Name != "心动盲盒" || g.BlindBox.GiftID != 32251 || g.BlindBox.Price != 15000 {
			t.Errorf("第 %d 份 BlindBox = %+v", i, g.BlindBox)
		}
		// 与 SEND_GIFT 样本钉住的两条金额关系保持一致。
		if g.TotalCoin != g.BlindBox.Price*g.Count {
			t.Errorf("第 %d 份 total_coin(%d) != BlindBox.Price(%d) * num(%d)",
				i, g.TotalCoin, g.BlindBox.Price, g.Count)
		}
		if g.Price != g.BlindBox.TipPrice {
			t.Errorf("第 %d 份 Gift.Price(%d) != BlindBox.TipPrice(%d)", i, g.Price, g.BlindBox.TipPrice)
		}
	}
	if evs[1].Timestamp.Unix() != 1792092616 {
		t.Errorf("第 2 份 Timestamp 应取自它自己的 timestamp，实际 %d", evs[1].Timestamp.Unix())
	}
}

// TestSendGiftV2EmptyMedalIsNil：没戴勋章时 medal_info 是一个空消息，
// 不能产出一枚零值勋章。
func TestSendGiftV2EmptyMedalIsNil(t *testing.T) {
	pb := pbVarint(nil, 1, 10001)
	pb = pbString(pb, 2, "无牌用户")
	pb = pbMessage(pb, 8, nil)
	pb = pbMessage(pb, 10, giftItemV2(31164, "粉丝团灯牌", 1, 100, 100, 1792092833))
	pb = pbVarint(pb, 11, 1)

	evs, err := Map(testCtx(), buildSendGiftV2(pb))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	if g := evs[0].Payload.(event.Gift); g.User.Medal != nil {
		t.Errorf("Medal 应为 nil，实际 %+v", g.User.Medal)
	}
}

// TestSendGiftV2FallsBackToSenderUinfo：顶层缺昵称、头像时取 sender_uinfo.base。
func TestSendGiftV2FallsBackToSenderUinfo(t *testing.T) {
	base := pbString(nil, 1, "uinfo昵称")
	base = pbString(base, 2, "https://i0.hdslb.com/bfs/face/uinfo.jpg")
	uinfo := pbMessage(pbVarint(nil, 1, 10001), 2, base)

	pb := pbVarint(nil, 1, 10001)
	pb = pbMessage(pb, 10, giftItemV2(31164, "粉丝团灯牌", 1, 100, 100, 1792092833))
	pb = pbVarint(pb, 11, 1)
	pb = pbMessage(pb, 15, uinfo)

	evs, err := Map(testCtx(), buildSendGiftV2(pb))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	g := evs[0].Payload.(event.Gift)
	if g.User.Username != "uinfo昵称" || g.User.AvatarURL != "https://i0.hdslb.com/bfs/face/uinfo.jpg" {
		t.Errorf("User = %+v", g.User)
	}
}

// TestSendGiftV2SwitchOffProducesNothing：B 站网页端在 switch 为 false 时
// 不处理这条广播，这里与网页端保持一致，不投递事件。
func TestSendGiftV2SwitchOffProducesNothing(t *testing.T) {
	pb := pbVarint(nil, 1, 10001)
	pb = pbString(pb, 2, "某人")
	pb = pbMessage(pb, 10, giftItemV2(31164, "粉丝团灯牌", 1, 100, 100, 1792092833))

	evs, err := Map(testCtx(), buildSendGiftV2(pb))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	if len(evs) != 0 {
		t.Errorf("switch=false 不应产出事件，实际 %d 个", len(evs))
	}
}

func TestSendGiftV2RejectsBadPayload(t *testing.T) {
	raw := json.RawMessage(`{"cmd":"SEND_GIFT_V2","data":{"pb":"!!!not-base64!!!"}}`)
	if _, err := Map(testCtx(), raw); err == nil {
		t.Error("非法 base64 应当报错")
	}
}

// TestSendGiftV2GoldenHeartbeat 用结构照抄真实报文的样本（含未用到的嵌套
// 消息与 fixed32 字段）钉住端到端结果。
func TestSendGiftV2GoldenHeartbeat(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "SEND_GIFT_V2_blindbox_heartbeat"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	if len(evs) != 2 {
		t.Fatalf("应产出 2 个事件，实际 %d", len(evs))
	}
	g := evs[0].Payload.(event.Gift)
	if g.User.UID != "22222222" || g.User.Username != "开盒观众" || g.User.GuardLevel != event.GuardCaptain {
		t.Errorf("User = %+v", g.User)
	}
	if g.User.Medal == nil || g.User.Medal.Name != "DD粉丝" || g.User.Medal.Level != 29 ||
		g.User.Medal.AnchorUID != "99999999" || !g.User.Medal.IsLighted {
		t.Errorf("Medal = %+v", g.User.Medal)
	}
	if g.GiftName != "棉花糖" || g.Price != 9000 || g.TotalCoin != 15000 {
		t.Errorf("礼物 = %q price=%d total=%d", g.GiftName, g.Price, g.TotalCoin)
	}
	want := event.BlindBox{Name: "心动盲盒", GiftID: 32251, Price: 15000, TipPrice: 9000}
	if g.BlindBox == nil || *g.BlindBox != want {
		t.Errorf("BlindBox = %+v, 期望 %+v", g.BlindBox, want)
	}
	if g2 := evs[1].Payload.(event.Gift); g2.GiftName != "电影票" || g2.BlindBox.TipPrice != 2000 {
		t.Errorf("第 2 份 = %q TipPrice=%d", g2.GiftName, g2.BlindBox.TipPrice)
	}
}
