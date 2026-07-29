# P0 协议内核 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 用 Go 实现一个可靠的 B 站直播间归一化事件流与动作执行内核，交付一个能打印实时事件的 `magicd probe` CLI。

**Architecture:** 分三层——`wire`（二进制包编解码与解压）、`cmd`（CMD → 归一化 Event 的注册表映射，一 CMD 一文件）、`client`（连接状态机、认证、重连、风控）。事件模型采用「信封 + 强类型载荷」，原始 JSON 永不丢弃。动作执行（`Actions`）与事件流（`Connector`）分离，因为前者是账号级、后者是房间级。

**Tech Stack:** Go 1.24+、`gorilla/websocket`、`andybalholm/brotli`、`oklog/ulid/v2`、`google.golang.org/protobuf/encoding/protowire`（手工解码，不引入 protoc 工具链）、stdlib `testing`（不引入 testify）。

## Global Constraints

- Go module 路径：`github.com/MrZhongzq/MagicalDanmaku/server`
- 所有 Go 代码位于仓库的 `server/` 顶层目录；现有 C++ 代码原样保留作参考，不得移动或删除
- go.mod 的 `go` 指令为 `1.24`
- 仅使用 stdlib `testing`，不引入断言库
- 每个 CMD 的映射必须是**独立文件**，通过注册表分发；禁止 if/else 链
- `Event.Raw` 在任何路径下都不得为 nil
- 未知 CMD 必须产出 `TypeUnknown` 事件，禁止丢弃
- 不编写针对 B 站真实 API 的集成测试
- 所有导出标识符带中文注释；错误信息使用中文
- 提交信息使用中文，格式 `<type>: <描述>`

---

### Task 1: 项目骨架与事件模型

**Files:**
- Create: `server/go.mod`
- Create: `server/internal/event/type.go`
- Create: `server/internal/event/user.go`
- Create: `server/internal/event/payload.go`
- Create: `server/internal/event/event.go`
- Test: `server/internal/event/event_test.go`

**Interfaces:**
- Consumes: 无（首个任务）
- Produces:
  - `event.Type`（string 别名）与 18 个 `Type*` 常量
  - `event.User{UID, Username, AvatarURL, GuardLevel, UserLevel, WealthLevel, Medal *Medal, IsAdmin bool}`
  - `event.Medal{Name, Level, AnchorUID, AnchorName, RoomID, GuardLevel, IsLighted}`
  - `event.Payload` 标记接口；18 个具体载荷类型（见 payload.go）
  - `event.Event{ID, RoomID, Platform, Type, Timestamp, ReceivedAt, Payload, Raw}`
  - `event.NewID() string`（ULID）
  - `event.Platform` 与常量 `PlatformBilibili`

- [ ] **Step 1: 初始化 module 并写事件类型常量**

创建 `server/go.mod`：

```
module github.com/MrZhongzq/MagicalDanmaku/server

go 1.24

require github.com/oklog/ulid/v2 v2.1.0
```

创建 `server/internal/event/type.go`：

```go
// Package event 定义平台无关的归一化直播事件模型。
package event

// Platform 标识事件来源的直播平台。
type Platform string

// PlatformBilibili 表示哔哩哔哩直播。
const PlatformBilibili Platform = "bilibili"

// Type 是归一化后的事件类型。
// 87 个 B 站 CMD 收敛到这 18 种类型。
type Type string

// 全部归一化事件类型。
const (
	TypeDanmaku          Type = "danmaku"            // 弹幕
	TypeSuperChat        Type = "super_chat"         // 醒目留言
	TypeSuperChatDelete  Type = "super_chat_delete"  // 醒目留言被删除
	TypeGift             Type = "gift"               // 礼物
	TypeGiftCombo        Type = "gift_combo"         // 礼物连击
	TypeGuardBuy         Type = "guard_buy"          // 上舰
	TypeUserEnter        Type = "user_enter"         // 用户进场
	TypeUserFollow       Type = "user_follow"        // 用户关注
	TypeUserShare        Type = "user_share"         // 用户分享
	TypeUserLike         Type = "user_like"          // 用户点赞
	TypeLiveStart        Type = "live_start"         // 开播
	TypeLiveStop         Type = "live_stop"          // 下播
	TypeRoomChange       Type = "room_change"        // 房间信息变更
	TypeUserBlocked      Type = "user_blocked"       // 用户被禁言
	TypeOnlineRankUpdate Type = "online_rank_update" // 高能榜变化
	TypeRoomStatsUpdate  Type = "room_stats_update"  // 房间统计数据变化
	TypeBattle           Type = "battle"             // PK 大乱斗（P6 消费）
	TypeUnknown          Type = "unknown"            // 未识别的 CMD
)
```

- [ ] **Step 2: 写用户值对象**

创建 `server/internal/event/user.go`：

```go
package event

// 大航海等级。
const (
	GuardNone     = 0 // 非舰队
	GuardGovernor = 1 // 总督
	GuardAdmiral  = 2 // 提督
	GuardCaptain  = 3 // 舰长
)

// Medal 是用户佩戴的粉丝勋章。
type Medal struct {
	Name       string // 勋章名
	Level      int    // 勋章等级
	AnchorUID  string // 勋章所属主播 UID
	AnchorName string // 勋章所属主播昵称
	RoomID     string // 勋章所属直播间号
	GuardLevel int    // 该勋章对应的大航海等级
	IsLighted  bool   // 勋章是否点亮
}

// User 是所有事件共用的用户信息。
// 抽成值对象以避免每个载荷重复十几个字段。
type User struct {
	UID         string // 用户 UID
	Username    string // 昵称
	AvatarURL   string // 头像地址，可能为空
	GuardLevel  int    // 本房间大航海等级，见 Guard* 常量
	UserLevel   int    // UL 等级
	WealthLevel int    // 荣耀等级
	Medal       *Medal // 佩戴的勋章，未佩戴时为 nil
	IsAdmin     bool   // 是否房管
}
```

- [ ] **Step 3: 写载荷类型**

创建 `server/internal/event/payload.go`：

