# P0 协议内核 Implementation Plan · Part 2（CMD 映射）

> 续 `2026-07-29-p0-protocol-core.md`。执行前请先完成 Part 1 的 Task 1–5。
> Global Constraints 沿用 Part 1，此处不再重复。

本篇覆盖 Task 6–10，把剩余 CMD 全部映射完毕。每个任务的结构一致：
准备黄金样本 → 写失败测试 → 确认失败 → 实现 → 确认通过 → 提交。

所有任务共用 Part 1 Task 5 建立的 `TestGoldenSamplesAllMap` 框架：
往 `server/testdata/cmds/` 新增样本文件即自动纳入回归。

---

### Task 6: 礼物类映射（SEND_GIFT、COMBO_SEND）

**Files:**
- Create: `server/internal/connector/bilibili/cmdmap/gift.go`
- Create: `server/internal/connector/bilibili/cmdmap/medal.go`
- Create: `server/testdata/cmds/SEND_GIFT_silver.json`
- Create: `server/testdata/cmds/SEND_GIFT_gold_medal.json`
- Create: `server/testdata/cmds/COMBO_SEND_basic.json`
- Test: `server/internal/connector/bilibili/cmdmap/gift_test.go`

**Interfaces:**
- Consumes: Part 1 Task 4 的 `Context`、`Register`、`NewEvent`、JSON 助手；Part 1 Task 5 的 `loadSample`、`testCtx`
- Produces:
  - `SEND_GIFT`、`COMBO_SEND` 的注册映射
  - `parseMedalInfo(m map[string]any) *event.Medal` — 解析 `medal_info` / `fans_medal` 对象结构的勋章（**Task 7、8 会复用**）
  - `parseUinfo(data map[string]any) (avatar string, wealthLevel int)` — 解析新版 `uinfo` 对象（**Task 8 会复用**）

- [ ] **Step 1: 准备黄金样本**

创建 `server/testdata/cmds/SEND_GIFT_silver.json`：

```json
{
  "cmd": "SEND_GIFT",
  "data": {
    "uid": 20285041,
    "uname": "路人甲",
    "face": "http://i1.hdslb.com/bfs/face/aaa.jpg",
    "giftId": 30607,
    "giftName": "小心心",
    "num": 3,
    "coin_type": "silver",
    "total_coin": 0,
    "price": 0,
    "action": "投喂",
    "timestamp": 1614439816,
    "guard_level": 0,
    "batch_combo_id": "",
    "medal_info": {"medal_level": 0, "medal_name": "", "anchor_roomid": 0, "target_id": 0, "guard_level": 0, "is_lighted": 0, "anchor_uname": ""}
  }
}
```

创建 `server/testdata/cmds/SEND_GIFT_gold_medal.json`：

```json
{
  "cmd": "SEND_GIFT",
  "data": {
    "uid": 87654321,
    "uname": "土豪乙",
    "face": "http://i1.hdslb.com/bfs/face/bbb.jpg",
    "giftId": 31531,
    "giftName": "小花花",
    "num": 2,
    "coin_type": "gold",
    "total_coin": 2000,
    "price": 1000,
    "action": "投喂",
    "timestamp": 1700000200,
    "guard_level": 3,
    "batch_combo_id": "batch:gift:combo_id:87654321:123:31531:1700000200.1",
    "medal_info": {"medal_level": 25, "medal_name": "KKZ", "anchor_roomid": 1010, "anchor_uname": "某某主播", "target_id": 389088, "guard_level": 3, "is_lighted": 1}
  }
}
```

创建 `server/testdata/cmds/COMBO_SEND_basic.json`：

```json
{
  "cmd": "COMBO_SEND",
  "data": {
    "uid": 87654321,
    "uname": "土豪乙",
    "gift_id": 31531,
    "gift_name": "小花花",
    "combo_num": 5,
    "total_num": 5,
    "batch_combo_id": "batch:gift:combo_id:87654321:123:31531:1700000200.1",
    "combo_total_coin": 5000,
    "action": "投喂",
    "medal_info": {"medal_level": 25, "medal_name": "KKZ", "anchor_roomid": 1010, "anchor_uname": "某某主播", "target_id": 389088, "guard_level": 3, "is_lighted": 1}
  }
}
```

- [ ] **Step 2: 写失败测试**

创建 `server/internal/connector/bilibili/cmdmap/gift_test.go`：

```go
package cmdmap

import (
	"testing"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

func TestSendGiftSilver(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "SEND_GIFT_silver"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	if len(evs) != 1 || evs[0].Type != event.TypeGift {
		t.Fatalf("结果错误: %+v", evs)
	}

	g := evs[0].Payload.(event.Gift)
	if g.User.UID != "20285041" {
		t.Errorf("UID = %q", g.User.UID)
	}
	if g.User.Username != "路人甲" {
		t.Errorf("Username = %q", g.User.Username)
	}
	if g.User.AvatarURL != "http://i1.hdslb.com/bfs/face/aaa.jpg" {
		t.Errorf("AvatarURL = %q", g.User.AvatarURL)
	}
	if g.User.Medal != nil {
		t.Errorf("空勋章应解析为 nil，实际 %+v", g.User.Medal)
	}
	if g.GiftID != 30607 {
		t.Errorf("GiftID = %d", g.GiftID)
	}
	if g.GiftName != "小心心" {
		t.Errorf("GiftName = %q", g.GiftName)
	}
	if g.Count != 3 {
		t.Errorf("Count = %d, 期望 3", g.Count)
	}
	if g.CoinType != "silver" {
		t.Errorf("CoinType = %q", g.CoinType)
	}
	if g.TotalCoin != 0 {
		t.Errorf("TotalCoin = %d", g.TotalCoin)
	}
	if g.Action != "投喂" {
		t.Errorf("Action = %q", g.Action)
	}
	if got := evs[0].Timestamp.Unix(); got != 1614439816 {
		t.Errorf("Timestamp = %d", got)
	}
}

func TestSendGiftGoldWithMedal(t *testing.T) {
	evs, _ := Map(testCtx(), loadSample(t, "SEND_GIFT_gold_medal"))
	g := evs[0].Payload.(event.Gift)

	if g.CoinType != "gold" || g.TotalCoin != 2000 {
		t.Errorf("CoinType=%q TotalCoin=%d", g.CoinType, g.TotalCoin)
	}
	if g.User.GuardLevel != event.GuardCaptain {
		t.Errorf("GuardLevel = %d", g.User.GuardLevel)
	}
	if g.User.Medal == nil {
		t.Fatal("Medal 不应为 nil")
	}
	if g.User.Medal.Level != 25 || g.User.Medal.Name != "KKZ" {
		t.Errorf("Medal = %+v", g.User.Medal)
	}
	if g.User.Medal.AnchorName != "某某主播" {
		t.Errorf("Medal.AnchorName = %q", g.User.Medal.AnchorName)
	}
	if g.User.Medal.RoomID != "1010" {
		t.Errorf("Medal.RoomID = %q", g.User.Medal.RoomID)
	}
	if g.User.Medal.AnchorUID != "389088" {
		t.Errorf("Medal.AnchorUID = %q", g.User.Medal.AnchorUID)
	}
	if !g.User.Medal.IsLighted {
		t.Error("IsLighted 应为 true")
	}
}

func TestComboSend(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "COMBO_SEND_basic"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	if len(evs) != 1 || evs[0].Type != event.TypeGiftCombo {
		t.Fatalf("结果错误: %+v", evs)
	}

	c := evs[0].Payload.(event.GiftCombo)
	if c.User.UID != "87654321" {
		t.Errorf("UID = %q", c.User.UID)
	}
	if c.GiftName != "小花花" {
		t.Errorf("GiftName = %q", c.GiftName)
	}
	if c.Count != 5 {
		t.Errorf("Count = %d, 期望 5", c.Count)
	}
	if c.TotalCoin != 5000 {
		t.Errorf("TotalCoin = %d", c.TotalCoin)
	}
	if c.ComboID == "" {
		t.Error("ComboID 不应为空")
	}
}
```

