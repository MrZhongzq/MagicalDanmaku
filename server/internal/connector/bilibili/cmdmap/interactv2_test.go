package cmdmap

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

// pbVarint 追加一个 varint 字段。
func pbVarint(b []byte, field protowire.Number, v uint64) []byte {
	b = protowire.AppendTag(b, field, protowire.VarintType)
	return protowire.AppendVarint(b, v)
}

// pbString 追加一个字符串字段。
func pbString(b []byte, field protowire.Number, s string) []byte {
	b = protowire.AppendTag(b, field, protowire.BytesType)
	return protowire.AppendString(b, s)
}

// pbMessage 追加一个嵌套消息字段。
func pbMessage(b []byte, field protowire.Number, msg []byte) []byte {
	b = protowire.AppendTag(b, field, protowire.BytesType)
	return protowire.AppendBytes(b, msg)
}

// buildInteractV2 构造一条 INTERACT_WORD_V2 的 JSON 消息。
func buildInteractV2(pb []byte) json.RawMessage {
	payload := map[string]any{
		"cmd":  "INTERACT_WORD_V2",
		"data": map[string]any{"pb": base64.StdEncoding.EncodeToString(pb)},
	}
	raw, _ := json.Marshal(payload)
	return raw
}

func TestInteractV2Enter(t *testing.T) {
	// UserInfo.BaseInfo
	base := pbString(nil, 1, "进场用户")
	base = pbString(base, 2, "https://i0.hdslb.com/bfs/face/v2.jpg")
	// UserInfo.MedalInfo
	medal := pbString(nil, 1, "真yu中")
	medal = pbVarint(medal, 2, 24)  // medal_level
	medal = pbVarint(medal, 9, 1)   // is_lighted
	medal = pbVarint(medal, 10, 99) // ruid
	medal = pbVarint(medal, 11, 3)  // guard_level
	// UserInfo.WealthInfo
	wealth := pbVarint(nil, 1, 17)
	// UserInfo.GuardInfo
	guard := pbVarint(nil, 1, 3)
	guard = pbString(guard, 2, "2026-08-15 23:59:59")

	uinfo := pbVarint(nil, 1, 3546675027118505)
	uinfo = pbMessage(uinfo, 2, base)
	uinfo = pbMessage(uinfo, 3, medal)
	uinfo = pbMessage(uinfo, 4, wealth)
	uinfo = pbMessage(uinfo, 6, guard)

	// FansMedal
	fans := pbVarint(nil, 1, 99)      // target_id
	fans = pbVarint(fans, 12, 123456) // room_id

	pb := pbVarint(nil, 1, 3546675027118505) // uid
	pb = pbString(pb, 2, "进场用户")             // uname
	pb = pbVarint(pb, 5, 1)                  // msg_type = 1 进入
	pb = pbVarint(pb, 6, 21452505)           // room_id
	pb = pbVarint(pb, 7, 1700000000)         // timestamp
	pb = pbMessage(pb, 9, fans)
	pb = pbVarint(pb, 16, 3) // guard_level
	pb = pbMessage(pb, 22, uinfo)

	evs, err := Map(testCtx(), buildInteractV2(pb))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	if len(evs) != 1 || evs[0].Type != event.TypeUserEnter {
		t.Fatalf("结果错误: %+v", evs)
	}

	e := evs[0].Payload.(event.UserEnter)
	if e.User.UID != "3546675027118505" {
		t.Errorf("UID = %q", e.User.UID)
	}
	if e.User.Username != "进场用户" {
		t.Errorf("Username = %q", e.User.Username)
	}
	if e.User.AvatarURL != "https://i0.hdslb.com/bfs/face/v2.jpg" {
		t.Errorf("AvatarURL = %q", e.User.AvatarURL)
	}
	if e.User.GuardLevel != event.GuardCaptain {
		t.Errorf("GuardLevel = %d, 期望 3", e.User.GuardLevel)
	}
	if e.User.WealthLevel != 17 {
		t.Errorf("WealthLevel = %d, 期望 17", e.User.WealthLevel)
	}
	if e.User.Medal == nil {
		t.Fatal("Medal 不应为 nil")
	}
	if e.User.Medal.Name != "真yu中" || e.User.Medal.Level != 24 {
		t.Errorf("Medal = %+v", e.User.Medal)
	}
	if e.User.Medal.AnchorUID != "99" {
		t.Errorf("Medal.AnchorUID = %q", e.User.Medal.AnchorUID)
	}
	if e.User.Medal.RoomID != "123456" {
		t.Errorf("Medal.RoomID = %q，应取自 fans_medal.room_id", e.User.Medal.RoomID)
	}
	if !e.User.Medal.IsLighted {
		t.Error("IsLighted 应为 true")
	}
	if got := evs[0].Timestamp.Unix(); got != 1700000000 {
		t.Errorf("Timestamp = %d", got)
	}
}

