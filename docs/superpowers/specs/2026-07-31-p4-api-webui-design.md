# P4 API + WebUI 设计文档

> 前置：P0 协议内核、P1 分发流水线、P2 规则引擎、P3 多租户数据层。
> P3 交付了存储层与授权模型，本阶段把它们暴露成 HTTP API，并配一个 Web 管理界面。

> **本文档在夜间无人值守时段写成。** 凡是本该征询用户的决策，都由控制器定夺并在
> 第 12 节「夜间自行定夺的决策」逐条列出，供用户复核。任何一条都可以推翻，
> 代价在该节中标明。

## 1. 目标

P4 结束时：

- 浏览器里能完成日常运维的全部操作：扫码登录 B 站账号、增删直播间、编辑规则、看实时弹幕、手动发言与禁言、授权他人
- 后端不依赖任何窗口，`magicd run` 一条命令同时跑机器人与 Web 服务
- 前端与后端彻底分离：浏览器只通过 HTTP/JSON 与后端交互，没有服务端模板渲染
- 权限在 API 层强制执行，P3 定义的七个权限点每一个都有对应的守卫

## 2. 进程模型：机器人与 API 同进程

**`magicd run` 同时跑机器人与 HTTP 服务，一个进程。** 不拆成两个可执行文件。

理由：

1. **实时事件流需要它。** 网页要看实时弹幕。机器人已经持有 `bilibili.Client.Events()` 这条通道；同进程可以直接扇出给 SSE 订阅者。拆成两个进程就得再造一套跨进程的事件总线（Redis、NATS 或数据库轮询），为一个单机工具引入这种复杂度不值得。
2. **用户的原始需求是「前后端分离」，指的是浏览器与服务端分离，不是机器人与 API 分离。** 后者是实现细节。
3. **部署简单。** 一个二进制、一个容器、一个进程要看护。

代价：改规则要重启才生效（见 §9 热重载）。这个代价明确接受。

HTTP 监听地址由 `MAGICD_HTTP_ADDR` 控制，**默认 `127.0.0.1:8080`**——默认只监听本机，不因为用户忘了配防火墙就把管理界面暴露到公网。Docker 部署需显式设为 `0.0.0.0:8080`，这一点写进部署文档。设为空字符串则完全不起 HTTP 服务，退化成纯机器人。

## 3. 会话与认证

### 3.1 服务端会话，不用 JWT

用服务端会话 + Cookie，不用 JWT。

理由：JWT 的卖点是无状态横向扩展，本项目是单进程单机，用不上；而它的代价是撤销困难（改密码、踢人都需要额外的黑名单机制）。服务端会话反过来：多一张表，换来即时撤销与「看谁在线」。

### 3.2 会话表

```sql
CREATE TABLE sessions (
    token_hash TEXT        PRIMARY KEY,          -- SHA-256(token)，不存原文
    user_id    BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    user_agent TEXT        NOT NULL DEFAULT ''
);
CREATE INDEX sessions_user_idx    ON sessions (user_id);
CREATE INDEX sessions_expires_idx ON sessions (expires_at);
```

这是 P4 的第一个迁移 `002_sessions.sql`。P3 的「只有一个迁移文件」是 P3 阶段的约束，不是永久的。

**会话令牌存哈希而非原文。** 与 B 站 Cookie 明文存储的处理不同，原因是两者的约束不同：B 站 Cookie 必须能还原成原文才能拿去请求 B 站，所以要么明文要么可逆加密；会话令牌只需要验证「你给的这个等不等于我发的那个」，单向哈希就够。既然哈希是免费的，就没有理由不做。

令牌 32 字节 `crypto/rand`，`base64.RawURLEncoding` 编码。

### 3.3 Cookie 设置

`HttpOnly`（阻断 XSS 读取）、`SameSite=Lax`（阻断跨站 CSRF，同时不影响正常导航）、`Path=/`。

`Secure` 标志按 `MAGICD_HTTP_SECURE_COOKIE` 决定，默认关闭——默认监听 `127.0.0.1`，走的是明文 HTTP，此时打 `Secure` 会让 Cookie 根本发不出去。反向代理加了 TLS 的部署要显式打开。