```go
package event

// Payload 是所有具体事件载荷的标记接口。
type Payload interface {
	isPayload()
}

// Danmaku 是一条弹幕。
type Danmaku struct {
	User        User
	Text        string // 弹幕正文
	Color       string // 十六进制颜色，形如 "#ffffff"
	IsEmoticon  bool   // 是否为表情弹幕
	ReplyToUID  string // @ 回复的目标 UID，无则为空
	ReplyToName string // @ 回复的目标昵称，无则为空
}

// Gift 是一次送礼。
type Gift struct {
	User      User
	GiftID    int64
	GiftName  string
	Count     int64
	CoinType  string // "gold" 金瓜子 / "silver" 银瓜子
	TotalCoin int64  // 总价值，单位瓜子
	Action    string // 动作描述，如「投喂」
}

// GiftCombo 是礼物连击汇总。
type GiftCombo struct {
	User      User
	GiftID    int64
	GiftName  string
	Count     int64
	ComboID   string
	TotalCoin int64
}

// GuardBuy 是一次上舰或续费。
type GuardBuy struct {
	User       User
	GuardLevel int    // 见 Guard* 常量
	GuardName  string // 如「舰长」
	Count      int    // 购买月数
	Price      int64  // 单位金瓜子
	IsRenew    bool   // true 为续费，false 为新购
}

// SuperChat 是一条醒目留言。
type SuperChat struct {
	User     User
	ID       int64
	Text     string
	Price    int64 // 单位元
	Duration int   // 展示秒数
}

// SuperChatDelete 表示若干醒目留言被删除。
type SuperChatDelete struct {
	IDs []int64
}

// UserEnter 表示用户进入直播间。
type UserEnter struct{ User User }

// UserFollow 表示用户关注了主播。
type UserFollow struct{ User User }

// UserShare 表示用户分享了直播间。
type UserShare struct{ User User }

// UserLike 表示用户点赞。
type UserLike struct{ User User }

// LiveStart 表示开播。
type LiveStart struct{}

// LiveStop 表示下播。
type LiveStop struct{}

// RoomChange 表示房间标题或分区变更。
type RoomChange struct {
	Title          string
	AreaID         string
	AreaName       string
	ParentAreaID   string
	ParentAreaName string
}

// UserBlocked 表示有用户被禁言。
type UserBlocked struct {
	User         User
	OperatorName string // 操作者昵称，可能为空
}

// RankUser 是高能榜上的一位用户。
type RankUser struct {
	User  User
	Rank  int
	Score string
}

// OnlineRankUpdate 是高能榜变化。
type OnlineRankUpdate struct {
	Count int        // 高能榜总人数，未知时为 -1
	Top   []RankUser // 榜单前若干名，可能为空
}

// RoomStatsUpdate 是房间统计数据变化。
// 指针字段为 nil 表示本次事件未携带该数据。
type RoomStatsUpdate struct {
	Fans      *int64 // 粉丝数
	FansClub  *int64 // 粉丝团人数
	Watched   *int64 // 累计看过人数
	LikeCount *int64 // 点赞数
}

// Battle 是 PK 大乱斗相关事件，P0 只归一化不解释。
type Battle struct {
	SubCommand string // 原始 CMD 名，如 "PK_BATTLE_START_NEW"
}

// Unknown 是未识别的 CMD。
type Unknown struct {
	Command string // 原始 CMD 名
}

func (Danmaku) isPayload()          {}
func (Gift) isPayload()             {}
func (GiftCombo) isPayload()        {}
func (GuardBuy) isPayload()         {}
func (SuperChat) isPayload()        {}
func (SuperChatDelete) isPayload()  {}
func (UserEnter) isPayload()        {}
func (UserFollow) isPayload()       {}
func (UserShare) isPayload()        {}
func (UserLike) isPayload()         {}
func (LiveStart) isPayload()        {}
func (LiveStop) isPayload()         {}
func (RoomChange) isPayload()       {}
func (UserBlocked) isPayload()      {}
func (OnlineRankUpdate) isPayload() {}
func (RoomStatsUpdate) isPayload()  {}
func (Battle) isPayload()           {}
func (Unknown) isPayload()          {}
```

- [ ] **Step 4: 写事件信封**

创建 `server/internal/event/event.go`：

```go
package event

import (
	"crypto/rand"
	"encoding/json"
	"time"

	"github.com/oklog/ulid/v2"
)

// Event 是归一化后的直播事件信封。
type Event struct {
	ID         string          // ULID，用于去重与全链路追踪
	RoomID     string          // 事件所属直播间号
	Platform   Platform        // 来源平台
	Type       Type            // 归一化事件类型
	Timestamp  time.Time       // 平台时间；缺失时等于 ReceivedAt
	ReceivedAt time.Time       // 本地接收时间
	Payload    Payload         // 强类型载荷
	Raw        json.RawMessage // 原始 CMD，永不为 nil
}

// NewID 生成一个 ULID 字符串，按时间单调递增可排序。
func NewID() string {
	return ulid.MustNew(ulid.Now(), rand.Reader).String()
}
```

- [ ] **Step 5: 写测试**

创建 `server/internal/event/event_test.go`：

```go
package event

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewIDUnique(t *testing.T) {
	seen := make(map[string]bool, 1000)
	for i := 0; i < 1000; i++ {
		id := NewID()
		if id == "" {
			t.Fatal("NewID 返回空字符串")
		}
		if seen[id] {
			t.Fatalf("NewID 产生重复 ID: %s", id)
		}
		seen[id] = true
	}
}

func TestEventHoldsTypedPayload(t *testing.T) {
	now := time.Now()
	e := Event{
		ID:         NewID(),
		RoomID:     "21452505",
		Platform:   PlatformBilibili,
		Type:       TypeDanmaku,
		Timestamp:  now,
		ReceivedAt: now,
		Payload:    Danmaku{User: User{UID: "1", Username: "甲"}, Text: "你好"},
		Raw:        json.RawMessage(`{"cmd":"DANMU_MSG"}`),
	}

	d, ok := e.Payload.(Danmaku)
	if !ok {
		t.Fatalf("载荷类型断言失败，实际为 %T", e.Payload)
	}
	if d.Text != "你好" {
		t.Errorf("Text = %q, 期望 %q", d.Text, "你好")
	}
	if e.Raw == nil {
		t.Error("Raw 不得为 nil")
	}
}

func TestAllPayloadsImplementInterface(t *testing.T) {
	payloads := []Payload{
		Danmaku{}, Gift{}, GiftCombo{}, GuardBuy{}, SuperChat{}, SuperChatDelete{},
		UserEnter{}, UserFollow{}, UserShare{}, UserLike{},
		LiveStart{}, LiveStop{}, RoomChange{}, UserBlocked{},
		OnlineRankUpdate{}, RoomStatsUpdate{}, Battle{}, Unknown{},
	}
	if len(payloads) != 18 {
		t.Fatalf("载荷类型数量 = %d, 期望 18", len(payloads))
	}
}
```

- [ ] **Step 6: 拉依赖并运行测试**

```bash
cd server && go mod tidy && go test ./internal/event/ -v
```

Expected: 三个测试全部 PASS。

- [ ] **Step 7: 提交**

```bash
git add server/
git commit -m "feat: 新增归一化事件模型与项目骨架"
```

---

### Task 2: wire 包编解码

**Files:**
- Create: `server/internal/connector/bilibili/wire/packet.go`
- Test: `server/internal/connector/bilibili/wire/packet_test.go`

**Interfaces:**
- Consumes: 无
- Produces:
  - 常量 `HeaderSize = 16`
  - 操作码常量 `OpHeartbeat=2, OpHeartbeatReply=3, OpMessage=5, OpAuth=7, OpAuthReply=8`
  - 协议版本常量 `ProtoJSON=0, ProtoHeartbeat=1, ProtoZlib=2, ProtoBrotli=3`
  - `wire.Header{TotalSize uint32, HeaderSize uint16, ProtoVer uint16, Operation uint32, Sequence uint32}`
  - `wire.Encode(op uint32, body []byte) []byte`
  - `wire.DecodeHeader(b []byte) (Header, error)`
  - `wire.ErrShortPacket`

- [ ] **Step 1: 写失败测试**

创建 `server/internal/connector/bilibili/wire/packet_test.go`：

