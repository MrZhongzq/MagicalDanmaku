package cmdmap

import (
	"encoding/json"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

func init() {
	Register("SUPER_CHAT_MESSAGE", mapSuperChat)
	// 日文翻译版承载同一条 SC，字段结构一致。
	Register("SUPER_CHAT_MESSAGE_JPN", mapSuperChat)
	Register("SUPER_CHAT_MESSAGE_DELETE", mapSuperChatDelete)
}

// mapSuperChat 解析醒目留言。
func mapSuperChat(ctx Context, raw json.RawMessage) ([]event.Event, error) {
	data, err := unmarshalData(raw, "SUPER_CHAT_MESSAGE")
	if err != nil {
		return nil, err
	}

	u := event.User{
		UID:   getString(data, "uid"),
		Medal: medalFrom(data),
	}
	if ui := getObject(data, "user_info"); ui != nil {
		u.Username = getString(ui, "uname")
		u.AvatarURL = getString(ui, "face")
		u.GuardLevel = int(getInt64(ui, "guard_level"))
		u.UserLevel = int(getInt64(ui, "user_level"))
		u.IsAdmin = getInt64(ui, "manager") != 0
	}

	sc := event.SuperChat{
		User:     u,
		ID:       getInt64(data, "id"),
		Text:     getString(data, "message"),
		Price:    getInt64(data, "price"),
		Duration: int(getInt64(data, "time")),
	}

	ts := timeFromUnixSec(getInt64(data, "start_time"))
	return []event.Event{NewEvent(ctx, event.TypeSuperChat, ts, sc, raw)}, nil
}

// mapSuperChatDelete 解析醒目留言删除通知。
func mapSuperChatDelete(ctx Context, raw json.RawMessage) ([]event.Event, error) {
	data, err := unmarshalData(raw, "SUPER_CHAT_MESSAGE_DELETE")
	if err != nil {
		return nil, err
	}

	arr := getArray(data, "ids")
	ids := make([]int64, 0, len(arr))
	for _, v := range arr {
		ids = append(ids, toInt64(v))
	}

	d := event.SuperChatDelete{IDs: ids}
	return []event.Event{NewEvent(ctx, event.TypeSuperChatDelete, ctx.ReceivedAt, d, raw)}, nil
}