有效期 30 天，可用 `MAGICD_SESSION_TTL` 调整。每次请求不滑动续期（实现简单，且 30 天足够长）。

### 3.4 CSRF

`SameSite=Lax` 已经挡住了跨站表单提交与跨站 XHR。P4 **不再额外做 CSRF token**。

需要注意的是 `Lax` 不拦截跨站的顶层 GET 导航，所以**所有会改变状态的接口一律不用 GET**——这是本项目的硬性约定，写进 §5 的接口表里。

## 4. 授权

### 4.1 两层守卫

```
请求 → 认证中间件（有没有有效会话？）→ 授权守卫（这个人对这个绑定有没有这个权限点？）→ 处理器
```

认证中间件把 `*store.User` 放进请求 context。授权守卫是一个按权限点参数化的包装函数：

```go
// requirePerm 返回一个中间件，它要求当前用户对 URL 里的 {binding} 拥有 p。
func (s *Server) requirePerm(p perm.Permission, h http.HandlerFunc) http.HandlerFunc
```

守卫内部调用 P3 的 `store.Can(userID, bindingID, p)`——**授权判定只有这一处实现**，处理器里不得再写权限判断。

### 4.2 权限点与接口的对应

| 权限点 | 守卫哪些接口 |
|---|---|
| `rule:read` | 读规则、读冷却组 |
| `rule:write` | 增删改规则、启停规则、改冷却组 |
| `danmaku:send` | 手动发弹幕 |
| `user:block` | 禁言/解禁、维护禁言名单 |
| `account:manage` | 改账号的 Cookie 与限流参数 |
| `member:manage` | 授权他人、撤销授权 |
| `event:read` | 实时事件流、历史业务日志 |

管理员（`users.is_admin`）绕过全部检查，这在 `store.Can` 里已经实现。

### 4.3 不属于绑定的资源

用户管理、账号创建与删除不挂在绑定上，因此不走 `requirePerm`：

- **创建/删除/列出用户**：仅管理员
- **创建账号（扫码登录）**：任何已登录用户，新账号归属自己
- **删除账号**：账号所有者或管理员
- **列出账号**：只返回「自己拥有的」加「自己在其某个绑定上有任意权限的」

最后一条要特别注意：**列表接口必须按调用者的可见范围过滤**，不能返回全部再让前端隐藏。这是最容易漏的一类越权。

## 5. HTTP 接口

### 5.1 约定

- 前缀 `/api`，`Content-Type: application/json`
- 错误统一 `{"error": "人类可读的中文说明"}`，HTTP 状态码承载语义
- `401` 未登录、`403` 已登录但无权限、`404` 不存在或**对调用者不可见**（不区分，避免探测）、`409` 冲突、`422` 请求体不合法
- **一切改变状态的接口都不用 GET**（见 §3.4）
- 路由用 Go 1.22+ 标准库 `http.ServeMux` 的方法与通配模式（`POST /api/bindings/{id}/rules`），**不引入第三方路由库**

### 5.2 接口清单