- [ ] **Step 3: 运行测试确认失败**

```bash
cd server && go test ./internal/connector/bilibili/cmdmap/ -run 'TestSendGift|TestComboSend' -v
```

Expected: FAIL，事件落入 `TypeUnknown`。

- [ ] **Step 4: 实现勋章解析助手**

创建 `server/internal/connector/bilibili/cmdmap/medal.go`：

```go
package cmdmap

import "github.com/MrZhongzq/MagicalDanmaku/server/internal/event"

// parseMedalInfo 解析对象形式的勋章信息。
//
// B 站在不同 CMD 中用不同键名承载同一份数据：SEND_GIFT/SUPER_CHAT 用
// medal_info，INTERACT_WORD/GUARD_BUY 用 fans_medal，字段名基本一致。
// medal_level 为 0 表示用户未佩戴勋章，此时返回 nil。
func parseMedalInfo(m map[string]any) *event.Medal {
	if m == nil {
		return nil
	}
	level := int(getInt64(m, "medal_level"))
	if level == 0 {
		return nil
	}
	return &event.Medal{
		Name:       getString(m, "medal_name"),
		Level:      level,
		AnchorUID:  getString(m, "target_id"),
		AnchorName: getString(m, "anchor_uname"),
		RoomID:     getString(m, "anchor_roomid"),
		GuardLevel: int(getInt64(m, "guard_level")),
		IsLighted:  getBool(m, "is_lighted"),
	}
}

// medalFrom 依次尝试 medal_info 与 fans_medal 两个键。
func medalFrom(data map[string]any) *event.Medal {
	if m := parseMedalInfo(getObject(data, "medal_info")); m != nil {
		return m
	}
	return parseMedalInfo(getObject(data, "fans_medal"))
}

// parseUinfo 解析 2023 年后新增的 uinfo 对象，返回头像与荣耀等级。
// 该对象在 INTERACT_WORD、LIKE_INFO_V3_CLICK 等 CMD 中承载用户扩展信息。
func parseUinfo(data map[string]any) (avatar string, wealthLevel int) {
	uinfo := getObject(data, "uinfo")
	if uinfo == nil {
		return "", 0
	}
	if base := getObject(uinfo, "base"); base != nil {
		avatar = getString(base, "face")
	}
	if wealth := getObject(uinfo, "wealth"); wealth != nil {
		wealthLevel = int(getInt64(wealth, "level"))
	}
	return avatar, wealthLevel
}
```

- [ ] **Step 5: 实现礼物映射**

创建 `server/internal/connector/bilibili/cmdmap/gift.go`：

```go
package cmdmap

import (
	"encoding/json"
	"fmt"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

func init() {
	Register("SEND_GIFT", mapSendGift)
	Register("COMBO_SEND", mapComboSend)
}

// unmarshalData 提取消息的 data 对象。多数 CMD 的业务字段都在这一层。
func unmarshalData(raw json.RawMessage, cmdName string) (map[string]any, error) {
	var msg struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil, fmt.Errorf("cmdmap: %s 解析失败: %w", cmdName, err)
	}
	if msg.Data == nil {
		return nil, fmt.Errorf("cmdmap: %s 缺少 data 字段", cmdName)
	}
	return msg.Data, nil
}

// mapSendGift 解析送礼消息。
func mapSendGift(ctx Context, raw json.RawMessage) ([]event.Event, error) {
	data, err := unmarshalData(raw, "SEND_GIFT")
	if err != nil {
		return nil, err
	}

	g := event.Gift{
		User: event.User{
			UID:        getString(data, "uid"),
			Username:   getString(data, "uname"),
			AvatarURL:  getString(data, "face"),
			GuardLevel: int(getInt64(data, "guard_level")),
			Medal:      medalFrom(data),
		},
		GiftID:    getInt64(data, "giftId"),
		GiftName:  getString(data, "giftName"),
		Count:     getInt64(data, "num"),
		CoinType:  getString(data, "coin_type"),
		TotalCoin: getInt64(data, "total_coin"),
		Action:    getString(data, "action"),
	}

	ts := timeFromUnixSec(getInt64(data, "timestamp"))
	return []event.Event{NewEvent(ctx, event.TypeGift, ts, g, raw)}, nil
}

// mapComboSend 解析礼物连击汇总消息。
//
// 注意：COMBO_SEND 与其对应的多条 SEND_GIFT 是重复计数关系，
// 二者的合并去重属于 P2 规则引擎的职责，P0 只如实投递两种事件。
func mapComboSend(ctx Context, raw json.RawMessage) ([]event.Event, error) {
	data, err := unmarshalData(raw, "COMBO_SEND")
	if err != nil {
		return nil, err
	}

	count := getInt64(data, "combo_num")
	if count == 0 {
		count = getInt64(data, "total_num")
	}

	c := event.GiftCombo{
		User: event.User{
			UID:        getString(data, "uid"),
			Username:   getString(data, "uname"),
			GuardLevel: int(getInt64(data, "guard_level")),
			Medal:      medalFrom(data),
		},
		GiftID:    getInt64(data, "gift_id"),
		GiftName:  getString(data, "gift_name"),
		Count:     count,
		ComboID:   getString(data, "batch_combo_id"),
		TotalCoin: getInt64(data, "combo_total_coin"),
	}

	return []event.Event{NewEvent(ctx, event.TypeGiftCombo, ctx.ReceivedAt, c, raw)}, nil
}
```

