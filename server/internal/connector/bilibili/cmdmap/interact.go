package cmdmap

import (
	"encoding/json"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

func init() {
	Register("INTERACT_WORD", mapInteractWord)
	Register("ENTRY_EFFECT", mapEntryEffect)
	// 老版进场消息，字段结构与 INTERACT_WORD 相近，统一按进场处理。
	Register("WELCOME", mapWelcome)
	Register("WELCOME_GUARD", mapWelcome)
	Register("LIKE_INFO_V3_CLICK", mapLikeClick)
}

// interactUser 从 INTERACT_WORD 系列的 data 中提取用户信息。
func interactUser(data map[string]any) event.User {
	medal := medalFrom(data)
	avatar, wealth := parseUinfo(data)

	u := event.User{
		UID:         getString(data, "uid"),
		Username:    getString(data, "uname"),
		AvatarURL:   avatar,
		WealthLevel: wealth,
		Medal:       medal,
	}
	// 这批 CMD 不单独下发本房间大航海等级，
	// 但佩戴本房间勋章时可从 fans_medal.guard_level 推得。
	if medal != nil {
		u.GuardLevel = medal.GuardLevel
	}
	return u
}

// mapInteractWord 解析互动消息，按 msg_type 分派到不同事件类型。
//
// msg_type: 1 进入直播间，2 关注，3 分享直播间，4 特别关注。
func mapInteractWord(ctx Context, raw json.RawMessage) ([]event.Event, error) {
	data, err := unmarshalData(raw, "INTERACT_WORD")
	if err != nil {
		return nil, err
	}

	u := interactUser(data)
	ts := timeFromUnixSec(getInt64(data, "timestamp"))

	var (
		typ event.Type
		p   event.Payload
	)
	switch getInt64(data, "msg_type") {
	case 2, 4: // 2 关注，4 特别关注
		typ, p = event.TypeUserFollow, event.UserFollow{User: u}
	case 3: // 分享直播间
		typ, p = event.TypeUserShare, event.UserShare{User: u}
	default: // 1 进入直播间，以及未知取值
		typ, p = event.TypeUserEnter, event.UserEnter{User: u}
	}

	return []event.Event{NewEvent(ctx, typ, ts, p, raw)}, nil
}

// mapEntryEffect 解析进场特效消息（舰长与高能榜用户进场时下发）。
//
// 该 CMD 不含昵称字段，昵称嵌在 copy_writing 的富文本里，
// 此处不做正则抠取——上层可凭 UID 查询，或直接消费 INTERACT_WORD。
func mapEntryEffect(ctx Context, raw json.RawMessage) ([]event.Event, error) {
	data, err := unmarshalData(raw, "ENTRY_EFFECT")
	if err != nil {
		return nil, err
	}

	u := event.User{
		UID:        getString(data, "uid"),
		AvatarURL:  getString(data, "face"),
		GuardLevel: int(getInt64(data, "privilege_type")),
	}

	return []event.Event{
		NewEvent(ctx, event.TypeUserEnter, ctx.ReceivedAt, event.UserEnter{User: u}, raw),
	}, nil
}

// mapWelcome 解析老版进场消息。
func mapWelcome(ctx Context, raw json.RawMessage) ([]event.Event, error) {
	data, err := unmarshalData(raw, "WELCOME")
	if err != nil {
		return nil, err
	}

	u := event.User{
		UID:        getString(data, "uid"),
		Username:   getString(data, "uname"),
		GuardLevel: int(getInt64(data, "guard_level")),
	}

	return []event.Event{
		NewEvent(ctx, event.TypeUserEnter, ctx.ReceivedAt, event.UserEnter{User: u}, raw),
	}, nil
}

// mapLikeClick 解析点赞消息。
func mapLikeClick(ctx Context, raw json.RawMessage) ([]event.Event, error) {
	data, err := unmarshalData(raw, "LIKE_INFO_V3_CLICK")
	if err != nil {
		return nil, err
	}

	u := interactUser(data)
	return []event.Event{
		NewEvent(ctx, event.TypeUserLike, ctx.ReceivedAt, event.UserLike{User: u}, raw),
	}, nil
}