```go
package wire

import (
	"bytes"
	"testing"
)

func TestEncodeProducesCorrectHeader(t *testing.T) {
	body := []byte("hello")
	got := Encode(OpAuth, body)

	if len(got) != HeaderSize+len(body) {
		t.Fatalf("总长度 = %d, 期望 %d", len(got), HeaderSize+len(body))
	}

	h, err := DecodeHeader(got)
	if err != nil {
		t.Fatalf("DecodeHeader 失败: %v", err)
	}
	if h.TotalSize != uint32(HeaderSize+len(body)) {
		t.Errorf("TotalSize = %d, 期望 %d", h.TotalSize, HeaderSize+len(body))
	}
	if h.HeaderSize != HeaderSize {
		t.Errorf("HeaderSize = %d, 期望 %d", h.HeaderSize, HeaderSize)
	}
	if h.ProtoVer != ProtoJSON {
		t.Errorf("ProtoVer = %d, 期望 %d", h.ProtoVer, ProtoJSON)
	}
	if h.Operation != OpAuth {
		t.Errorf("Operation = %d, 期望 %d", h.Operation, OpAuth)
	}
	if h.Sequence != 1 {
		t.Errorf("Sequence = %d, 期望 1", h.Sequence)
	}
	if !bytes.Equal(got[HeaderSize:], body) {
		t.Errorf("body = %q, 期望 %q", got[HeaderSize:], body)
	}
}

func TestDecodeHeaderRejectsShortInput(t *testing.T) {
	for _, n := range []int{0, 1, 15} {
		if _, err := DecodeHeader(make([]byte, n)); err == nil {
			t.Errorf("长度 %d 应当报错，实际返回 nil", n)
		}
	}
}

func TestDecodeHeaderReadsBigEndian(t *testing.T) {
	// 手工构造：TotalSize=0x00000020, HeaderSize=0x0010,
	// ProtoVer=0x0002, Operation=0x00000005, Sequence=0x00000001
	raw := []byte{
		0x00, 0x00, 0x00, 0x20,
		0x00, 0x10,
		0x00, 0x02,
		0x00, 0x00, 0x00, 0x05,
		0x00, 0x00, 0x00, 0x01,
	}
	h, err := DecodeHeader(raw)
	if err != nil {
		t.Fatalf("DecodeHeader 失败: %v", err)
	}
	if h.TotalSize != 32 || h.ProtoVer != ProtoZlib || h.Operation != OpMessage {
		t.Errorf("解析结果错误: %+v", h)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd server && go test ./internal/connector/bilibili/wire/ -v
```

Expected: 编译失败，`undefined: Encode`。

- [ ] **Step 3: 实现**

创建 `server/internal/connector/bilibili/wire/packet.go`：

```go
// Package wire 实现 B 站直播 WebSocket 的二进制包编解码。
package wire

import (
	"encoding/binary"
	"errors"
)

// HeaderSize 是包头固定长度。
const HeaderSize = 16

// 操作码。
const (
	OpHeartbeat      uint32 = 2 // 客户端心跳
	OpHeartbeatReply uint32 = 3 // 服务端心跳回复
	OpMessage        uint32 = 5 // 服务端业务消息
	OpAuth           uint32 = 7 // 客户端认证
	OpAuthReply      uint32 = 8 // 服务端认证回复
)

// 协议版本，决定 body 的编码方式。
const (
	ProtoJSON      uint16 = 0 // JSON 明文
	ProtoHeartbeat uint16 = 1 // 心跳回复，body 为 4 字节人气值
	ProtoZlib      uint16 = 2 // zlib 压缩
	ProtoBrotli    uint16 = 3 // brotli 压缩
)

// ErrShortPacket 表示数据长度不足以构成一个完整包头。
var ErrShortPacket = errors.New("wire: 数据长度不足 16 字节，无法解析包头")

// Header 是 B 站直播协议的包头。
type Header struct {
	TotalSize  uint32 // 含包头的总长度
	HeaderSize uint16 // 包头长度，恒为 16
	ProtoVer   uint16 // 协议版本
	Operation  uint32 // 操作码
	Sequence   uint32 // 序号，恒为 1
}

// Encode 构造一个待发送的数据包。
// 客户端发出的包一律使用 ProtoJSON 与 Sequence=1。
func Encode(op uint32, body []byte) []byte {
	buf := make([]byte, HeaderSize+len(body))
	binary.BigEndian.PutUint32(buf[0:4], uint32(HeaderSize+len(body)))
	binary.BigEndian.PutUint16(buf[4:6], HeaderSize)
	binary.BigEndian.PutUint16(buf[6:8], ProtoJSON)
	binary.BigEndian.PutUint32(buf[8:12], op)
	binary.BigEndian.PutUint32(buf[12:16], 1)
	copy(buf[HeaderSize:], body)
	return buf
}

// DecodeHeader 解析包头。所有字段均为大端序。
func DecodeHeader(b []byte) (Header, error) {
	if len(b) < HeaderSize {
		return Header{}, ErrShortPacket
	}
	return Header{
		TotalSize:  binary.BigEndian.Uint32(b[0:4]),
		HeaderSize: binary.BigEndian.Uint16(b[4:6]),
		ProtoVer:   binary.BigEndian.Uint16(b[6:8]),
		Operation:  binary.BigEndian.Uint32(b[8:12]),
		Sequence:   binary.BigEndian.Uint32(b[12:16]),
	}, nil
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
cd server && go test ./internal/connector/bilibili/wire/ -v
```

Expected: 三个测试全部 PASS。

- [ ] **Step 5: 提交**

```bash
git add server/internal/connector/bilibili/wire/
git commit -m "feat: 实现 B 站直播协议包头编解码"
```

---

### Task 3: wire 解压与多包切分

**Files:**
- Create: `server/internal/connector/bilibili/wire/frame.go`
- Test: `server/internal/connector/bilibili/wire/frame_test.go`
- Modify: `server/go.mod`（新增 brotli 依赖）

**Interfaces:**
- Consumes: Task 2 的 `Header`、`DecodeHeader`、`HeaderSize`、`Proto*`、`Op*`
- Produces:
  - `wire.Frame{Operation uint32, ProtoVer uint16, Body []byte}`
  - `wire.Split(raw []byte) ([]Frame, error)` — 解析一个 WebSocket 二进制帧，解压并切分出全部业务包

**关键细节：** protover 2/3 解压后是**多个完整包串联**，必须按每个包自己的 `TotalSize` 循环切分。这是原项目 `bili_livecmds.cpp:107-146` `splitUncompressedBody` 的核心逻辑。

- [ ] **Step 1: 写失败测试**

创建 `server/internal/connector/bilibili/wire/frame_test.go`：