- [ ] **Step 6: 运行测试确认通过**

```bash
cd server && go test ./internal/connector/bilibili/cmdmap/ -v
```

Expected: 全部 PASS，`TestGoldenSamplesAllMap` 已校验 5 个样本。

- [ ] **Step 7: 提交**

```bash
git add server/
git commit -m "feat: 实现礼物与连击 CMD 映射"
```

---

### Task 7: 舰长与醒目留言映射

**Files:**
- Create: `server/internal/connector/bilibili/cmdmap/guard.go`
- Create: `server/internal/connector/bilibili/cmdmap/super_chat.go`
- Create: `server/testdata/cmds/GUARD_BUY_basic.json`
- Create: `server/testdata/cmds/USER_TOAST_MSG_renew.json`
- Create: `server/testdata/cmds/SUPER_CHAT_MESSAGE_basic.json`
- Create: `server/testdata/cmds/SUPER_CHAT_MESSAGE_DELETE_basic.json`
- Test: `server/internal/connector/bilibili/cmdmap/guard_test.go`
- Test: `server/internal/connector/bilibili/cmdmap/super_chat_test.go`

**Interfaces:**
- Consumes: Task 6 的 `unmarshalData`、`medalFrom`、`parseMedalInfo`
- Produces: `GUARD_BUY`、`USER_TOAST_MSG`、`SUPER_CHAT_MESSAGE`、`SUPER_CHAT_MESSAGE_JPN`、`SUPER_CHAT_MESSAGE_DELETE` 的注册映射

**大航海等级换算：** `guard_level` 1=总督 2=提督 3=舰长，与 `event.Guard*` 常量一致，直接透传。

- [ ] **Step 1: 准备黄金样本**

创建 `server/testdata/cmds/GUARD_BUY_basic.json`：

```json
{
  "cmd": "GUARD_BUY",
  "data": {
    "uid": 67756641,
    "username": "新舰长",
    "gift_id": 10003,
    "gift_name": "舰长",
    "guard_level": 3,
    "num": 1,
    "price": 198000,
    "start_time": 1611343771,
    "end_time": 1611343771
  }
}
```

创建 `server/testdata/cmds/USER_TOAST_MSG_renew.json`：

```json
{
  "cmd": "USER_TOAST_MSG",
  "data": {
    "uid": 67756641,
    "username": "老舰长",
    "guard_level": 2,
    "num": 3,
    "price": 594000,
    "unit": "月",
    "role_name": "提督",
    "is_auto_renew": 1,
    "start_time": 1700000300,
    "toast_msg": "老舰长 自动续费了提督"
  }
}
```

创建 `server/testdata/cmds/SUPER_CHAT_MESSAGE_basic.json`：

```json
{
  "cmd": "SUPER_CHAT_MESSAGE",
  "data": {
    "id": 1278390,
    "uid": 389088,
    "message": "最右边可以爬上去",
    "price": 30,
    "time": 60,
    "start_time": 1613125845,
    "end_time": 1613125905,
    "user_info": {
      "uname": "SC用户",
      "face": "https://i0.hdslb.com/bfs/face/ccc.jpg",
      "guard_level": 3,
      "manager": 0,
      "user_level": 20
    },
    "medal_info": {
      "medal_level": 25,
      "medal_name": "KKZ",
      "anchor_roomid": 1010,
      "anchor_uname": "某某主播",
      "target_id": 389088,
      "guard_level": 3,
      "is_lighted": 1
    }
  }
}
```

创建 `server/testdata/cmds/SUPER_CHAT_MESSAGE_DELETE_basic.json`：

```json
{
  "cmd": "SUPER_CHAT_MESSAGE_DELETE",
  "data": {
    "ids": [1278390, 1278391]
  }
}
```

- [ ] **Step 2: 写失败测试**

创建 `server/internal/connector/bilibili/cmdmap/guard_test.go`：

```go
package cmdmap

import (
	"testing"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

func TestGuardBuy(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "GUARD_BUY_basic"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	if len(evs) != 1 || evs[0].Type != event.TypeGuardBuy {
		t.Fatalf("结果错误: %+v", evs)
	}

	g := evs[0].Payload.(event.GuardBuy)
	if g.User.UID != "67756641" {
		t.Errorf("UID = %q", g.User.UID)
	}
	if g.User.Username != "新舰长" {
		t.Errorf("Username = %q", g.User.Username)
	}
	if g.GuardLevel != event.GuardCaptain {
		t.Errorf("GuardLevel = %d, 期望 3", g.GuardLevel)
	}
	if g.GuardName != "舰长" {
		t.Errorf("GuardName = %q", g.GuardName)
	}
	if g.Count != 1 {
		t.Errorf("Count = %d", g.Count)
	}
	if g.Price != 198000 {
		t.Errorf("Price = %d", g.Price)
	}
	if g.IsRenew {
		t.Error("GUARD_BUY 是新购，IsRenew 应为 false")
	}
	// User.GuardLevel 也应被填上，方便下游统一取用
	if g.User.GuardLevel != event.GuardCaptain {
		t.Errorf("User.GuardLevel = %d, 期望 3", g.User.GuardLevel)
	}
	if got := evs[0].Timestamp.Unix(); got != 1611343771 {
		t.Errorf("Timestamp = %d", got)
	}
}

func TestUserToastMsgRenew(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "USER_TOAST_MSG_renew"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	if evs[0].Type != event.TypeGuardBuy {
		t.Fatalf("Type = %s, 期望 %s", evs[0].Type, event.TypeGuardBuy)
	}

	g := evs[0].Payload.(event.GuardBuy)
	if g.GuardLevel != event.GuardAdmiral {
		t.Errorf("GuardLevel = %d, 期望 2", g.GuardLevel)
	}
	if g.GuardName != "提督" {
		t.Errorf("GuardName = %q", g.GuardName)
	}
	if g.Count != 3 {
		t.Errorf("Count = %d, 期望 3", g.Count)
	}
	if !g.IsRenew {
		t.Error("is_auto_renew=1 时 IsRenew 应为 true")
	}
}
```

创建 `server/internal/connector/bilibili/cmdmap/super_chat_test.go`：

