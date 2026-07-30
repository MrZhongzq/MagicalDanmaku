package cmdmap

import (
	"encoding/json"
	"fmt"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

func init() {
	Register("LIVE", mapLive)
	Register("PREPARING", mapPreparing)
	Register("ROOM_CHANGE", mapRoomChange)
	Register("ROOM_BLOCK_MSG", mapRoomBlock)
	Register("ONLINE_RANK_V2", mapOnlineRankList)
	Register("ONLINE_RANK_TOP3", mapOnlineRankList)
	Register("ONLINE_RANK_COUNT", mapOnlineRankCount)
	Register("ROOM_REAL_TIME_MESSAGE_UPDATE", mapRealTimeUpdate)
	Register("WATCHED_CHANGE", mapWatchedChange)
	Register("LIKE_INFO_V3_UPDATE", mapLikeCountUpdate)
}

// int64Ptr 返回 v 的地址，用于 RoomStatsUpdate 的可选字段。
func int64Ptr(v int64) *int64 { return &v }

// unmarshalTop 解析顶层对象。LIVE、PREPARING 等 CMD 的字段不在 data 里。
func unmarshalTop(raw json.RawMessage, cmdName string) (map[string]any, error) {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("cmdmap: %s 解析失败: %w", cmdName, err)
	}
	return m, nil
}

// mapLive 解析开播消息。
func mapLive(ctx Context, raw json.RawMessage) ([]event.Event, error) {
	top, err := unmarshalTop(raw, "LIVE")
	if err != nil {
		return nil, err
	}
	ts := timeFromUnixSec(getInt64(top, "live_time"))
	return []event.Event{NewEvent(ctx, event.TypeLiveStart, ts, event.LiveStart{}, raw)}, nil
}

// mapPreparing 解析下播消息。
func mapPreparing(ctx Context, raw json.RawMessage) ([]event.Event, error) {
	return []event.Event{
		NewEvent(ctx, event.TypeLiveStop, ctx.ReceivedAt, event.LiveStop{}, raw),
	}, nil
}

// mapRoomChange 解析房间标题与分区变更。
func mapRoomChange(ctx Context, raw json.RawMessage) ([]event.Event, error) {
	data, err := unmarshalData(raw, "ROOM_CHANGE")
	if err != nil {
		return nil, err
	}
	r := event.RoomChange{
		Title:          getString(data, "title"),
		AreaID:         getString(data, "area_id"),
		AreaName:       getString(data, "area_name"),
		ParentAreaID:   getString(data, "parent_area_id"),
		ParentAreaName: getString(data, "parent_area_name"),
	}
	return []event.Event{NewEvent(ctx, event.TypeRoomChange, ctx.ReceivedAt, r, raw)}, nil
}

// mapRoomBlock 解析用户被禁言通知。
func mapRoomBlock(ctx Context, raw json.RawMessage) ([]event.Event, error) {
	top, err := unmarshalTop(raw, "ROOM_BLOCK_MSG")
	if err != nil {
		return nil, err
	}
	// 该 CMD 历史上同时在顶层与 data 里放用户信息，两处都要兼容。
	src := top
	if data := getObject(top, "data"); data != nil && getString(data, "uid") != "" {
		src = data
	}
	b := event.UserBlocked{
		User: event.User{
			UID:      getString(src, "uid"),
			Username: getString(src, "uname"),
		},
	}
	return []event.Event{NewEvent(ctx, event.TypeUserBlocked, ctx.ReceivedAt, b, raw)}, nil
}

// mapOnlineRankList 解析高能榜名次列表。
func mapOnlineRankList(ctx Context, raw json.RawMessage) ([]event.Event, error) {
	data, err := unmarshalData(raw, "ONLINE_RANK_V2")
	if err != nil {
		return nil, err
	}

	list := getArray(data, "list")
	top := make([]event.RankUser, 0, len(list))
	for _, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		top = append(top, event.RankUser{
			User: event.User{
				UID:        getString(m, "uid"),
				Username:   getString(m, "name"),
				AvatarURL:  getString(m, "face"),
				GuardLevel: int(getInt64(m, "guard_level")),
			},
			Rank:  int(getInt64(m, "rank")),
			Score: getString(m, "score"),
		})
	}

	// 本 CMD 不下发榜单总人数，用 -1 表示未知。
	r := event.OnlineRankUpdate{Count: -1, Top: top}
	return []event.Event{NewEvent(ctx, event.TypeOnlineRankUpdate, ctx.ReceivedAt, r, raw)}, nil
}

// mapOnlineRankCount 解析高能榜总人数变化。
func mapOnlineRankCount(ctx Context, raw json.RawMessage) ([]event.Event, error) {
	data, err := unmarshalData(raw, "ONLINE_RANK_COUNT")
	if err != nil {
		return nil, err
	}
	r := event.OnlineRankUpdate{Count: int(getInt64(data, "count"))}
	return []event.Event{NewEvent(ctx, event.TypeOnlineRankUpdate, ctx.ReceivedAt, r, raw)}, nil
}

// mapRealTimeUpdate 解析粉丝数与粉丝团人数变化。
func mapRealTimeUpdate(ctx Context, raw json.RawMessage) ([]event.Event, error) {
	data, err := unmarshalData(raw, "ROOM_REAL_TIME_MESSAGE_UPDATE")
	if err != nil {
		return nil, err
	}
	s := event.RoomStatsUpdate{
		Fans:     int64Ptr(getInt64(data, "fans")),
		FansClub: int64Ptr(getInt64(data, "fans_club")),
	}
	return []event.Event{NewEvent(ctx, event.TypeRoomStatsUpdate, ctx.ReceivedAt, s, raw)}, nil
}

// mapWatchedChange 解析累计看过人数变化。
func mapWatchedChange(ctx Context, raw json.RawMessage) ([]event.Event, error) {
	data, err := unmarshalData(raw, "WATCHED_CHANGE")
	if err != nil {
		return nil, err
	}
	s := event.RoomStatsUpdate{Watched: int64Ptr(getInt64(data, "num"))}
	return []event.Event{NewEvent(ctx, event.TypeRoomStatsUpdate, ctx.ReceivedAt, s, raw)}, nil
}

// mapLikeCountUpdate 解析点赞总数变化。
func mapLikeCountUpdate(ctx Context, raw json.RawMessage) ([]event.Event, error) {
	data, err := unmarshalData(raw, "LIKE_INFO_V3_UPDATE")
	if err != nil {
		return nil, err
	}
	s := event.RoomStatsUpdate{LikeCount: int64Ptr(getInt64(data, "click_count"))}
	return []event.Event{NewEvent(ctx, event.TypeRoomStatsUpdate, ctx.ReceivedAt, s, raw)}, nil
}