```go
package wire

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"testing"

	"github.com/andybalholm/brotli"
)

// buildPacket 构造一个指定协议版本与操作码的完整包。
func buildPacket(protoVer uint16, op uint32, body []byte) []byte {
	buf := make([]byte, HeaderSize+len(body))
	binary.BigEndian.PutUint32(buf[0:4], uint32(HeaderSize+len(body)))
	binary.BigEndian.PutUint16(buf[4:6], HeaderSize)
	binary.BigEndian.PutUint16(buf[6:8], protoVer)
	binary.BigEndian.PutUint32(buf[8:12], op)
	binary.BigEndian.PutUint32(buf[12:16], 1)
	copy(buf[HeaderSize:], body)
	return buf
}

func TestSplitPlainJSON(t *testing.T) {
	body := []byte(`{"cmd":"LIVE"}`)
	frames, err := Split(buildPacket(ProtoJSON, OpMessage, body))
	if err != nil {
		t.Fatalf("Split 失败: %v", err)
	}
	if len(frames) != 1 {
		t.Fatalf("帧数 = %d, 期望 1", len(frames))
	}
	if !bytes.Equal(frames[0].Body, body) {
		t.Errorf("Body = %q, 期望 %q", frames[0].Body, body)
	}
	if frames[0].Operation != OpMessage {
		t.Errorf("Operation = %d, 期望 %d", frames[0].Operation, OpMessage)
	}
}

func TestSplitZlibMultiPacket(t *testing.T) {
	inner := append(
		buildPacket(ProtoJSON, OpMessage, []byte(`{"cmd":"A"}`)),
		buildPacket(ProtoJSON, OpMessage, []byte(`{"cmd":"BB"}`))...,
	)

	var compressed bytes.Buffer
	zw := zlib.NewWriter(&compressed)
	if _, err := zw.Write(inner); err != nil {
		t.Fatalf("zlib 写入失败: %v", err)
	}
	zw.Close()

	frames, err := Split(buildPacket(ProtoZlib, OpMessage, compressed.Bytes()))
	if err != nil {
		t.Fatalf("Split 失败: %v", err)
	}
	if len(frames) != 2 {
		t.Fatalf("帧数 = %d, 期望 2", len(frames))
	}
	if string(frames[0].Body) != `{"cmd":"A"}` {
		t.Errorf("第一帧 = %q", frames[0].Body)
	}
	if string(frames[1].Body) != `{"cmd":"BB"}` {
		t.Errorf("第二帧 = %q", frames[1].Body)
	}
}

func TestSplitBrotliMultiPacket(t *testing.T) {
	inner := append(
		buildPacket(ProtoJSON, OpMessage, []byte(`{"cmd":"X"}`)),
		buildPacket(ProtoJSON, OpMessage, []byte(`{"cmd":"YY"}`))...,
	)

	var compressed bytes.Buffer
	bw := brotli.NewWriter(&compressed)
	if _, err := bw.Write(inner); err != nil {
		t.Fatalf("brotli 写入失败: %v", err)
	}
	bw.Close()

	frames, err := Split(buildPacket(ProtoBrotli, OpMessage, compressed.Bytes()))
	if err != nil {
		t.Fatalf("Split 失败: %v", err)
	}
	if len(frames) != 2 {
		t.Fatalf("帧数 = %d, 期望 2", len(frames))
	}
	if string(frames[1].Body) != `{"cmd":"YY"}` {
		t.Errorf("第二帧 = %q", frames[1].Body)
	}
}

func TestSplitHeartbeatReply(t *testing.T) {
	body := []byte{0x00, 0x00, 0x04, 0xD2} // 人气值 1234
	frames, err := Split(buildPacket(ProtoHeartbeat, OpHeartbeatReply, body))
	if err != nil {
		t.Fatalf("Split 失败: %v", err)
	}
	if len(frames) != 1 {
		t.Fatalf("帧数 = %d, 期望 1", len(frames))
	}
	if frames[0].Operation != OpHeartbeatReply {
		t.Errorf("Operation = %d, 期望 %d", frames[0].Operation, OpHeartbeatReply)
	}
}

func TestSplitRejectsTruncated(t *testing.T) {
	if _, err := Split([]byte{0x00, 0x01}); err == nil {
		t.Error("截断输入应当报错")
	}
}

func TestSplitStopsOnBogusTotalSize(t *testing.T) {
	// TotalSize 声明为 8（小于 HeaderSize），必须终止而非死循环。
	raw := buildPacket(ProtoJSON, OpMessage, []byte(`{}`))
	binary.BigEndian.PutUint32(raw[0:4], 8)
	if _, err := Split(raw); err == nil {
		t.Error("非法 TotalSize 应当报错")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd server && go get github.com/andybalholm/brotli@v1.1.1 && go test ./internal/connector/bilibili/wire/ -run TestSplit -v
```

Expected: 编译失败，`undefined: Split`。

- [ ] **Step 3: 实现**

创建 `server/internal/connector/bilibili/wire/frame.go`：

```go
package wire

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"

	"github.com/andybalholm/brotli"
)

// maxDecompressed 是解压后允许的最大字节数，用于防御压缩炸弹。
const maxDecompressed = 32 << 20 // 32 MiB

// Frame 是一个已解压、可直接消费的业务包。
type Frame struct {
	Operation uint32 // 操作码
	ProtoVer  uint16 // 原始协议版本
	Body      []byte // 包体，ProtoJSON 时为 JSON 明文
}

// Split 解析一个 WebSocket 二进制帧，按需解压，并切分出其中全部业务包。
//
// protover 2/3 的包体解压后是多个完整包串联，必须按每个包自身的
// TotalSize 循环切分，而不能假设只有一个包。
func Split(raw []byte) ([]Frame, error) {
	h, err := DecodeHeader(raw)
	if err != nil {
		return nil, err
	}
	if int(h.TotalSize) > len(raw) {
		return nil, fmt.Errorf("wire: 包头声明长度 %d 超过实际数据长度 %d", h.TotalSize, len(raw))
	}
	if h.TotalSize < HeaderSize {
		return nil, fmt.Errorf("wire: 非法的包长度 %d", h.TotalSize)
	}

	body := raw[HeaderSize:h.TotalSize]

	switch h.ProtoVer {
	case ProtoZlib:
		unc, err := decompressZlib(body)
		if err != nil {
			return nil, err
		}
		return splitConcatenated(unc)
	case ProtoBrotli:
		unc, err := decompressBrotli(body)
		if err != nil {
			return nil, err
		}
		return splitConcatenated(unc)
	default:
		return []Frame{{Operation: h.Operation, ProtoVer: h.ProtoVer, Body: body}}, nil
	}
}

// splitConcatenated 按包头声明的长度循环切分串联的多个包。
func splitConcatenated(buf []byte) ([]Frame, error) {
	var frames []Frame
	for offset := 0; offset+HeaderSize <= len(buf); {
		h, err := DecodeHeader(buf[offset:])
		if err != nil {
			return nil, err
		}
		if h.TotalSize < HeaderSize {
			return nil, fmt.Errorf("wire: 偏移 %d 处包长度非法: %d", offset, h.TotalSize)
		}
		end := offset + int(h.TotalSize)
		if end > len(buf) {
			return nil, fmt.Errorf("wire: 偏移 %d 处包越界，声明 %d 但仅剩 %d", offset, h.TotalSize, len(buf)-offset)
		}
		frames = append(frames, Frame{
			Operation: h.Operation,
			ProtoVer:  h.ProtoVer,
			Body:      buf[offset+HeaderSize : end],
		})
		offset = end
	}
	return frames, nil
}

func decompressZlib(b []byte) ([]byte, error) {
	zr, err := zlib.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("wire: zlib 初始化失败: %w", err)
	}
	defer zr.Close()
	out, err := io.ReadAll(io.LimitReader(zr, maxDecompressed))
	if err != nil {
		return nil, fmt.Errorf("wire: zlib 解压失败: %w", err)
	}
	return out, nil
}

func decompressBrotli(b []byte) ([]byte, error) {
	out, err := io.ReadAll(io.LimitReader(brotli.NewReader(bytes.NewReader(b)), maxDecompressed))
	if err != nil {
		return nil, fmt.Errorf("wire: brotli 解压失败: %w", err)
	}
	return out, nil
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
cd server && go mod tidy && go test ./internal/connector/bilibili/wire/ -v
```