```go
package cmdmap

import (
	"testing"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

func TestSuperChatMessage(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "SUPER_CHAT_MESSAGE_basic"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	if len(evs) != 1 || evs[0].Type != event.TypeSuperChat {
		t.Fatalf("结果错误: %+v", evs)
	}

	sc := evs[0].Payload.(event.SuperChat)
	if sc.ID != 1278390 {
		t.Errorf("ID = %d", sc.ID)
	}
	if sc.Text != "最右边可以爬上去" {
		t.Errorf("Text = %q", sc.Text)
	}
	if sc.Price != 30 {
		t.Errorf("Price = %d", sc.Price)
	}
	if sc.Duration != 60 {
		t.Errorf("Duration = %d", sc.Duration)
	}
	if sc.User.UID != "389088" {
		t.Errorf("UID = %q", sc.User.UID)
	}
	if sc.User.Username != "SC用户" {
		t.Errorf("Username = %q", sc.User.Username)
	}
	if sc.User.AvatarURL != "https://i0.hdslb.com/bfs/face/ccc.jpg" {
		t.Errorf("AvatarURL = %q", sc.User.AvatarURL)
	}
	if sc.User.UserLevel != 20 {
		t.Errorf("UserLevel = %d", sc.User.UserLevel)
	}
	if sc.User.GuardLevel != event.GuardCaptain {
		t.Errorf("GuardLevel = %d", sc.User.GuardLevel)
	}
	if sc.User.Medal == nil || sc.User.Medal.Name != "KKZ" {
		t.Errorf("Medal = %+v", sc.User.Medal)
	}
	if got := evs[0].Timestamp.Unix(); got != 1613125845 {
		t.Errorf("Timestamp = %d", got)
	}
}

func TestSuperChatDelete(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "SUPER_CHAT_MESSAGE_DELETE_basic"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	if len(evs) != 1 || evs[0].Type != event.TypeSuperChatDelete {
		t.Fatalf("结果错误: %+v", evs)
	}

	d := evs[0].Payload.(event.SuperChatDelete)
	if len(d.IDs) != 2 || d.IDs[0] != 1278390 || d.IDs[1] != 1278391 {
		t.Errorf("IDs = %v", d.IDs)
	}
}
```

- [ ] **Step 3: 运行测试确认失败**

```bash
cd server && go test ./internal/connector/bilibili/cmdmap/ -run 'TestGuard|TestUserToast|TestSuperChat' -v
```

Expected: FAIL，事件落入 `TypeUnknown`。

- [ ] **Step 4: 实现舰长映射**

创建 `server/internal/connector/bilibili/cmdmap/guard.go`：

```go
package cmdmap

import (
	"encoding/json"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

func init() {
	Register("GUARD_BUY", mapGuardBuy)
	Register("USER_TOAST_MSG", mapUserToastMsg)
}

// guardName 把大航海等级转成中文名。
func guardName(level int) string {
	switch level {
	case event.GuardGovernor:
		return "总督"
	case event.GuardAdmiral:
		return "提督"
	case event.GuardCaptain:
		return "舰长"
	default:
		return ""
	}
}

// mapGuardBuy 解析新购大航海消息。
func mapGuardBuy(ctx Context, raw json.RawMessage) ([]event.Event, error) {
	data, err := unmarshalData(raw, "GUARD_BUY")
	if err != nil {
		return nil, err
	}

	level := int(getInt64(data, "guard_level"))
	name := getString(data, "gift_name")
	if name == "" {
		name = guardName(level)
	}

	g := event.GuardBuy{
		User: event.User{
			UID:        getString(data, "uid"),
			Username:   getString(data, "username"),
			GuardLevel: level,
			Medal:      medalFrom(data),
		},
		GuardLevel: level,
		GuardName:  name,
		Count:      int(getInt64(data, "num")),
		Price:      getInt64(data, "price"),
		IsRenew:    false,
	}

	ts := timeFromUnixSec(getInt64(data, "start_time"))
	return []event.Event{NewEvent(ctx, event.TypeGuardBuy, ts, g, raw)}, nil
}

// mapUserToastMsg 解析大航海续费消息。
//
// USER_TOAST_MSG 与 GUARD_BUY 描述同一类业务动作，归一化为同一事件类型，
// 用 IsRenew 区分新购与续费。
func mapUserToastMsg(ctx Context, raw json.RawMessage) ([]event.Event, error) {
	data, err := unmarshalData(raw, "USER_TOAST_MSG")
	if err != nil {
		return nil, err
	}

	level := int(getInt64(data, "guard_level"))
	name := getString(data, "role_name")
	if name == "" {
		name = guardName(level)
	}

	g := event.GuardBuy{
		User: event.User{
			UID:        getString(data, "uid"),
			Username:   getString(data, "username"),
			GuardLevel: level,
			Medal:      medalFrom(data),
		},
		GuardLevel: level,
		GuardName:  name,
		Count:      int(getInt64(data, "num")),
		Price:      getInt64(data, "price"),
		IsRenew:    true,
	}

	ts := timeFromUnixSec(getInt64(data, "start_time"))
	return []event.Event{NewEvent(ctx, event.TypeGuardBuy, ts, g, raw)}, nil
}
```

- [ ] **Step 5: 实现醒目留言映射**

创建 `server/internal/connector/bilibili/cmdmap/super_chat.go`：

```go
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
```

- [ ] **Step 6: 运行测试确认通过**

```bash
cd server && go test ./internal/connector/bilibili/cmdmap/ -v
```

Expected: 全部 PASS，已校验 9 个黄金样本。

- [ ] **Step 7: 提交**

```bash
git add server/
git commit -m "feat: 实现上舰与醒目留言 CMD 映射"
```

---

### Task 8: 互动类映射（进场、关注、分享、点赞）

**Files:**
- Create: `server/internal/connector/bilibili/cmdmap/interact.go`
- Create: `server/testdata/cmds/INTERACT_WORD_enter.json`
- Create: `server/testdata/cmds/INTERACT_WORD_follow.json`
- Create: `server/testdata/cmds/INTERACT_WORD_share.json`
- Create: `server/testdata/cmds/ENTRY_EFFECT_guard.json`
- Create: `server/testdata/cmds/LIKE_INFO_V3_CLICK_basic.json`
- Test: `server/internal/connector/bilibili/cmdmap/interact_test.go`

**Interfaces:**
- Consumes: Task 6 的 `unmarshalData`、`medalFrom`、`parseUinfo`
- Produces: `INTERACT_WORD`、`ENTRY_EFFECT`、`WELCOME`、`WELCOME_GUARD`、`LIKE_INFO_V3_CLICK` 的注册映射