```
认证
  POST   /api/auth/login              {username, password} → 种 Cookie，返回用户
  POST   /api/auth/logout             撤销当前会话
  GET    /api/auth/me                 当前用户 + 其全部授权

用户（仅管理员，改自己密码除外）
  GET    /api/users
  POST   /api/users                   {username, password, isAdmin}
  POST   /api/users/{name}/password   {oldPassword, newPassword} 改自己的需带旧密码；管理员改他人不需要
  DELETE /api/users/{name}

B 站账号
  GET    /api/accounts                按可见范围过滤
  POST   /api/accounts/qrcode         开始扫码 → {key, url}
  POST   /api/accounts/qrcode/{key}   轮询一次 → {status} ；成功时按 name 建号或换 Cookie
  PATCH  /api/accounts/{name}         {rateLimitMs, maxLength}      account:manage
  DELETE /api/accounts/{name}         所有者或管理员

绑定
  GET    /api/bindings                按可见范围过滤，带上调用者在每个绑定上的权限点
  POST   /api/bindings                {accountName, roomId}
  PATCH  /api/bindings/{id}           {enabled}                      rule:write
  DELETE /api/bindings/{id}           账号所有者或管理员

规则
  GET    /api/bindings/{id}/rules                 rule:read
  PUT    /api/bindings/{id}/rules                 整组替换                rule:write
  POST   /api/bindings/{id}/rules                 新增单条                rule:write
  PUT    /api/bindings/{id}/rules/{name}          覆盖单条                rule:write
  PATCH  /api/bindings/{id}/rules/{name}          {enabled} 只切启停       rule:write
  DELETE /api/bindings/{id}/rules/{name}                                rule:write
  POST   /api/bindings/{id}/rules/validate        只校验不保存             rule:read

冷却组
  GET    /api/bindings/{id}/cooldown-groups       rule:read
  PUT    /api/bindings/{id}/cooldown-groups       rule:write

禁言名单
  GET    /api/bindings/{id}/blocklist             user:block
  POST   /api/bindings/{id}/blocklist             user:block
  DELETE /api/bindings/{id}/blocklist/{uid}       user:block

业务日志与实时流
  GET    /api/bindings/{id}/activity              event:read  分页历史
  GET    /api/bindings/{id}/stream                event:read  SSE 实时事件

即时动作
  POST   /api/bindings/{id}/danmaku               {text}            danmaku:send
  POST   /api/bindings/{id}/block                 {uid, hours}      user:block
  POST   /api/bindings/{id}/unblock               {uid}             user:block

授权
  GET    /api/bindings/{id}/members                                 member:manage
  PUT    /api/bindings/{id}/members/{username}    {permissions:[]}   member:manage
  DELETE /api/bindings/{id}/members/{username}                      member:manage

元数据（给前端渲染用，只需登录）
  GET    /api/meta/permissions        七个权限点及中文说明
  GET    /api/meta/event-types        事件类型清单及中文说明
  GET    /api/meta/action-types       动作类型清单
  GET    /api/meta/template-funcs     模板函数清单及签名
  GET    /api/meta/runtime            当前进程里每个绑定的连接状态

无需认证
  GET    /api/health                  存活探针
```

### 5.3 扫码登录的两步

浏览器里的扫码登录复用 P0 的 `auth.QRLogin`，但要把「生成」与「轮询」拆成两个无状态的 HTTP 调用：

1. `POST /api/accounts/qrcode` 带 `{name}`（要建的账号名）。服务端调 `QRLogin.Generate`，把 `{key → name, userID}` 存进一个**带 TTL 的内存表**（3 分钟，二维码本身就 3 分钟失效），返回 `{key, url}`。前端自己把 url 渲染成二维码图片。
2. `POST /api/accounts/qrcode/{key}` 轮询。服务端调 `QRLogin.Poll`。返回 `waiting` / `scanned` / `expired` / `success`。**成功时在服务端完成建号或换 Cookie**，Cookie 绝不回传给浏览器。

内存表用 `sync.Map` 加定时清理，不入库——待确认的扫码会话是纯瞬态的，进程重启后重新扫即可。

**Cookie 绝不出现在任何响应体里。** 这是硬性约束，`GET /api/accounts` 返回的账号对象里没有 `cookie` 字段。

## 6. 实时事件流

### 6.1 用 SSE 而非 WebSocket

`text/event-stream`，服务端单向推送。

理由：需求是单向的（服务端 → 浏览器），SSE 走普通 HTTP、浏览器原生 `EventSource` 自带断线重连、没有握手协议要维护。WebSocket 的双向能力用不上，为它引入自己的一套连接管理不划算（项目里已有的 `gorilla/websocket` 是用来连 B 站的，与对外服务无关）。

### 6.2 事件中枢

```go
// Hub 把机器人收到的事件扇出给 SSE 订阅者。
type Hub struct { ... }

func (h *Hub) Publish(bindingID int64, ev event.Event)
func (h *Hub) Subscribe(bindingID int64) (<-chan event.Event, func())
```

接进 `run` 的事件循环：

```go
for ev := range client.Events() {
    engine.Handle(ev)          // 规则处理（原有）
    hub.Publish(bindingID, ev) // 扇出给网页（新增）
}
```