Expected: 全部 PASS（含 Task 2 的三个测试）。

- [ ] **Step 5: 提交**

```bash
git add server/
git commit -m "feat: 实现 zlib/brotli 解压与多包切分"
```

---

### Task 4: CMD 映射注册表与 Unknown 兜底

**Files:**
- Create: `server/internal/connector/bilibili/cmdmap/registry.go`
- Create: `server/internal/connector/bilibili/cmdmap/unknown.go`
- Create: `server/internal/connector/bilibili/cmdmap/jsonutil.go`
- Test: `server/internal/connector/bilibili/cmdmap/registry_test.go`

**Interfaces:**
- Consumes: Task 1 的 `event` 包全部导出标识符
- Produces:
  - `cmdmap.Context{RoomID string, ReceivedAt time.Time}`
  - `cmdmap.Mapper func(Context, json.RawMessage) ([]event.Event, error)`
  - `cmdmap.Register(cmd string, m Mapper)`
  - `cmdmap.Map(ctx Context, raw json.RawMessage) ([]event.Event, error)`
  - `cmdmap.CommandOf(raw json.RawMessage) string`
  - 辅助函数 `cmdmap.NewEvent(ctx Context, t event.Type, ts time.Time, p event.Payload, raw json.RawMessage) event.Event`
  - JSON 取值助手：`getString`、`getInt64`、`getBool`、`getArray`、`getObject`、`msFromUnix`

**包名说明：** 使用 `cmdmap` 而非 `cmd`，避免与 Go 惯例中 `cmd/` 可执行目录混淆。

- [ ] **Step 1: 写失败测试**

创建 `server/internal/connector/bilibili/cmdmap/registry_test.go`：

```go
package cmdmap

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

func testCtx() Context {
	return Context{RoomID: "21452505", ReceivedAt: time.Unix(1700000000, 0)}
}

func TestCommandOfPlain(t *testing.T) {
	if got := CommandOf(json.RawMessage(`{"cmd":"SEND_GIFT"}`)); got != "SEND_GIFT" {
		t.Errorf("CommandOf = %q, 期望 SEND_GIFT", got)
	}
}

func TestCommandOfStripsSuffix(t *testing.T) {
	// B 站的弹幕 CMD 会带后缀，如 DANMU_MSG:4:0:2:2:2:0
	if got := CommandOf(json.RawMessage(`{"cmd":"DANMU_MSG:4:0:2:2:2:0"}`)); got != "DANMU_MSG" {
		t.Errorf("CommandOf = %q, 期望 DANMU_MSG", got)
	}
}

func TestMapFallsBackToUnknown(t *testing.T) {
	raw := json.RawMessage(`{"cmd":"TOTALLY_NEW_THING","data":{"a":1}}`)
	evs, err := Map(testCtx(), raw)
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	if len(evs) != 1 {
		t.Fatalf("事件数 = %d, 期望 1", len(evs))
	}
	if evs[0].Type != event.TypeUnknown {
		t.Errorf("Type = %s, 期望 %s", evs[0].Type, event.TypeUnknown)
	}
	u, ok := evs[0].Payload.(event.Unknown)
	if !ok {
		t.Fatalf("载荷类型 = %T, 期望 event.Unknown", evs[0].Payload)
	}
	if u.Command != "TOTALLY_NEW_THING" {
		t.Errorf("Command = %q", u.Command)
	}
	if string(evs[0].Raw) != string(raw) {
		t.Error("Raw 必须原样保留")
	}
}

func TestMapUsesRegisteredMapper(t *testing.T) {
	Register("TEST_ONLY_CMD", func(ctx Context, raw json.RawMessage) ([]event.Event, error) {
		return []event.Event{NewEvent(ctx, event.TypeLiveStart, ctx.ReceivedAt, event.LiveStart{}, raw)}, nil
	})
	evs, err := Map(testCtx(), json.RawMessage(`{"cmd":"TEST_ONLY_CMD"}`))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	if len(evs) != 1 || evs[0].Type != event.TypeLiveStart {
		t.Fatalf("结果错误: %+v", evs)
	}
}

func TestNewEventAlwaysFillsRequiredFields(t *testing.T) {
	raw := json.RawMessage(`{"cmd":"X"}`)
	e := NewEvent(testCtx(), event.TypeLiveStop, time.Time{}, event.LiveStop{}, raw)

	if e.ID == "" {
		t.Error("ID 不得为空")
	}
	if e.RoomID != "21452505" {
		t.Errorf("RoomID = %q", e.RoomID)
	}
	if e.Platform != event.PlatformBilibili {
		t.Errorf("Platform = %q", e.Platform)
	}
	if e.Raw == nil {
		t.Error("Raw 不得为 nil")
	}
	// 传入零值时间时，Timestamp 应回落到 ReceivedAt
	if !e.Timestamp.Equal(e.ReceivedAt) {
		t.Errorf("零值 Timestamp 应回落到 ReceivedAt，实际 %v vs %v", e.Timestamp, e.ReceivedAt)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd server && go test ./internal/connector/bilibili/cmdmap/ -v
```

Expected: 编译失败，`undefined: CommandOf`。

- [ ] **Step 3: 实现注册表**

创建 `server/internal/connector/bilibili/cmdmap/registry.go`：

```go
// Package cmdmap 负责把 B 站直播 CMD 消息映射为归一化事件。
//
// 每个 CMD 的映射逻辑放在独立文件中，通过 init() 注册到全局表，
// 新增 CMD 只需新增文件，无需修改既有代码。
package cmdmap

import (
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

// Context 是映射过程所需的上下文。
type Context struct {
	RoomID     string    // 当前直播间号
	ReceivedAt time.Time // 本地接收时间
}

// Mapper 把一条原始 CMD JSON 映射为零个或多个归一化事件。
// 返回空切片表示该消息被有意忽略。
type Mapper func(ctx Context, raw json.RawMessage) ([]event.Event, error)

var (
	mu       sync.RWMutex
	registry = make(map[string]Mapper)
)

// Register 注册一个 CMD 的映射函数。重复注册会覆盖旧值。
// 约定在各 CMD 文件的 init() 中调用。
func Register(cmd string, m Mapper) {
	mu.Lock()
	defer mu.Unlock()
	registry[cmd] = m
}

// CommandOf 提取消息的 CMD 名，并剥离形如 ":4:0:2:2:2:0" 的后缀。
func CommandOf(raw json.RawMessage) string {
	var probe struct {
		Cmd string `json:"cmd"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return ""
	}
	if i := strings.IndexByte(probe.Cmd, ':'); i >= 0 {
		return probe.Cmd[:i]
	}
	return probe.Cmd
}