**`INTERACT_WORD` 的 `msg_type` 分支**（源自原项目 `bili_livecmds.cpp:1657`）：

| msg_type | 归一化事件类型 |
|---|---|
| 1 | `TypeUserEnter`（进入直播间） |
| 2 | `TypeUserFollow`（关注） |
| 3 | `TypeUserShare`（分享直播间） |
| 4 | `TypeUserFollow`（特别关注，归一到关注） |
| 其他 | `TypeUserEnter`（保守回落） |

**关于 `INTERACT_WORD_V2`：** 该 CMD 的 `data` 是 base64 编码的 protobuf。P0 **暂不解码**，落入 `TypeUnknown` 由 `Raw` 兜底——`INTERACT_WORD` v1 目前仍在下发，信息不会丢失。protobuf 解码留到 P2，届时用 `protowire` 手工解码即可，无需引入 protoc 工具链（字段编号已记录在设计文档中）。

- [ ] **Step 1: 准备黄金样本**

创建 `server/testdata/cmds/INTERACT_WORD_enter.json`：

```json
{
  "cmd": "INTERACT_WORD",
  "data": {
    "msg_type": 1,
    "uid": 20285041,
    "uname": "进场用户",
    "uname_color": "",
    "timestamp": 1617974941,
    "roomid": 22639465,
    "identities": [1],
    "fans_medal": {
      "anchor_roomid": 0, "guard_level": 0, "is_lighted": 0,
      "medal_level": 0, "medal_name": "", "target_id": 0
    },
    "uinfo": {
      "base": {"face": "https://i0.hdslb.com/bfs/face/ddd.jpg", "uname": "进场用户"},
      "wealth": {"level": 12}
    }
  }
}
```

创建 `server/testdata/cmds/INTERACT_WORD_follow.json`：

```json
{
  "cmd": "INTERACT_WORD",
  "data": {
    "msg_type": 2,
    "uid": 33333333,
    "uname": "新粉丝",
    "timestamp": 1700000400,
    "roomid": 22639465,
    "fans_medal": {
      "anchor_roomid": 1010, "anchor_uname": "某某主播", "guard_level": 3,
      "is_lighted": 1, "medal_level": 15, "medal_name": "KKZ", "target_id": 389088
    }
  }
}
```

创建 `server/testdata/cmds/INTERACT_WORD_share.json`：

```json
{
  "cmd": "INTERACT_WORD",
  "data": {
    "msg_type": 3,
    "uid": 44444444,
    "uname": "分享用户",
    "timestamp": 1700000500,
    "roomid": 22639465,
    "fans_medal": {"medal_level": 0, "medal_name": ""}
  }
}
```

创建 `server/testdata/cmds/ENTRY_EFFECT_guard.json`：

```json
{
  "cmd": "ENTRY_EFFECT",
  "data": {
    "uid": 55555555,
    "copy_writing": "欢迎舰长 <%舰长用户%> 进入直播间",
    "face": "https://i0.hdslb.com/bfs/face/eee.jpg",
    "privilege_type": 3,
    "trigger_time": 1700000600000000000
  }
}
```

创建 `server/testdata/cmds/LIKE_INFO_V3_CLICK_basic.json`：

```json
{
  "cmd": "LIKE_INFO_V3_CLICK",
  "data": {
    "uid": 66666666,
    "uname": "点赞用户",
    "like_text": "为主播点赞了",
    "fans_medal": {"medal_level": 0, "medal_name": ""},
    "uinfo": {
      "base": {"face": "https://i0.hdslb.com/bfs/face/fff.jpg", "uname": "点赞用户"},
      "wealth": {"level": 5}
    }
  }
}
```

- [ ] **Step 2: 写失败测试**

创建 `server/internal/connector/bilibili/cmdmap/interact_test.go`：

```go
package cmdmap

import (
	"testing"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

func TestInteractWordEnter(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "INTERACT_WORD_enter"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	if len(evs) != 1 || evs[0].Type != event.TypeUserEnter {
		t.Fatalf("结果错误: %+v", evs)
	}

	e := evs[0].Payload.(event.UserEnter)
	if e.User.UID != "20285041" {
		t.Errorf("UID = %q", e.User.UID)
	}
	if e.User.Username != "进场用户" {
		t.Errorf("Username = %q", e.User.Username)
	}
	if e.User.AvatarURL != "https://i0.hdslb.com/bfs/face/ddd.jpg" {
		t.Errorf("AvatarURL = %q，应从 uinfo.base.face 取得", e.User.AvatarURL)
	}
	if e.User.WealthLevel != 12 {
		t.Errorf("WealthLevel = %d，应从 uinfo.wealth.level 取得", e.User.WealthLevel)
	}
	if e.User.Medal != nil {
		t.Errorf("medal_level=0 应解析为 nil，实际 %+v", e.User.Medal)
	}
	if got := evs[0].Timestamp.Unix(); got != 1617974941 {
		t.Errorf("Timestamp = %d", got)
	}
}

func TestInteractWordFollow(t *testing.T) {
	evs, _ := Map(testCtx(), loadSample(t, "INTERACT_WORD_follow"))
	if evs[0].Type != event.TypeUserFollow {
		t.Fatalf("Type = %s, 期望 %s", evs[0].Type, event.TypeUserFollow)
	}
	f := evs[0].Payload.(event.UserFollow)
	if f.User.Username != "新粉丝" {
		t.Errorf("Username = %q", f.User.Username)
	}
	if f.User.Medal == nil || f.User.Medal.Level != 15 {
		t.Errorf("Medal = %+v", f.User.Medal)
	}
	if f.User.GuardLevel != event.GuardCaptain {
		t.Errorf("GuardLevel = %d，应从 fans_medal.guard_level 取得", f.User.GuardLevel)
	}
}

func TestInteractWordShare(t *testing.T) {
	evs, _ := Map(testCtx(), loadSample(t, "INTERACT_WORD_share"))
	if evs[0].Type != event.TypeUserShare {
		t.Fatalf("Type = %s, 期望 %s", evs[0].Type, event.TypeUserShare)
	}
	if _, ok := evs[0].Payload.(event.UserShare); !ok {
		t.Fatalf("载荷类型 = %T", evs[0].Payload)
	}
}

func TestEntryEffect(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "ENTRY_EFFECT_guard"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	if evs[0].Type != event.TypeUserEnter {
		t.Fatalf("Type = %s, 期望 %s", evs[0].Type, event.TypeUserEnter)
	}

	e := evs[0].Payload.(event.UserEnter)
	if e.User.UID != "55555555" {
		t.Errorf("UID = %q", e.User.UID)
	}
	if e.User.GuardLevel != event.GuardCaptain {
		t.Errorf("GuardLevel = %d，应从 privilege_type 取得", e.User.GuardLevel)
	}
	if e.User.AvatarURL != "https://i0.hdslb.com/bfs/face/eee.jpg" {
		t.Errorf("AvatarURL = %q", e.User.AvatarURL)
	}
	// ENTRY_EFFECT 不含昵称字段，允许为空
	if e.User.Username != "" {
		t.Errorf("Username = %q, 期望空串", e.User.Username)
	}
}

func TestLikeInfoClick(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "LIKE_INFO_V3_CLICK_basic"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	if evs[0].Type != event.TypeUserLike {
		t.Fatalf("Type = %s, 期望 %s", evs[0].Type, event.TypeUserLike)
	}
	l := evs[0].Payload.(event.UserLike)
	if l.User.Username != "点赞用户" {
		t.Errorf("Username = %q", l.User.Username)
	}
	if l.User.WealthLevel != 5 {
		t.Errorf("WealthLevel = %d", l.User.WealthLevel)
	}
}
```