**订阅者的通道满了就丢弃该订阅者的这条事件，绝不阻塞 `Publish`。** 与 `ActivityWriter` 同一条原则：网页看丢一条弹幕可以接受，拖慢规则处理不行。每个订阅者一个容量 256 的缓冲通道。

`Publish` 必须在 `engine.Handle` **之后**调用。若放在前面，网页会先看到事件、后看到机器人的响应，但机器人的响应本身也是通过弹幕事件回流的，顺序颠倒会让因果看起来是反的。

### 6.3 断线与心跳

每 30 秒发一个 SSE 注释行 `: keepalive` 防止中间代理掐断空闲连接。客户端断开由 `r.Context().Done()` 感知，随即退订。

## 7. 前端

### 7.1 技术选型

**Vue 3 + TypeScript + Vite**，产物构建到 `web/dist`，用 `embed.FS` 打进 Go 二进制。

这条在 P0 的全局决策附录里就是建议方案，本阶段确认。理由：规则编辑器要处理**任意嵌套的条件树**（`all`/`any`/`not` 可递归），这是组件化框架的主场，用原生 DOM 操作写会非常痛苦且难维护。

**构建产物必须提交进仓库。** 这样 `go build` 不需要 node，六平台交叉编译与 GoReleaser 流程完全不变；只有改前端时才需要 node。仓库里放一个占位的 `web/dist/index.html`，保证任何时候 `go build` 都能过（`embed.FS` 在目录不存在时会编译失败）。

### 7.2 页面

| 页面 | 内容 |
|---|---|
| 登录 | 用户名 + 密码 |
| 总览 | 绑定卡片墙：连接状态、今日事件数、规则数、快捷开关 |
| 直播间 | 实时弹幕流 + 手动发言框 + 一键禁言 |
| 规则编辑 | 规则列表 + 条件树编辑器 + 模板编辑器 + 试运行 |
| 账号 | 账号列表、扫码登录、限流参数 |
| 授权 | 某绑定上的成员与权限点，含「运营」「房管」预设一键勾选 |
| 日志 | 业务日志查询：按类型、时间、用户过滤 |
| 用户 | 仅管理员：增删用户、改密码 |

### 7.3 权限预设在这里展开

P3 的设计明确：**角色是若干权限点的预设组合，展开发生在界面层，存储层只有权限点。** 本阶段兑现它：

```ts
const PRESETS = {
  运营: ['rule:read', 'rule:write', 'event:read'],
  房管: ['user:block', 'event:read'],
  观察: ['rule:read', 'event:read'],
}
```

按钮只是一键勾选一组复选框，提交给后端的永远是展开后的权限点数组。

## 8. 包结构

```
server/internal/httpapi/
  server.go          Server 结构、路由注册、优雅关停
  middleware.go      认证中间件、授权守卫、请求日志、panic 恢复
  respond.go         JSON 响应与错误响应的唯一出口
  session.go         会话的创建、校验、撤销
  auth_handler.go    login / logout / me
  user_handler.go    用户管理
  account_handler.go 账号与扫码登录
  binding_handler.go 绑定
  rule_handler.go    规则与冷却组
  block_handler.go   禁言名单与即时禁言
  activity_handler.go 业务日志与 SSE
  member_handler.go  授权
  meta_handler.go    元数据
  hub.go             事件扇出中枢
  static.go          embed 前端产物与 SPA 回退路由

server/internal/store/
  session.go         会话表读写（P4 新增）
  migrations/002_sessions.sql

web/                 前端源码（Vue3 + TS + Vite）
web/dist/            构建产物，提交进仓库
```

`httpapi` 包不直接依赖 `bilibili`，只依赖 `store`、`perm`、`event`、`connector`（为了 `Actions` 接口与 `State`）。`run.go` 负责把具体实现注进去。

## 9. 配置热重载：本阶段不做

在网页上改了规则，**需要重启 `magicd run` 才生效**。

