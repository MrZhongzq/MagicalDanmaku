# P0 协议内核 · 设计文档

- 日期：2026-07-29
- 状态：已评审（事件模型部分经用户确认）
- 所属项目：神奇弹幕重构（Go 重写版）
- 前置决策来源：本文件「附录 A：已锁定的全局决策」

---

## 0. 背景

原项目 [Bilibili-MagicalDanmaku](https://github.com/iwxyi/Bilibili-MagicalDanmaku) 是 Qt5/C++17 桌面应用，v6.0.0，已于 2025-12-25 归档停更。本次重构目标是：Go 重写、前后端分离、多平台分发、全功能免费。

原 C++ 代码在本次重构中的定位是**参考资料**，不是移植对象。其真正价值在于协议知识的沉淀——87 个 CMD 的字段含义、风控触发条件、包格式的历史遗留细节——而非代码本身。

P0 是整个重构的地基，也是唯一需要大量考古原 C++ 代码的子项目。协议知识主要集中在 `services/live_services/bilibili/bili_liveservice.cpp`（5441 行）与 `bili_livecmds.cpp`（3029 行）。

---

## 1. 范围与交付物

### 1.1 交付物

一个可独立运行的 Go 模块，加一个用于验证的 CLI：

```bash
$ magicd probe --room 21452505 --cookie-file ./cookie.txt
[19:23:01] DANMAKU   某某某(12345678) UL18 舰长: 主播晚上好
[19:23:04] GIFT      某某某 送出 小心心 x3 (免费)
[19:23:07] ENTER     某某某 进入直播间
[19:23:11] GUARD_BUY 某某某 购买 舰长 x1 (¥138)
[19:23:15] UNKNOWN   cmd=LIVE_MULTI_VIEW_CHANGE (raw 已保留)
```

### 1.2 边界

P0 只负责两件事：

1. 把一个 B 站直播间变成一条**可靠的、归一化的事件流**
2. 把**动作**（发弹幕、禁言、解禁）可靠地发出去

### 1.3 明确不做

以下功能属于后续子项目，P0 不得实现，以避免范围蔓延：

| 不做的事 | 归属 |
|---|---|
| 规则匹配、脚本执行 | P2 规则引擎 |
| 礼物连击合并、事件业务去重 | P2 规则引擎 |
| 冷却通道、发送策略 | P2 规则引擎（P0 只提供限流**机制**，不定策略） |
| 数据库、持久化 | P3 数据层 |
| HTTP API、WebUI | P4 |
| 统计聚合 | P5 |
| PK 大乱斗业务逻辑 | P6（P0 只负责把 `PK_BATTLE_*` 归一化为 `Battle` 事件，不消费） |

---

## 2. 归一化事件模型

这是 P0 的核心资产，也是整个项目最不能定错的部分。

### 2.1 要修的病灶

原项目的 `LiveDanmaku`（`services/entities/livedanmaku.h`，1208 行）是一个上帝结构体：弹幕、礼物、上舰、进场、PK 全部塞进同一个类，靠 `msgType` 字段区分，几十个字段对大多数事件类型都是无意义的空值。任何人读代码都无法判断某个字段在什么事件下有效。

### 2.2 事件信封

```go
package event

type Event struct {
    ID         string          // ULID，用于去重与全链路追踪
    RoomID     string
    Platform   Platform        // 目前恒为 Bilibili
    Type       Type
    Timestamp  time.Time       // 平台时间；缺失时回落到 ReceivedAt
    ReceivedAt time.Time
    Payload    Payload         // 强类型载荷，见 2.3
    Raw        json.RawMessage // 原始 CMD，永不丢弃
}

// Payload 是所有具体载荷类型的标记接口
type Payload interface {
    isPayload()
}
```

### 2.3 事件类型

87 个 CMD 归一化后收敛为 18 种事件类型：

| 事件类型 | 来源 CMD |
|---|---|
| `Danmaku` | `DANMU_MSG`、`DANMU_MSG:*` |
| `SuperChat` | `SUPER_CHAT_MESSAGE`、`SUPER_CHAT_MESSAGE_JPN` |
| `SuperChatDelete` | `SUPER_CHAT_MESSAGE_DELETE` |
| `Gift` | `SEND_GIFT` |
| `GiftCombo` | `COMBO_SEND` |
| `GuardBuy` | `GUARD_BUY`、`USER_TOAST_MSG` |
| `UserEnter` | `INTERACT_WORD`、`INTERACT_WORD_V2`(protobuf)、`ENTRY_EFFECT`、`WELCOME`、`WELCOME_GUARD` |
| `UserFollow` | `INTERACT_WORD` msg_type=2 |
| `UserShare` | `INTERACT_WORD` msg_type=3 |
| `UserLike` | `LIKE_INFO_V3_CLICK` |
| `LiveStart` | `LIVE` |
| `LiveStop` | `PREPARING` |
| `RoomChange` | `ROOM_CHANGE` |
| `UserBlocked` | `ROOM_BLOCK_MSG` |
| `OnlineRankUpdate` | `ONLINE_RANK_V2`、`ONLINE_RANK_TOP3`、`ONLINE_RANK_COUNT` |
| `RoomStatsUpdate` | `ROOM_REAL_TIME_MESSAGE_UPDATE`、`WATCHED_CHANGE`、`LIKE_INFO_V3_UPDATE` |
| `Battle` | 20+ 个 `PK_BATTLE_*`（P0 只归一化，P6 消费） |
| `Unknown` | 其余全部 |

### 2.4 三条硬规则

每条都对应原项目的一个真实教训。

**规则 1：`Raw` 永不丢弃。**
B 站 CMD 字段增删频繁，原码里到处是 v1/v2 分支和注释掉的旧字段。保留原始 JSON 让用户脚本能兜底新字段，也让线上排障成为可能。

**规则 2：`Unknown` 事件照常投递，不吞。**
原项目遇到未知 CMD 是 `qWarning` 打日志丢弃（`bili_livecmds.cpp:99`）。新版投递到事件流，用户脚本可自行处理 B 站的新功能，不必等上游发版。

**规则 3：归一化层不做业务判断。**
「是不是舰长」「是不是粉丝团」是 payload 上的字段，不是事件类型。原项目把 `WELCOME_GUARD` 单独当一类处理，导致「用户进场」这一件事散落在三条代码路径上。

### 2.5 载荷设计原则

- 每个 Payload 只包含**该事件确实有的字段**
- 平台特有的、暂时用不上的字段不进 Payload，从 `Raw` 取
- 用户信息统一抽成 `User` 值对象（uid / uname / 头像 / 粉丝牌 / 舰长等级 / UL 等级），避免每个 Payload 重复十几个字段

```go
type User struct {
    UID        string
    Username   string
    AvatarURL  string
    GuardLevel int      // 0=无 1=总督 2=提督 3=舰长
    UserLevel  int      // UL 等级
    Medal      *Medal   // 粉丝牌，可空
    IsAdmin    bool     // 房管
}
```

### 2.6 要修的两个具体 bug

**Bug 1：非线程安全的全局去重。**
`bili_livecmds.cpp:13-27` 用函数内 `static bool processing` + `static QByteArray lastMessage` 做去重。这是非线程安全的全局状态，且会误杀合法的重复弹幕（两个人发一样的字会被吞掉一条）。

新版做法：不在 P0 做业务去重。P0 只保证同一个 WebSocket 帧不被重复解析。真正的去重（判断是否同一条弹幕）依据 `Event.ID` 指纹，指纹计算含时间戳与用户 ID，去重逻辑上移到 P2。

**Bug 2：靠字符串裁剪构造 POST body。**
`bili_liveservice.cpp:4780-4820` 的 `sendRoomMsg` 对用户从浏览器粘贴的原始 POST body 做 `indexOf("msg=")` + `replace(posl, posr-posl, ...)` 的字符串手术，用来替换 msg / roomid / csrf。极其脆弱，且要求用户手工从 devtools 复制 body。

新版做法：结构化参数构造请求，Cookie 只解析不拼接，csrf 从 Cookie 的 `bili_jct` 字段解析得到。用户只需提供 Cookie，不需要提供 POST body。

---

## 3. Connector 抽象

### 3.1 接口定义

```go
package connector

// Connector 是平台接入的唯一抽象点。
// 一个 Connector 实例对应一个直播间的事件流。
type Connector interface {
    // Run 阻塞运行直到 ctx 取消；内部自行处理重连。
    Run(ctx context.Context) error
    // Events 返回归一化事件流（只读）。
    Events() <-chan event.Event
    // State 返回当前连接状态，用于上层展示。
    State() State
}

// Actions 是可执行动作集，与 Connector 分离。
type Actions interface {
    SendDanmaku(ctx context.Context, req SendDanmakuRequest) error
    BlockUser(ctx context.Context, req BlockRequest) error
    UnblockUser(ctx context.Context, roomID, uid string) error
}
```

### 3.2 为什么 Connector 和 Actions 要分开

事件流是**房间级**的：一个房间一条流，与身份无关（甚至可以匿名连接，只是功能受限）。

动作是**账号级**的：需要具体账号的 Cookie 与 csrf，且要支持多账号轮换发言（原项目的「副账号」功能）。

原项目把两者混在 `LiveServiceBase` 里，直接后果是子账号发弹幕的逻辑只能在 `sendRoomMsg` 内部做字符串手术替换 csrf（`bili_liveservice.cpp:4806-4830`）。分离后，多账号只是持有多个 `Actions` 实例。

### 3.3 平台预留

`Connector` 与 `Actions` 是纯接口，`internal/connector/bilibili` 是目前唯一实现。新增平台只需新增一个包，核心与上层不动。**不提前构造任何抽象基类**——原项目 `LiveServiceBase` 有约 100 个空实现的 virtual 方法，是过度抽象的反面教材。

---

## 4. 认证与会话

### 4.1 登录方式

**扫码登录**（主路径，headless 友好）：

1. `GET https://passport.bilibili.com/x/passport-login/web/qrcode/generate` → `qrcode_key` + `url`
2. 渲染二维码。**后端不引入 QR 图像库**：P0 阶段 CLI 在终端打印 ASCII 二维码；P4 阶段后端只返回 url，由前端 JS 渲染。
3. 轮询 `GET https://passport.bilibili.com/x/passport-login/web/qrcode/poll?qrcode_key=...`，从响应的 Set-Cookie 提取会话

**Cookie 手动导入**（备选路径）：用户从浏览器复制 Cookie 字符串。

### 4.2 会话必需字段

| 字段 | 用途 |
|---|---|
| `SESSDATA` | 身份凭证 |
| `bili_jct` | csrf token，所有写操作必需 |
| `DedeUserID` | 自己的 UID |
| `buvid3` | 设备指纹，缺失会触发风控 |

### 4.3 buvid 获取

优先从 Cookie 的 `buvid3=` 解析。缺失则请求 `GET https://api.bilibili.com/x/frontend/finger/spi`，取 `data.b_3`。

原项目发现：Cookie 缺 `buvid3` 时 `getDanmuInfo` 会返回 -352 风控码，补全 `buvid3` / `buvid4` / `b_nut` 后重试即可恢复（`bili_liveservice.cpp:369-441`）。这个补全逻辑必须保留。

### 4.4 wbi 签名

B 站部分接口（含 `getDanmuInfo`）要求 wbi 签名。算法（源自 `bili_liveservice.cpp:269-333`）：

1. `GET https://api.bilibili.com/x/web-interface/nav` → `data.wbi_img.img_url` 与 `sub_url`
2. 从两个 URL 提取文件名（不含扩展名），拼接得到 64 字符的 `wbi_img_sub`
3. 按固定的 64 元素置换表 `mixinKeyEncTab` 重排，取前 32 位 = `mixinKey`
4. 请求时：参数按 key 字典序排序，追加 `wts=<unix秒>`，对 `<sorted_params>+<mixinKey>` 取 MD5 小写十六进制 = `w_rid`

`mixinKey` 按天缓存，跨天刷新。置换表是常量，直接从原码 `mixinKeyEncTab` 抄录。

### 4.5 会话有效性

`GET https://passport.bilibili.com/x/passport-login/web/cookie/info` 可检测 Cookie 是否需要刷新。P0 只做检测与上报，自动刷新（refresh_token 流程）留待后续。

---

## 5. 连接生命周期与容错

### 5.1 状态机

```
Idle → ResolvingRoom → FetchingDanmuInfo → Connecting → Connected
                                                ↑           │
                                                └─ Reconnecting ←┘
任意状态 → RiskControlled（风控，长退避）
任意状态 → Closed（ctx 取消）
```

### 5.2 连接流程

1. `GET /room/v1/Room/get_info?room_id=` → 拿到真实长房间号与开播状态
2. `GET /xlive/web-room/v1/index/getDanmuInfo?<wbi签名>` → 拿到 `token` 与 `host_list`
3. 从 `host_list` 取一个 host，建立 WSS 连接
4. 发送认证包（`OP_AUTH`）：
   ```json
   {"uid":<自己UID>,"roomid":<房间号>,"protover":2,"platform":"web","type":2,"key":"<token>","buvid":"<buvid3>"}
   ```
5. 收到 `OP_AUTH_REPLY` 且 `code==0` → 进入 Connected
6. 启动心跳循环

### 5.3 包格式

```
offset  0-3   总包长      uint32 BE
offset  4-5   头长度      uint16 BE（恒 16）
offset  6-7   协议版本    uint16 BE
offset  8-11  操作码      uint32 BE
offset 12-15  sequence   uint32 BE（恒 1）
offset 16-    body
```

协议版本：`0`=JSON 明文，`1`=心跳回复（body 是 4 字节人气值），`2`=zlib 压缩，`3`=brotli 压缩

操作码：`2`=心跳，`3`=心跳回复，`5`=消息，`7`=认证，`8`=认证回复

**关键细节**：protover 2/3 解压后是**多个完整包串联**，必须按每个包自己的 `总包长` 字段循环切分（原项目 `splitUncompressedBody`，`bili_livecmds.cpp:107-146`）。

### 5.4 心跳

30 秒一次，操作码 `OP_HEARTBEAT`，body 固定为字面量字符串 `[object Object]`。这是 B 站前端的历史遗留 bug（JS 对象被隐式转字符串），服务端至今兼容，必须原样发送。

### 5.5 重连策略

- 初始间隔 3 秒，指数退避，上限 60 秒（沿用原项目 `INTERVAL_RECONNECT_WS` 常量）
- 加入随机抖动，避免多房间同时重连
- 每次重连轮换到 `host_list` 的下一个 host
- 上播/下播事件重置退避间隔

### 5.6 风控处理

收到 -352 时：

1. 若 Cookie 缺 `buvid3` / `buvid4` / `b_nut`，自动补全后重试一次
2. 仍失败 → 进入 `RiskControlled` 状态，采用长退避（起步 5 分钟），并向上层上报

**不得无限快速重试。** 原项目在风控下会持续重试，反而加剧风控。这是必须修正的行为。

---

## 6. 动作执行

### 6.1 支持的动作

| 动作 | 接口 |
|---|---|
| 发弹幕 | `POST https://api.live.bilibili.com/msg/send` |
| 禁言 | `POST /xlive/web-ucenter/v1/banned/AddSilentUser` |
| 解禁 | `POST /xlive/web-ucenter/v1/banned/DelSilentUser` |

发弹幕参数：`bubble=0`、`msg`、`color=16777215`、`mode=1`、`fontsize=25`、`rnd=<unix秒>`、`roomid`、`csrf`、`csrf_token`。

`reply_mid` 支持 @ 回复。P0 只支持显式 uid（`@12345678`）；按用户名反查 uid 依赖最近弹幕缓存，属于 P2 职责。

### 6.2 长弹幕切分

单条弹幕字数上限由账号 UL 等级与舰长状态决定（20 / 30 / 40）。超长文本按标点优先切分为多条依次发送。

### 6.3 限流

P0 **只提供机制，不定策略**：

```go
type RateLimiter interface {
    Wait(ctx context.Context) error
}
```

默认实现为最小间隔 1.5 秒（沿用原项目 `AUTO_MSG_CD`）。具体的冷却通道、优先级、去重策略全部留给 P2。

### 6.4 返回码处理

| 返回码 | 含义 | 处理 |
|---|---|---|
| `0` | 成功 | — |
| `10030` | 发送频率过快 | 退避后重试 |
| `-101` | 未登录 | 标记会话失效，**不重试** |
| `-111` | csrf 失效 | 标记会话失效，**不重试** |
| `1003` | 已被禁言 | **不重试** |
| 其他 | 未知 | 记录后不重试 |

---

## 7. 测试策略

### 7.1 黄金样本回归测试（最重要）

原项目已在 SQLite 中存储原始 CMD（`SqlService::insertCmd`）。采集一批真实 CMD JSON 存入 `testdata/cmds/`，每个样本对应一个期望的 Event 快照。

这是保证「归一化不丢信息、不误解字段」的唯一可靠手段，也是重构相对原项目**唯一的正确性基准**。

样本必须覆盖：每种事件类型至少 3 个样本、v1/v2 双版本的 CMD、含粉丝牌与不含粉丝牌、舰长与非舰长、表情弹幕与普通弹幕。

### 7.2 包解析单测

构造 zlib / brotli 压缩的多包串联二进制样本，验证切分正确。含边界情况：单包、空 body、截断包。

### 7.3 状态机单测

用 fake WebSocket server 驱动状态迁移，验证重连退避、host 轮换、风控降级。

### 7.4 不做的测试

**不对 B 站真实 API 做集成测试。** 不稳定、依赖有效账号、且频繁调用本身就有风控风险。改用录制-回放。

---

## 8. 仓库布局

```
magicaldanmaku/
├── cmd/
│   └── magicd/                  # CLI 入口（P0 提供 probe 子命令）
├── internal/
│   ├── event/                   # 事件模型（P0 核心资产）
│   │   ├── event.go             # Event 信封
│   │   ├── payload.go           # 各类 Payload
│   │   ├── user.go              # User / Medal 值对象
│   │   └── type.go              # Type 枚举
│   ├── connector/
│   │   ├── connector.go         # Connector / Actions 接口
│   │   └── bilibili/
│   │       ├── auth/            # 扫码登录、Cookie、wbi、buvid
│   │       ├── wire/            # 包编解码、zlib/brotli 解压
│   │       ├── cmd/             # CMD → Event 映射，一 CMD 一文件
│   │       ├── action/          # 发弹幕、禁言
│   │       └── client.go        # 状态机与生命周期
│   └── ratelimit/
├── testdata/
│   └── cmds/                    # 黄金样本
└── go.mod
```

**关键约束：CMD → Event 映射必须一个 CMD 一个文件**，通过注册表（`map[string]Mapper`）分发。彻底摆脱原项目 3029 行的 if/else 链。新增 CMD 只加文件，不改现有代码。

### 8.1 与现有 C++ 代码的共存

Go 代码放在仓库新建的 `server/` 顶层目录，C++ 代码原样保留在当前位置作为参考资料。等 Go 版本功能对齐后再决定是否清理 C++ 部分。

理由：C++ 代码在 P0～P2 期间会被反复查阅，删除或移动会增加考古成本；两套构建系统在不同目录下互不干扰；此决策可逆。

---

## 附录 A：已锁定的全局决策

以下决策适用于整个重构项目，不限于 P0：

| 决策项 | 结论 |
|---|---|
| 后端语言 | **Go**（纯重写，C++ 仅作参考） |
| 规则表达方式 | **弃用原「神奇代码」DSL**，改用嵌入式脚本 |
| 脚本引擎 | **goja**（JavaScript / ES5.1+，纯 Go，天然白名单沙箱） |
| 直播平台 | **只做 B 站**，但 Connector 保持干净接口以便扩展 |
| 部署形态 | 需同时支持：单用户单房间 / 单用户多房间 / 多用户多房间 / **多用户单房间（角色分工）** |
| 权限模型 | 房间内 RBAC（主播 / 运营 / 房管等角色） |
| 前端 | Vue3 + TypeScript，用 `embed.FS` 打进 Go 二进制（*建议，待 P4 确认*） |
| 数据库 | SQLite 默认，PostgreSQL 可选（*建议，待 P3 确认*） |
| 分发目标 | win / darwin / linux × amd64 / arm64，加 Docker |
| 收费限制 | 全部移除 |

## 附录 B：保留与删除的功能清单

**保留**：弹幕接收与事件流、自动欢迎进场、礼物/关注/上舰答谢、关键词自动回复、定时任务、事件动作、自动禁言与永久禁言、弹幕记录与查询、多账号轮换发言、数据统计看板、PK 大乱斗

**删除**：`www/` OBS 浏览器源托管、五子棋等 extension、点歌姬与音乐播放器、ChatGPT AI 聊天、Qt 桌面 UI、所有付费与权限校验、直播间运营动作（改标题/分区/封面、开播下播、每日签到、天选之人、挂机领小心心）、语音播报 TTS

**默认不做**（未来可加）：抽奖、私信收发、弹幕远程控制、AI 粉丝档案分析、录播下载、舰长/高能榜管理面板、用户档案与黑白名单、biliup 录播联动

## 附录 C：子项目路线图

| # | 子项目 | 交付物 | 依赖 |
|---|---|---|---|
| **P0** | 协议内核 | 打印实时事件流的 CLI | — |
| **P1** | 分发流水线 | GoReleaser + Docker buildx，8 目标一键出包 | P0 |
| **P2** | 规则引擎 | 配置驱动的无头机器人 | P0 |
| **P3** | 多租户数据层 | User/Room/Membership/Rule/Script/EventLog + RBAC | P2 |
| **P4** | API + WebUI | 完整 Web 管理界面 | P3 |
| **P5** | 数据看板 | 统计聚合与图表 | P4 |
| **P6** | PK 大乱斗 | 独立模块，订阅事件流 | P0, P2 |

P1 故意排在 P2 之前：越早验证 8 目标交叉编译，越早暴露依赖踩坑。