- [ ] **Step 3: 运行测试确认失败**

```bash
cd server && go test ./internal/connector/bilibili/cmdmap/ -run 'TestInteract|TestEntryEffect|TestLikeInfo' -v
```

Expected: FAIL，事件落入 `TypeUnknown`。

- [ ] **Step 4: 实现**

创建 `server/internal/connector/bilibili/cmdmap/interact.go`：

```go
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
```

- [ ] **Step 5: 运行测试确认通过**

```bash
cd server && go test ./internal/connector/bilibili/cmdmap/ -v
```

Expected: 全部 PASS，已校验 14 个黄金样本。

- [ ] **Step 6: 提交**

```bash
git add server/
git commit -m "feat: 实现进场关注分享点赞 CMD 映射"
```

---

### Task 9: 房间状态类映射

**Files:**
- Create: `server/internal/connector/bilibili/cmdmap/room.go`
- Create: `server/testdata/cmds/LIVE_basic.json`
- Create: `server/testdata/cmds/PREPARING_basic.json`
- Create: `server/testdata/cmds/ROOM_CHANGE_basic.json`
- Create: `server/testdata/cmds/ROOM_BLOCK_MSG_basic.json`
- Create: `server/testdata/cmds/ONLINE_RANK_COUNT_basic.json`
- Create: `server/testdata/cmds/ONLINE_RANK_V2_basic.json`
- Create: `server/testdata/cmds/ROOM_REAL_TIME_MESSAGE_UPDATE_basic.json`
- Create: `server/testdata/cmds/WATCHED_CHANGE_basic.json`
- Test: `server/internal/connector/bilibili/cmdmap/room_test.go`

**Interfaces:**
- Consumes: Task 6 的 `unmarshalData`
- Produces: `LIVE`、`PREPARING`、`ROOM_CHANGE`、`ROOM_BLOCK_MSG`、`ONLINE_RANK_V2`、`ONLINE_RANK_TOP3`、`ONLINE_RANK_COUNT`、`ROOM_REAL_TIME_MESSAGE_UPDATE`、`WATCHED_CHANGE`、`LIKE_INFO_V3_UPDATE` 的注册映射；辅助函数 `int64Ptr(v int64) *int64`

**注意：** `LIVE` 与 `PREPARING` 的业务字段在**顶层**，不在 `data` 里。

- [ ] **Step 1: 准备黄金样本**

创建 `server/testdata/cmds/LIVE_basic.json`：

```json
{"cmd": "LIVE", "live_key": "123456", "live_time": 1700000000, "roomid": 21452505}
```

创建 `server/testdata/cmds/PREPARING_basic.json`：

```json
{"cmd": "PREPARING", "roomid": "21452505"}
```

创建 `server/testdata/cmds/ROOM_CHANGE_basic.json`：

```json
{
  "cmd": "ROOM_CHANGE",
  "data": {
    "title": "今天也在唱歌",
    "area_id": 21,
    "area_name": "视频唱见",
    "parent_area_id": 1,
    "parent_area_name": "娱乐"
  }
}
```

创建 `server/testdata/cmds/ROOM_BLOCK_MSG_basic.json`：

```json
{
  "cmd": "ROOM_BLOCK_MSG",
  "uid": 99999999,
  "uname": "被禁言的人",
  "data": {"uid": 99999999, "uname": "被禁言的人", "operator": 1}
}
```

创建 `server/testdata/cmds/ONLINE_RANK_COUNT_basic.json`：

```json
{"cmd": "ONLINE_RANK_COUNT", "data": {"count": 233}}
```

创建 `server/testdata/cmds/ONLINE_RANK_V2_basic.json`：

```json
{
  "cmd": "ONLINE_RANK_V2",
  "data": {
    "rank_type": "gold-rank",
    "list": [
      {"uid": 111, "name": "榜一", "face": "https://i0.hdslb.com/bfs/face/g1.jpg", "rank": 1, "score": "12000", "guard_level": 3},
      {"uid": 222, "name": "榜二", "face": "https://i0.hdslb.com/bfs/face/g2.jpg", "rank": 2, "score": "8000", "guard_level": 0}
    ]
  }
}
```

创建 `server/testdata/cmds/ROOM_REAL_TIME_MESSAGE_UPDATE_basic.json`：

```json
{
  "cmd": "ROOM_REAL_TIME_MESSAGE_UPDATE",
  "data": {"roomid": 21452505, "fans": 12345, "fans_club": 678, "red_notice": -1}
}
```

创建 `server/testdata/cmds/WATCHED_CHANGE_basic.json`：

```json
{
  "cmd": "WATCHED_CHANGE",
  "data": {"num": 4567, "text_small": "4567", "text_large": "4567人看过"}
}
```

- [ ] **Step 2: 写失败测试**

创建 `server/internal/connector/bilibili/cmdmap/room_test.go`：