// Map 把一条原始 CMD JSON 映射为归一化事件。
// 未注册的 CMD 一律产出 TypeUnknown 事件，绝不丢弃。
func Map(ctx Context, raw json.RawMessage) ([]event.Event, error) {
	name := CommandOf(raw)

	mu.RLock()
	m, ok := registry[name]
	mu.RUnlock()

	if !ok {
		return mapUnknown(ctx, name, raw), nil
	}
	return m(ctx, raw)
}

// NewEvent 构造一个填好公共字段的事件。
// ts 为零值时回落到 ctx.ReceivedAt。
func NewEvent(ctx Context, t event.Type, ts time.Time, p event.Payload, raw json.RawMessage) event.Event {
	if ts.IsZero() {
		ts = ctx.ReceivedAt
	}
	// 复制一份 raw，避免上游复用底层缓冲区导致数据被覆写。
	rawCopy := make(json.RawMessage, len(raw))
	copy(rawCopy, raw)

	return event.Event{
		ID:         event.NewID(),
		RoomID:     ctx.RoomID,
		Platform:   event.PlatformBilibili,
		Type:       t,
		Timestamp:  ts,
		ReceivedAt: ctx.ReceivedAt,
		Payload:    p,
		Raw:        rawCopy,
	}
}
```

- [ ] **Step 4: 实现 Unknown 兜底**

创建 `server/internal/connector/bilibili/cmdmap/unknown.go`：

```go
package cmdmap