不做热重载的理由：规则引擎的 `Engine` 在构造时就完成了规则校验、合并器创建与定时任务注册，运行期替换需要停掉全部合并窗口、重注册 cron、处理正在执行的动作——这是一整套并发生命周期管理，值得单独一个阶段，塞进 P4 会让本已很大的范围失控。

**但界面必须诚实地暴露这一点**：改动保存后，页面上出现「配置已保存，重启后生效」的提示，且总览页显示「当前运行的配置版本」与「数据库里的配置版本」是否一致。不做热重载可以接受，让用户以为已生效不行。

实现方式：`run` 启动时把 `LoadRunConfig` 的结果算一个哈希存进内存，API 提供 `GET /api/meta/runtime` 返回该哈希与当前数据库配置的哈希。

## 10. 测试策略

- **处理器测试用 `httptest.NewServer` 打真实 HTTP**，不直接调用 handler 函数——路由匹配、方法限制、中间件顺序都是要测的行为
- **数据库仍然用真库**，沿用 P3 的 `testStore` 独立 schema 基座
- **每个受权限守卫的接口，至少有一个「无权限则 403」的测试**。这是本阶段最容易漏且后果最重的一类缺陷，不能只测 happy path
- **列表接口必须有「看不到别人的东西」的测试**，光测「能看到自己的」不够
- SSE 用 `httptest` + 读取响应流的前几个事件，不做长连接压测
- 前端不做单元测试。E2E 留给未来，本阶段靠人工走查

## 11. 阶段拆分

P4 的范围明显大于前几个阶段，拆成两个可独立交付的部分：

| | 内容 | 交付物 |
|---|---|---|
| **P4-1** | HTTP API、会话认证、权限守卫、SSE、接进 `run` | 一套可以用 curl 完整驱动的 API |
| **P4-2** | Vue3 前端与嵌入 | 浏览器里的管理界面 |

P4-1 不依赖任何前端决策，可以独立完成并测试。P4-2 的详细计划在 P4-1 落地、接口形状固定之后再写——**前端计划里写死一个尚未实现的接口的字段名，是在给自己埋返工**。

本文档同时是 P4-2 的设计依据，其页面与预设已在 §7 定稿。

## 12. 夜间自行定夺的决策

以下决策本应征询用户，因夜间无人值守由控制器定夺。每条都标出了推翻它的代价。

| # | 决策 | 理由 | 推翻的代价 |
|---|---|---|---|
| 1 | 机器人与 API 同进程，`magicd run` 一条命令 | 实时事件流需要共享进程内的事件通道；拆进程要引入跨进程总线 | 中：要造事件总线，或放弃实时流改用轮询数据库 |
| 2 | 服务端会话 + Cookie，不用 JWT | 单机单进程用不上无状态；换来即时撤销 | 小：`session.go` 与一张表换成 JWT 签发校验 |
| 3 | 会话令牌存 SHA-256 哈希 | 只需验证相等，不需还原，哈希免费 | 极小 |
| 4 | 默认监听 `127.0.0.1:8080` | 不因忘配防火墙而暴露管理界面 | 极小：改默认值 |
| 5 | 不做 CSRF token，靠 `SameSite=Lax` + 禁止 GET 改状态 | Lax 已挡住跨站提交；额外 token 是重复防护 | 小：加一层中间件 |
| 6 | 实时流用 SSE 不用 WebSocket | 单向需求，浏览器原生重连，无需连接管理 | 中：改协议要动前后端两侧 |
| 7 | 路由用标准库 `ServeMux`，不引第三方 | Go 1.22+ 已支持方法与通配模式，够用 | 小 |
| 8 | 前端 Vue3 + TS + Vite，产物提交进仓库 | 嵌套条件树编辑器需要组件化；提交产物让 `go build` 不依赖 node | 大：换框架等于重写前端 |
| 9 | 不做配置热重载，但界面必须提示「重启后生效」 | 热重载涉及一整套并发生命周期管理，值得单独阶段 | 大：是一个独立阶段的工作量 |
| 10 | P4 拆成 P4-1（API）与 P4-2（前端）两部分 | 范围过大；且前端计划应在接口定型后再写 | 极小 |

**最需要用户复核的是 #1、#8、#9** ——它们最难回头。