```go
package cmdmap

import (
	"testing"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

func TestLiveStart(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "LIVE_basic"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	if len(evs) != 1 || evs[0].Type != event.TypeLiveStart {
		t.Fatalf("结果错误: %+v", evs)
	}
	if _, ok := evs[0].Payload.(event.LiveStart); !ok {
		t.Fatalf("载荷类型 = %T", evs[0].Payload)
	}
	// live_time 在顶层，应被用作事件时间
	if got := evs[0].Timestamp.Unix(); got != 1700000000 {
		t.Errorf("Timestamp = %d, 期望 1700000000", got)
	}
}

func TestLiveStop(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "PREPARING_basic"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	if evs[0].Type != event.TypeLiveStop {
		t.Fatalf("Type = %s, 期望 %s", evs[0].Type, event.TypeLiveStop)
	}
}

func TestRoomChange(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "ROOM_CHANGE_basic"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	r := evs[0].Payload.(event.RoomChange)
	if r.Title != "今天也在唱歌" {
		t.Errorf("Title = %q", r.Title)
	}
	if r.AreaID != "21" || r.AreaName != "视频唱见" {
		t.Errorf("Area = %q/%q", r.AreaID, r.AreaName)
	}
	if r.ParentAreaID != "1" || r.ParentAreaName != "娱乐" {
		t.Errorf("ParentArea = %q/%q", r.ParentAreaID, r.ParentAreaName)
	}
}

func TestRoomBlockMsg(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "ROOM_BLOCK_MSG_basic"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	if evs[0].Type != event.TypeUserBlocked {
		t.Fatalf("Type = %s", evs[0].Type)
	}
	b := evs[0].Payload.(event.UserBlocked)
	if b.User.UID != "99999999" || b.User.Username != "被禁言的人" {
		t.Errorf("User = %+v", b.User)
	}
}

func TestOnlineRankCount(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "ONLINE_RANK_COUNT_basic"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	r := evs[0].Payload.(event.OnlineRankUpdate)
	if r.Count != 233 {
		t.Errorf("Count = %d, 期望 233", r.Count)
	}
	if len(r.Top) != 0 {
		t.Errorf("Top 应为空，实际 %d 项", len(r.Top))
	}
}

func TestOnlineRankV2(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "ONLINE_RANK_V2_basic"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	r := evs[0].Payload.(event.OnlineRankUpdate)
	if r.Count != -1 {
		t.Errorf("未下发总数时 Count 应为 -1，实际 %d", r.Count)
	}
	if len(r.Top) != 2 {
		t.Fatalf("Top 项数 = %d, 期望 2", len(r.Top))
	}
	if r.Top[0].User.UID != "111" || r.Top[0].Rank != 1 || r.Top[0].Score != "12000" {
		t.Errorf("Top[0] = %+v", r.Top[0])
	}
	if r.Top[0].User.GuardLevel != event.GuardCaptain {
		t.Errorf("Top[0].GuardLevel = %d", r.Top[0].User.GuardLevel)
	}
	if r.Top[1].User.Username != "榜二" {
		t.Errorf("Top[1].Username = %q", r.Top[1].User.Username)
	}
}

func TestRoomRealTimeMessageUpdate(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "ROOM_REAL_TIME_MESSAGE_UPDATE_basic"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	s := evs[0].Payload.(event.RoomStatsUpdate)
	if s.Fans == nil || *s.Fans != 12345 {
		t.Errorf("Fans = %v", s.Fans)
	}
	if s.FansClub == nil || *s.FansClub != 678 {
		t.Errorf("FansClub = %v", s.FansClub)
	}
	if s.Watched != nil {
		t.Errorf("本 CMD 不含 Watched，应为 nil，实际 %v", *s.Watched)
	}
}

func TestWatchedChange(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "WATCHED_CHANGE_basic"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	s := evs[0].Payload.(event.RoomStatsUpdate)
	if s.Watched == nil || *s.Watched != 4567 {
		t.Errorf("Watched = %v", s.Watched)
	}
	if s.Fans != nil {
		t.Error("本 CMD 不含 Fans，应为 nil")
	}
}
```

- [ ] **Step 3: 运行测试确认失败**

```bash
cd server && go test ./internal/connector/bilibili/cmdmap/ -run 'TestLive|TestRoom|TestOnlineRank|TestWatched' -v
```

Expected: FAIL，事件落入 `TypeUnknown`。

- [ ] **Step 4: 实现**

创建 `server/internal/connector/bilibili/cmdmap/room.go`：

```go
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
```

- [ ] **Step 5: 运行测试确认通过**

```bash
cd server && go test ./internal/connector/bilibili/cmdmap/ -v
```

Expected: 全部 PASS，已校验 22 个黄金样本。

- [ ] **Step 6: 提交**

```bash
git add server/
git commit -m "feat: 实现房间状态类 CMD 映射"
```

---

### Task 10: PK 大乱斗归一化与噪声 CMD 忽略

**Files:**
- Create: `server/internal/connector/bilibili/cmdmap/battle.go`
- Create: `server/internal/connector/bilibili/cmdmap/ignored.go`
- Create: `server/testdata/cmds/PK_BATTLE_START_NEW_basic.json`
- Test: `server/internal/connector/bilibili/cmdmap/battle_test.go`

**Interfaces:**
- Consumes: Task 4 的 `Register`、`NewEvent`、`CommandOf`
- Produces: 全部 `PK_BATTLE_*` 的注册映射（归一为 `TypeBattle`）；噪声 CMD 的忽略注册（返回空切片）

**设计要点：** P0 不解释 PK 业务语义，只把 CMD 名放进 `Battle.SubCommand`，完整数据留在 `Raw` 里给 P6 消费。这样 P6 开发时无需改动 P0。

**噪声 CMD** 指广告横幅、运营活动一类与弹幕机器人无关的高频消息。它们返回空切片被静默丢弃——这是**唯一**允许丢弃消息的场景，且必须显式列举，不得使用通配。

- [ ] **Step 1: 准备黄金样本**

创建 `server/testdata/cmds/PK_BATTLE_START_NEW_basic.json`：

```json
{
  "cmd": "PK_BATTLE_START_NEW",
  "pk_id": 12345678,
  "pk_status": 201,
  "timestamp": 1700000700,
  "data": {
    "battle_type": 2,
    "init_info": {"room_id": 21452505, "best_uname": ""},
    "match_info": {"room_id": 33333, "best_uname": "对面主播"},
    "pk_start_time": 1700000700,
    "pk_end_time": 1700001000
  }
}
```

- [ ] **Step 2: 写失败测试**

创建 `server/internal/connector/bilibili/cmdmap/battle_test.go`：