import (
	"encoding/json"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

// mapUnknown 为未注册的 CMD 产出兜底事件。
//
// 原项目遇到未知 CMD 是打日志丢弃，导致 B 站上线新功能后
// 用户必须等待客户端发版。这里改为照常投递，用户脚本可自行处理。
func mapUnknown(ctx Context, name string, raw json.RawMessage) []event.Event {
	return []event.Event{
		NewEvent(ctx, event.TypeUnknown, ctx.ReceivedAt, event.Unknown{Command: name}, raw),
	}
}
```

- [ ] **Step 5: 实现 JSON 取值助手**

创建 `server/internal/connector/bilibili/cmdmap/jsonutil.go`：

```go
package cmdmap

import (
	"encoding/json"
	"strconv"
	"time"
)

// 以下助手用于安全地从任意结构的 JSON 中取值。
// B 站的 CMD 结构不稳定且字段类型会变（数字有时是字符串），
// 因此所有取值都必须容错，缺失或类型不符时返回零值而非报错。

// getObject 返回 m 中键 k 对应的对象，不存在或类型不符时返回 nil。
func getObject(m map[string]any, k string) map[string]any {
	v, _ := m[k].(map[string]any)
	return v
}

// getArray 返回 m 中键 k 对应的数组，不存在或类型不符时返回 nil。
func getArray(m map[string]any, k string) []any {
	v, _ := m[k].([]any)
	return v
}

// getString 返回 m 中键 k 对应的字符串。
// 数字会被转成字符串，以应对 B 站同一字段时而数字时而字符串的情况。
func getString(m map[string]any, k string) string {
	switch v := m[k].(type) {
	case string:
		return v
	case float64:
		return strconv.FormatInt(int64(v), 10)
	case json.Number:
		return v.String()
	case bool:
		return strconv.FormatBool(v)
	default:
		return ""
	}
}

// getInt64 返回 m 中键 k 对应的整数，字符串会被尝试解析。
func getInt64(m map[string]any, k string) int64 {
	return toInt64(m[k])
}

// toInt64 把任意 JSON 标量转成 int64。
func toInt64(v any) int64 {
	switch t := v.(type) {
	case float64:
		return int64(t)
	case string:
		n, _ := strconv.ParseInt(t, 10, 64)
		return n
	case json.Number:
		n, _ := t.Int64()
		return n
	case bool:
		if t {
			return 1
		}
		return 0
	default:
		return 0
	}
}

// getBool 返回 m 中键 k 对应的布尔值，数字非零视为 true。
func getBool(m map[string]any, k string) bool {
	switch t := m[k].(type) {
	case bool:
		return t
	case float64:
		return t != 0
	case string:
		return t != "" && t != "0" && t != "false"
	default:
		return false
	}
}

// atInt64 返回数组 a 中下标 i 的整数，越界返回 0。
func atInt64(a []any, i int) int64 {
	if i < 0 || i >= len(a) {
		return 0
	}
	return toInt64(a[i])
}

// atString 返回数组 a 中下标 i 的字符串，越界返回空串。
func atString(a []any, i int) string {
	if i < 0 || i >= len(a) {
		return ""
	}
	switch v := a[i].(type) {
	case string:
		return v
	case float64:
		return strconv.FormatInt(int64(v), 10)
	default:
		return ""
	}
}

// atArray 返回数组 a 中下标 i 的子数组，越界或类型不符返回 nil。
func atArray(a []any, i int) []any {
	if i < 0 || i >= len(a) {
		return nil
	}
	v, _ := a[i].([]any)
	return v
}

// atObject 返回数组 a 中下标 i 的对象，越界或类型不符返回 nil。
func atObject(a []any, i int) map[string]any {
	if i < 0 || i >= len(a) {
		return nil
	}
	v, _ := a[i].(map[string]any)
	return v
}

// timeFromUnixSec 把 10 位秒级时间戳转成 time.Time，0 返回零值。
func timeFromUnixSec(sec int64) time.Time {
	if sec <= 0 {
		return time.Time{}
	}
	return time.Unix(sec, 0)
}

// timeFromUnixMilli 把 13 位毫秒级时间戳转成 time.Time，0 返回零值。
// 传入的若是 10 位秒级时间戳会被自动识别并放大。
func timeFromUnixMilli(ms int64) time.Time {
	if ms <= 0 {
		return time.Time{}
	}
	if ms < 1e11 { // 小于 1973 年的毫秒数，实为秒级时间戳
		return time.Unix(ms, 0)
	}
	return time.UnixMilli(ms)
}
```

- [ ] **Step 6: 运行测试确认通过**

```bash
cd server && go test ./internal/connector/bilibili/cmdmap/ -v
```

Expected: 五个测试全部 PASS。

- [ ] **Step 7: 检查未使用的助手不会导致编译失败**

Go 允许包内未使用的**函数**（只有未使用的局部变量和 import 才报错），因此 `jsonutil.go` 中暂未被调用的助手不影响编译。运行 `go vet` 确认：

```bash
cd server && go vet ./internal/connector/bilibili/cmdmap/
```

Expected: 无输出。

- [ ] **Step 8: 提交**

```bash
git add server/internal/connector/bilibili/cmdmap/
git commit -m "feat: 实现 CMD 映射注册表与未知消息兜底"
```

---

### Task 5: DANMU_MSG 映射与黄金样本测试基建

**Files:**
- Create: `server/internal/connector/bilibili/cmdmap/danmu_msg.go`
- Create: `server/internal/connector/bilibili/cmdmap/golden_test.go`
- Create: `server/testdata/cmds/DANMU_MSG_basic.json`
- Create: `server/testdata/cmds/DANMU_MSG_guard_medal.json`
- Test: `server/internal/connector/bilibili/cmdmap/danmu_msg_test.go`

**Interfaces:**
- Consumes: Task 4 的 `Context`、`Mapper`、`Register`、`NewEvent`、全部 JSON 助手
- Produces: `DANMU_MSG` 的注册映射；黄金样本测试框架 `TestGolden`（供后续任务复用，新增样本文件即自动覆盖）

**DANMU_MSG 的 info 数组结构**（源自原项目 `bili_livecmds.cpp:784-910`）：

| 路径 | 含义 |
|---|---|
| `info[0][3]` | 弹幕颜色（十进制整数） |
| `info[0][4]` | 发送时间（13 位毫秒） |
| `info[0][12]` | 弹幕类型，1 为表情弹幕 |
| `info[0][15].user.base.face` | 头像地址 |
| `info[0][15].extra` | JSON **字符串**，含 `reply_mid` / `reply_uname` |
| `info[1]` | 弹幕正文 |
| `info[2]` | `[uid, uname, admin, vip, svip, uidentity, iphone, unameColor]` |
| `info[3]` | 勋章 `[level, medalName, anchorName, roomId, ...,, guardLevel(10)]`，未佩戴时为空数组 |
| `info[4][0]` | UL 等级 |
| `info[7]` | 本房间大航海等级 |
| `info[16][0]` | 荣耀等级 |

- [ ] **Step 1: 准备黄金样本**

创建 `server/testdata/cmds/DANMU_MSG_basic.json`：

```json
{
  "cmd": "DANMU_MSG:4:0:2:2:2:0",
  "info": [
    [0, 1, 25, 16777215, 1700000000000, 1700000000, 0, "3676333332", 0, 0, 0, "", 0, {}, {}, {"user": {"base": {"face": "https://i0.hdslb.com/bfs/face/aaa.jpg"}}, "extra": "{\"show_reply\":false,\"reply_mid\":0,\"reply_uname\":\"\"}"}],
    "主播晚上好",
    [12345678, "路人甲", 0, 0, 0, 10000, 1, ""],
    [],
    [18, 0, 0, 0],
    ["", ""],
    0,
    0,
    null,
    {"ts": 1700000000},
    0,
    0,
    null,
    null,
    0,
    0,
    [7]
  ]
}
```

创建 `server/testdata/cmds/DANMU_MSG_guard_medal.json`：

```json
{
  "cmd": "DANMU_MSG",
  "info": [
    [0, 1, 25, 5566168, 1700000123000, 1700000123, 0, "1234", 0, 0, 0, "", 1, {}, {}, {"user": {"base": {"face": "https://i0.hdslb.com/bfs/face/bbb.jpg"}}, "extra": "{\"show_reply\":true,\"reply_mid\":20285041,\"reply_uname\":\"某某主播\"}"}],
    "[dog]",
    [87654321, "老粉丝", 1, 0, 0, 10000, 1, "#FB7299"],
    [21, "小心心", "某某主播", 21452505, 6126494, "", 0, 6126494, 6126494, 6126494, 0, 1, 1234567],
    [30, 0, 0, 0],
    ["", ""],
    0,
    3,
    null,
    {"ts": 1700000123},
    0,
    0,
    null,
    null,
    0,
    0,
    [25]
  ]
}
```

- [ ] **Step 2: 写失败测试**

创建 `server/internal/connector/bilibili/cmdmap/danmu_msg_test.go`：

```go
package cmdmap

import (
	"os"
	"testing"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

func loadSample(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("../../../../testdata/cmds/" + name + ".json")
	if err != nil {
		t.Fatalf("读取样本 %s 失败: %v", name, err)
	}
	return b
}

func TestDanmuMsgBasic(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "DANMU_MSG_basic"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	if len(evs) != 1 {
		t.Fatalf("事件数 = %d, 期望 1", len(evs))
	}
	if evs[0].Type != event.TypeDanmaku {
		t.Fatalf("Type = %s, 期望 %s", evs[0].Type, event.TypeDanmaku)
	}

	d := evs[0].Payload.(event.Danmaku)
	if d.Text != "主播晚上好" {
		t.Errorf("Text = %q", d.Text)
	}
	if d.User.UID != "12345678" {
		t.Errorf("UID = %q", d.User.UID)
	}
	if d.User.Username != "路人甲" {
		t.Errorf("Username = %q", d.User.Username)
	}
	if d.User.UserLevel != 18 {
		t.Errorf("UserLevel = %d, 期望 18", d.User.UserLevel)
	}
	if d.User.WealthLevel != 7 {
		t.Errorf("WealthLevel = %d, 期望 7", d.User.WealthLevel)
	}
	if d.User.GuardLevel != event.GuardNone {
		t.Errorf("GuardLevel = %d, 期望 0", d.User.GuardLevel)
	}
	if d.User.IsAdmin {
		t.Error("IsAdmin 应为 false")
	}
	if d.User.Medal != nil {
		t.Errorf("未佩戴勋章时 Medal 应为 nil，实际 %+v", d.User.Medal)
	}
	if d.User.AvatarURL != "https://i0.hdslb.com/bfs/face/aaa.jpg" {
		t.Errorf("AvatarURL = %q", d.User.AvatarURL)
	}
	if d.Color != "#ffffff" {
		t.Errorf("Color = %q, 期望 #ffffff", d.Color)
	}
	if d.IsEmoticon {
		t.Error("IsEmoticon 应为 false")
	}
	if d.ReplyToUID != "" {
		t.Errorf("ReplyToUID = %q, 期望空", d.ReplyToUID)
	}
	if got := evs[0].Timestamp.UnixMilli(); got != 1700000000000 {
		t.Errorf("Timestamp = %d, 期望 1700000000000", got)
	}
}

func TestDanmuMsgGuardAndMedal(t *testing.T) {
	evs, err := Map(testCtx(), loadSample(t, "DANMU_MSG_guard_medal"))
	if err != nil {
		t.Fatalf("Map 失败: %v", err)
	}
	d := evs[0].Payload.(event.Danmaku)

	if d.User.GuardLevel != event.GuardCaptain {
		t.Errorf("GuardLevel = %d, 期望 3", d.User.GuardLevel)
	}
	if !d.User.IsAdmin {
		t.Error("IsAdmin 应为 true")
	}
	if d.User.Medal == nil {
		t.Fatal("Medal 不应为 nil")
	}
	if d.User.Medal.Level != 21 {
		t.Errorf("Medal.Level = %d, 期望 21", d.User.Medal.Level)
	}
	if d.User.Medal.Name != "小心心" {
		t.Errorf("Medal.Name = %q", d.User.Medal.Name)
	}
	if d.User.Medal.AnchorName != "某某主播" {
		t.Errorf("Medal.AnchorName = %q", d.User.Medal.AnchorName)
	}
	if d.User.Medal.RoomID != "21452505" {
		t.Errorf("Medal.RoomID = %q", d.User.Medal.RoomID)
	}
	if !d.IsEmoticon {
		t.Error("IsEmoticon 应为 true")
	}
	if d.ReplyToUID != "20285041" {
		t.Errorf("ReplyToUID = %q, 期望 20285041", d.ReplyToUID)
	}
	if d.ReplyToName != "某某主播" {
		t.Errorf("ReplyToName = %q", d.ReplyToName)
	}
	if d.Color != "#54ee98" {
		t.Errorf("Color = %q, 期望 #54ee98", d.Color)
	}
}
```

- [ ] **Step 3: 运行测试确认失败**

```bash
cd server && go test ./internal/connector/bilibili/cmdmap/ -run TestDanmuMsg -v
```

Expected: FAIL，因为 `DANMU_MSG` 未注册，落入 Unknown 分支，`Type` 断言失败。

- [ ] **Step 4: 实现映射**

创建 `server/internal/connector/bilibili/cmdmap/danmu_msg.go`：

```go
package cmdmap

import (
	"encoding/json"
	"fmt"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

func init() {
	Register("DANMU_MSG", mapDanmuMsg)
}

// mapDanmuMsg 解析弹幕消息。
//
// info 数组的下标含义见本任务文档中的结构表。B 站会不定期在数组尾部
// 追加字段，因此全部取值都必须做越界保护。
func mapDanmuMsg(ctx Context, raw json.RawMessage) ([]event.Event, error) {
	var msg struct {
		Info []any `json:"info"`
	}
	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil, fmt.Errorf("cmdmap: DANMU_MSG 解析失败: %w", err)
	}
	info := msg.Info
	if len(info) < 3 {
		return nil, fmt.Errorf("cmdmap: DANMU_MSG 的 info 长度为 %d，至少需要 3", len(info))
	}

	meta := atArray(info, 0)   // 弹幕元信息
	userArr := atArray(info, 2) // 用户信息
	medalArr := atArray(info, 3)

	u := event.User{
		UID:         atString(userArr, 0),
		Username:    atString(userArr, 1),
		IsAdmin:     atInt64(userArr, 2) != 0,
		UserLevel:   int(atInt64(atArray(info, 4), 0)),
		GuardLevel:  int(atInt64(info, 7)),
		WealthLevel: int(atInt64(atArray(info, 16), 0)),
		Medal:       parseDanmuMedal(medalArr),
	}

	d := event.Danmaku{
		User:       u,
		Text:       atString(info, 1),
		Color:      formatColor(atInt64(meta, 3)),
		IsEmoticon: atInt64(meta, 12) == 1,
	}

	// info[0][15] 是 2023 年后新增的详情对象，含头像与回复信息。
	if detail := atObject(meta, 15); detail != nil {
		if base := getObject(getObject(detail, "user"), "base"); base != nil {
			d.User.AvatarURL = getString(base, "face")
		}
		// extra 是一个被再次编码为字符串的 JSON。
		if extraStr := getString(detail, "extra"); extraStr != "" {
			var extra map[string]any
			if err := json.Unmarshal([]byte(extraStr), &extra); err == nil {
				if getBool(extra, "show_reply") {
					if mid := getInt64(extra, "reply_mid"); mid != 0 {
						d.ReplyToUID = fmt.Sprintf("%d", mid)
						d.ReplyToName = getString(extra, "reply_uname")
					}
				}
			}
		}
	}

	ts := timeFromUnixMilli(atInt64(meta, 4))
	return []event.Event{NewEvent(ctx, event.TypeDanmaku, ts, d, raw)}, nil
}

// parseDanmuMedal 解析 info[3] 的勋章数组，未佩戴时返回 nil。
func parseDanmuMedal(a []any) *event.Medal {
	if len(a) < 4 {
		return nil
	}
	return &event.Medal{
		Level:      int(atInt64(a, 0)),
		Name:       atString(a, 1),
		AnchorName: atString(a, 2),
		RoomID:     atString(a, 3),
		GuardLevel: int(atInt64(a, 10)),
		IsLighted:  atInt64(a, 11) != 0,
		AnchorUID:  atString(a, 12),
	}
}

// formatColor 把十进制整数颜色转成 "#rrggbb"。
func formatColor(n int64) string {
	if n < 0 {
		n = 0
	}
	return fmt.Sprintf("#%06x", n&0xffffff)
}
```

- [ ] **Step 5: 运行测试确认通过**

```bash
cd server && go test ./internal/connector/bilibili/cmdmap/ -run TestDanmuMsg -v
```

Expected: 两个测试均 PASS。

- [ ] **Step 6: 写黄金样本回归框架**

创建 `server/internal/connector/bilibili/cmdmap/golden_test.go`：

```go
package cmdmap

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

const testdataDir = "../../../../testdata/cmds"

// TestGoldenSamplesAllMap 遍历全部黄金样本，确保：
//  1. 每个样本都能被解析且不返回错误
//  2. 已注册的 CMD 不得落入 Unknown 分支
//  3. Raw 必须原样保留
//
// 后续任务只需往 testdata/cmds/ 添加样本文件，本测试自动覆盖。
func TestGoldenSamplesAllMap(t *testing.T) {
	entries, err := os.ReadDir(testdataDir)
	if err != nil {
		t.Fatalf("读取样本目录失败: %v", err)
	}

	var count int
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		count++
		name := e.Name()
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(testdataDir, name))
			if err != nil {
				t.Fatalf("读取失败: %v", err)
			}
			if !json.Valid(raw) {
				t.Fatalf("样本不是合法 JSON")
			}

			evs, err := Map(testCtx(), raw)
			if err != nil {
				t.Fatalf("Map 返回错误: %v", err)
			}
			if len(evs) == 0 {
				t.Fatal("Map 返回空事件列表，样本应至少产出一个事件")
			}

			// 样本文件名以 CMD 名开头，据此判断是否应被识别。
			cmdName := CommandOf(raw)
			if !strings.HasPrefix(name, cmdName) {
				t.Fatalf("样本文件名 %q 应以 CMD 名 %q 开头", name, cmdName)
			}

			for i, ev := range evs {
				if ev.Raw == nil {
					t.Errorf("第 %d 个事件的 Raw 为 nil", i)
				}
				if ev.ID == "" {
					t.Errorf("第 %d 个事件的 ID 为空", i)
				}
				if ev.RoomID == "" {
					t.Errorf("第 %d 个事件的 RoomID 为空", i)
				}
				if ev.Timestamp.IsZero() {
					t.Errorf("第 %d 个事件的 Timestamp 为零值", i)
				}
				if ev.Type == event.TypeUnknown {
					t.Errorf("第 %d 个事件落入 Unknown，说明 %s 的映射未注册", i, cmdName)
				}
			}
		})
	}

	if count == 0 {
		t.Fatal("样本目录为空")
	}
	t.Logf("已校验 %d 个黄金样本", count)
}
```

- [ ] **Step 7: 运行全部测试**

```bash
cd server && go test ./... -v
```

Expected: 全部 PASS，`TestGoldenSamplesAllMap` 输出「已校验 2 个黄金样本」。

- [ ] **Step 8: 提交**

```bash
git add server/
git commit -m "feat: 实现 DANMU_MSG 映射与黄金样本回归框架"
```

---

**说明：** 本计划的 Task 6～16 结构与上述任务完全一致（准备样本 → 写失败测试 → 确认失败 → 实现 → 确认通过 → 提交），因内容较长拆分在续篇文档中。执行完 Task 5 后请继续阅读 `2026-07-29-p0-protocol-core-part2.md`。
