# 幽灵弹幕检测

2026-08-09

## 起因

桃酥直播间（1781257934）的粉丝竹七痕（uid 1434809451）发不出弹幕，但礼物、
SC 都正常，且未被禁言、在别的直播间能发言。真机数据把现象钉死了：

| event_type | 计数 | 最后一次（北京时间） |
|---|---|---|
| danmaku | 468 | 2026-08-07 10:14:23 |
| gift | 126 | 2026-08-08 12:46:24 |
| super_chat | 1 | 2026-08-08 12:30:29 |
| user_enter | 10 | 2026-08-08 12:45:49 |

8-08 那天他在房间待了一个多小时，送礼十几次、点赞、发了一条 SC，弹幕 0 条；
同期全房 708 条弹幕、33 人发言，其中包含 UL=0 / 荣耀=0 / 无粉丝牌的白板号。
房间没有等级门槛（否则先被挡的是那些白板号），他本人是房管，全库唯一一条
`ROOM_BLOCK_MSG` 的对象是机器人而不是他，主播也确认后台没有任何限制设置。

也就是说：**弹幕发出去了，服务端收下了，但没有广播给房间**。这是 B 站的
隐形风控（隐形关键词、隐形 UL 门槛等），发送者自己看得见自己的弹幕，别人
看不见。原作者的 Qt 版弹幕姬能把这类问题打印出来，本项目缺这个能力。

## 判据

> `msg/send` 返回成功（code=0），但 N 秒内收不到自己的回显 = 幽灵弹幕。

这个判据在真机历史数据上验证过。8-07 机器人被禁言的窗口里有 3 条轮播没有
回显，但它们**不是幽灵**——B 站明确返回了 `code=1003: 你已经在本房间被禁言`：

```
10:15:27  【ROOM_BLOCK_MSG】禁言 子どもの箱
10:18:00  轮播 → 无回显，send 报错 code=1003
10:21:00  轮播 → 无回显，send 报错 code=1003
10:24:01  轮播 → 无回显，send 报错 code=1003
10:27:00  轮播 → 02:27:01 恢复回显
```

禁言、频率过快（code=10030）这类都是**显式拒绝**，发送这一步就失败了，
根本不会进入登记，判据自动把它们排除。当天其余 32 条弹幕全部有回显，
延迟稳定在 0.2~1.5 秒，假阳性 0 条。

## 组件

### `internal/ghost`

纯逻辑：不碰网络、不起 goroutine、时钟由调用方注入，可以在单测里把整条
时序跑完而不用 sleep。

```go
// Sent 登记一条已成功发出的弹幕。分段发送时每段各登记一条。
func (d *Detector) Sent(roomID, uid, text string, at time.Time)

// Observe 用收到的一条自己的弹幕销账。
func (d *Detector) Observe(roomID, uid, text string, at time.Time)

// Sweep 判定超过 window 仍未销账的项，逐条回调。
// online 返回 false 的房间顺延，不判定。
func (d *Detector) Sweep(now time.Time, online func(roomID string) bool)
```

pending 按 `(roomID, uid)` 分桶，桶内 FIFO。

两个回调：

- `OnGhost` —— 发送成功但超时无回显
- `OnMismatch` —— 收到了自己 uid 的回显，但文本和发出去的不一致

第二个是附带能力，但信息量比第一个更大：它正是隐形关键词被改写的指纹。

### 接线

| 位置 | 做什么 |
|---|---|
| `account.Binding.SendDanmaku` | `segs, err := Actions.SendDanmaku(...)`，成功后逐段 `Sent()` |
| `runtime_manager` 事件循环 | `ev.Type==danmaku && ev.User.UID==本绑定账号` → `Observe()` |
| 1 秒 ticker | 驱动 `Sweep()`，`online` 由 `client.State()==StateConnected` 提供 |

`account.Binding` 是唯一同时握有 uid、roomID 和 Actions 的地方，登记必须
放在这一层。

## 三类假阳性抑制

### 1. 分段发送

`Actions.SendDanmaku` 内部用 `SplitLongText` 把超长文本切成多条依次发送，
回显自然也是分开的。按整条登记就永远匹配不上，**所有超长弹幕 100% 误报**。

因此把接口签名改成返回实际发出的每一段：

```go
// 旧
SendDanmaku(ctx context.Context, req SendDanmakuRequest) error
// 新
SendDanmaku(ctx context.Context, req SendDanmakuRequest) ([]string, error)
```

"到底发了哪几条"本来就是调用方的合法关切，分段是实现细节但结果不是。
波及 `connector.Actions` 接口、`bilibili.Actions` 实现、`account.Binding`
调用点，以及两处测试 stub。

`rules.BotAPI.SendDanmaku(text) error` 和 `httpapi` 的同名方法在不同层，
不做登记，签名不动。

### 2. 断线

WS 断开期间收不到任何回显，此时判定就是纯误报。`Sweep` 对 `online` 为
false 的房间顺延，不判定也不丢弃。

### 3. 重复文本

轮播三分钟一条，可能重复相同文本。桶内 FIFO，回显匹配最早的那条未销账项。

## 主动探针

`POST /api/bindings/{id}/ghost-probe`

发一条带随机短后缀的文本（避免和业务弹幕撞），同步等最多 window 秒。

```json
{"result": "ok" | "ghost" | "mismatch" | "send_failed",
 "code": 1003, "message": "...", "echoText": "..."}
```

`send_failed` 时带上 B 站的原始 code 和 message——那本身就是答案。

必须是 POST。本项目用 `SameSite=Lax` 顶掉 CSRF token，任何改变状态的端点
都不能走 GET。

WebUI 在绑定页加一个「测一下」按钮。

## 日志

```
WARN 疑似幽灵弹幕：发送成功但未收到回显  binding=子箱@1781257934 text="..." waited=10s
WARN 弹幕回显与发送不一致，疑似关键词改写  binding=... sent="..." echo="..."
```

只打日志，不落库。落库要先补 `activity_logs` 的两个缺口（动作日志既没记
实际发出的文本，只记了模板候选列表；也没记发送结果，失败的动作照样入库），
那是另一件事，这次不做。

## 配置

| 键 | 默认 | 说明 |
|---|---|---|
| `ghost.enabled` | true | 总开关 |
| `ghost.window` | 10s | 等待回显的超时窗口，约 7 倍于实测上界 |

## 测试

`internal/ghost` 纯单测，时钟注入不用 sleep：

- 正常销账不报
- 超时报幽灵
- 分段各自登记、各自判定
- 重复文本按 FIFO 销账
- 断线期间顺延不判定，恢复后正常判定
- 回显文本不一致报 mismatch 而不是 ghost

`account` 层：`SendDanmaku` 成功后确实逐段登记（fake Actions 返回 2 段）。

## 已知限制

1. **只覆盖本系统账号发出的弹幕。** 竹七痕这样的普通观众，除非他的 B 站
   账号登录进本系统，否则这个功能对他给不出任何结论。这次不做账号接入。

2. **B 站是否会对文本做无害规范化（首尾空格、全半角之类）没有在真机验证
   过。** 如果会，`OnMismatch` 会有噪声。上线后先观察，必要时再加规范化
   白名单——现在就加等于凭想象写规则。