```go
package cmdmap

import (
	"encoding/json"
	"testing"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

func TestBattleNormalized(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "PK_BATTLE_START_NEW_basic"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	if len(evs) != 1 || evs[0].Type != event.TypeBattle {
		t.Fatalf("结果错误: %+v", evs)
	}

	b := evs[0].Payload.(event.Battle)
	if b.SubCommand != "PK_BATTLE_START_NEW" {
		t.Errorf("SubCommand = %q", b.SubCommand)
	}
	// 完整数据必须留在 Raw 里供 P6 使用
	if !json.Valid(evs[0].Raw) {
		t.Error("Raw 必须是合法 JSON")
	}
	var probe map[string]any
	if err := json.Unmarshal(evs[0].Raw, &probe); err != nil {
		t.Fatalf("Raw 解析失败: %v", err)
	}
	if probe["pk_id"] == nil {
		t.Error("Raw 中应保留 pk_id")
	}
}

func TestAllBattleCommandsRegistered(t *testing.T) {
	for _, name := range battleCommands {
		raw := json.RawMessage(`{"cmd":"` + name + `"}`)
		evs, err := Map(testCtx(), raw)
		if err != nil {
			t.Errorf("%s: Map 失败: %v", name, err)
			continue
		}
		if len(evs) != 1 || evs[0].Type != event.TypeBattle {
			t.Errorf("%s: 未归一化为 Battle，实际 %+v", name, evs)
		}
	}
}

func TestIgnoredCommandsProduceNoEvents(t *testing.T) {
	for _, name := range ignoredCommands {
		raw := json.RawMessage(`{"cmd":"` + name + `"}`)
		evs, err := Map(testCtx(), raw)
		if err != nil {
			t.Errorf("%s: Map 失败: %v", name, err)
			continue
		}
		if len(evs) != 0 {
			t.Errorf("%s: 应被忽略，实际产出 %d 个事件", name, len(evs))
		}
	}
}
```

- [ ] **Step 3: 运行测试确认失败**

```bash
cd server && go test ./internal/connector/bilibili/cmdmap/ -run 'TestBattle|TestAllBattle|TestIgnored' -v
```

Expected: 编译失败，`undefined: battleCommands`。

- [ ] **Step 4: 实现 PK 归一化**

创建 `server/internal/connector/bilibili/cmdmap/battle.go`：

```go
package cmdmap

import (
	"encoding/json"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

// battleCommands 是全部 PK 大乱斗相关 CMD。
//
// P0 只把它们归一化为 TypeBattle 并保留原始数据，不解释业务语义。
// PK 的状态机、偷塔、串门等逻辑属于 P6，届时从 Event.Raw 取数即可，
// 无需改动本文件。
var battleCommands = []string{
	"PK_BATTLE_PRE",
	"PK_BATTLE_PRE_NEW",
	"PK_BATTLE_START",
	"PK_BATTLE_START_NEW",
	"PK_BATTLE_PROCESS",
	"PK_BATTLE_PROCESS_NEW",
	"PK_BATTLE_RANK_CHANGE",
	"PK_BATTLE_FINAL_PROCESS",
	"PK_BATTLE_END",
	"PK_BATTLE_SETTLE",
	"PK_BATTLE_SETTLE_NEW",
	"PK_BATTLE_SETTLE_USER",
	"PK_BATTLE_SETTLE_V2",
	"PK_BATTLE_PUNISH_END",
	"PK_BATTLE_MATCH_TIMEOUT",
	"PK_BATTLE_ENTRANCE",
	"PK_BATTLE_VIDEO_PUNISH_BEGIN",
	"PK_BATTLE_VIDEO_PUNISH_END",
	"PK_LOTTERY_START",
}

func init() {
	for _, name := range battleCommands {
		Register(name, mapBattle)
	}
}

// mapBattle 把 PK 相关 CMD 归一化为 Battle 事件。
func mapBattle(ctx Context, raw json.RawMessage) ([]event.Event, error) {
	b := event.Battle{SubCommand: CommandOf(raw)}
	return []event.Event{NewEvent(ctx, event.TypeBattle, ctx.ReceivedAt, b, raw)}, nil
}
```

- [ ] **Step 5: 实现噪声 CMD 忽略**

创建 `server/internal/connector/bilibili/cmdmap/ignored.go`：

```go
package cmdmap

import (
	"encoding/json"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

// ignoredCommands 是与弹幕机器人无关的高频噪声 CMD。
//
// 这是本项目中唯一允许丢弃消息的场景，因此必须逐条显式列举，
// 不得使用前缀或通配匹配——否则 B 站新增的有用 CMD 会被误吞。
// 未列出的 CMD 一律走 Unknown 兜底，仍会投递给上层。
var ignoredCommands = []string{
	"WIDGET_BANNER",              // 活动横幅
	"ROOM_BANNER",                // 房间横幅
	"ACTIVITY_BANNER_UPDATE_V2",  // 活动横幅更新
	"PANEL",                      // 面板数据
	"ONLINERANK",                 // 旧版高能榜，已被 ONLINE_RANK_V2 取代
	"STOP_LIVE_ROOM_LIST",        // 全站下播房间列表广播
	"NOTICE_MSG",                 // 全站通告广播
	"HOT_ROOM_NOTIFY",            // 热门房间提示
	"WIDGET_GIFT_STAR_PROCESS",   // 礼物星球进度
	"LIVE_INTERACTIVE_GAME",      // 互动游戏内部消息
	"POPULARITY_RED_POCKET_START", // 红包活动
	"POPULARITY_RED_POCKET_NEW",
	"POPULARITY_RED_POCKET_WINNER_LIST",
	"AREA_RANK_CHANGED",          // 分区排行变化
	"COMMON_NOTICE_DANMAKU",      // 系统通知弹幕
	"LOG_IN_NOTICE",              // 登录提示
	"SPREAD_SHOW_FEET_V2",        // 推广位
	"RECOMMEND_CARD",             // 推荐卡片
}

func init() {
	for _, name := range ignoredCommands {
		Register(name, mapIgnored)
	}
}

// mapIgnored 静默丢弃噪声消息。
func mapIgnored(_ Context, _ json.RawMessage) ([]event.Event, error) {
	return nil, nil
}
```

- [ ] **Step 6: 运行测试确认通过**

```bash
cd server && go test ./internal/connector/bilibili/cmdmap/ -v
```

Expected: 全部 PASS，已校验 23 个黄金样本。

- [ ] **Step 7: 运行完整测试与 vet**

```bash
cd server && go vet ./... && go test ./... 
```

Expected: 无 vet 输出，全部测试 PASS。

- [ ] **Step 8: 提交**

```bash
git add server/
git commit -m "feat: 归一化 PK 大乱斗 CMD 并显式忽略噪声消息"
```

---

**下一步：** CMD 映射层至此完成。继续阅读 `2026-07-29-p0-protocol-core-part3.md`，实现认证、连接与动作执行。