func TestInteractV2MsgTypeDispatch(t *testing.T) {
	cases := []struct {
		msgType uint64
		want    event.Type
	}{
		{1, event.TypeUserEnter},
		{2, event.TypeUserFollow},
		{3, event.TypeUserShare},
		{4, event.TypeUserFollow},
		{99, event.TypeUserEnter}, // 未知取值保守回落
	}
	for _, tc := range cases {
		pb := pbVarint(nil, 1, 123)
		pb = pbString(pb, 2, "某人")
		pb = pbVarint(pb, 5, tc.msgType)
		pb = pbVarint(pb, 7, 1700000000)

		evs, err := Map(testCtx(), buildInteractV2(pb))
		if err != nil {
			t.Errorf("msg_type=%d Map 失败: %v", tc.msgType, err)
			continue
		}
		if len(evs) != 1 || evs[0].Type != tc.want {
			t.Errorf("msg_type=%d Type = %s, 期望 %s", tc.msgType, evs[0].Type, tc.want)
		}
	}
}

func TestInteractV2FallsBackToTopLevelUname(t *testing.T) {
	// 只有顶层 uname，没有 uinfo.base
	pb := pbVarint(nil, 1, 456)
	pb = pbString(pb, 2, "顶层昵称")
	pb = pbVarint(pb, 5, 1)
	pb = pbVarint(pb, 7, 1700000000)

	evs, err := Map(testCtx(), buildInteractV2(pb))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	e := evs[0].Payload.(event.UserEnter)
	if e.User.Username != "顶层昵称" {
		t.Errorf("Username = %q，缺 uinfo 时应回落到顶层 uname", e.User.Username)
	}
	if e.User.UID != "456" {
		t.Errorf("UID = %q", e.User.UID)
	}
}

func TestInteractV2RejectsBadPayload(t *testing.T) {
	raw := json.RawMessage(`{"cmd":"INTERACT_WORD_V2","data":{"pb":"!!!not-base64!!!"}}`)
	if _, err := Map(testCtx(), raw); err == nil {
		t.Error("非法 base64 应当报错")
	}
}

func TestInteractV2IgnoresUnknownFields(t *testing.T) {
	// 混入未来可能新增的字段，解码不得失败
	pb := pbVarint(nil, 1, 789)
	pb = pbString(pb, 2, "某人")
	pb = pbVarint(pb, 5, 1)
	pb = pbVarint(pb, 7, 1700000000)
	pb = pbString(pb, 77, "未来新增的字符串字段")
	pb = pbVarint(pb, 88, 12345)

	evs, err := Map(testCtx(), buildInteractV2(pb))
	if err != nil {
		t.Fatalf("遇到未知字段不应报错: %v", err)
	}
	if evs[0].Payload.(event.UserEnter).User.UID != "789" {
		t.Error("未知字段干扰了已知字段的解析")
	}
}
