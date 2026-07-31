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

页面按**主播的使用心态**划分，不按数据表划分。一个开播中的主播想的是
「欢迎语要不要改」「刚才那个刷屏的禁了没」，不是「我要编辑 rules 表」。

| # | 页面 | 内容 |
|---|---|---|
| 1 | **登录** | 用户名 + 密码 |
| 2 | **账号与直播间** | 每个 B 站账号的**登录状态**与其绑定的直播间 |
| 3 | **房管** | 禁言 / 拉黑相关的全部功能，分主播区与房管区 |
| 4 | **弹幕姬** | 核心页面：进房欢迎、礼物答谢、PK 播报、轮播 |
| 5 | **自定义弹幕姬** | 触发器 + 模板的自由组合，可屏蔽通用规则 |
| 6 | **统计** | 分账号分直播间的直播数据 |
| 7 | **日志** | 分账号的业务日志，支持搜索、过滤、清除 |
| 8 | **管理** | 普通用户只有改密码；管理员另有用户管理面板 |

#### 页面 2：账号与直播间

**B 站的登录态失效得很快**，这是本页面存在的首要理由——不能等到发弹幕失败
才发现账号掉线了。

- 账号列表，每个账号显示：昵称、UID、**登录状态**（有效 / 已失效 / 检测中）、上次检测时间
- 登录状态由后端**定期主动检测**（见 §13.1），不是等操作失败才发现
- 失效的账号在列表里高亮，并给出「重新扫码」按钮
- 扫码登录：生成二维码、轮询、成功后落库
- 每个账号下挂它绑定的直播间，可增删、可单独启停
- 账号参数：发送间隔、单条字数上限

#### 页面 3：房管

**分两个区，因为权限不同**：

| 区 | 功能 | 谁能用 |
|---|---|---|
| 房管区 | 禁言、解除禁言、自动禁言关键词、指定昵称自动禁言 | 主播本人与粉丝房管 |
| 主播区 | 拉黑、解除拉黑、自动拉黑关键词、指定昵称拉黑 | 仅主播本人 |

**关键词与昵称匹配都要同时支持通配符与正则**。指定昵称自动处理不是可有可无
的功能：有时会有名字极其离谱的账号进房，引来平台巡查，直接导致直播间被封。

**权限不足时警告，但绝不把面板变灰锁死。**

这是本页面最重要的交互决定。理由：我们无法可靠地预判某个账号在某个直播间
到底有没有房管权限——B 站可能刚给、刚撤，或者接口返回的状态是滞后的。
把面板灰掉意味着「我判断你没权限，所以不让你试」，而这个判断本身可能是错的。

正确做法：

- 检测到当前账号在该直播间不是房管时，页面顶部显示**警告条**，说明「该账号在本直播间似乎没有房管权限，操作可能失败」
- 所有控件**保持可用**
- 用户执行操作后，B 站回退失败 → **把失败如实写进日志**，并在界面上提示这一次失败
- 主播区同理：非主播本人账号使用拉黑功能时警告，但不锁

#### 页面 4：弹幕姬

核心页面。以下每一项都是一个可独立开关的功能块。

**进房欢迎**

- 欢迎筛选：
  - 只欢迎**佩戴粉丝牌**的用户。判定口径见 §13.2——不是「有粉丝牌字段」就算
  - 只欢迎粉丝牌等级 ≥ N 的用户
  - 只欢迎大航海用户（可选等级）
- 欢迎频次限制：**X 分钟内再次进房不重复欢迎**（滚动窗口）
- 自定义欢迎语：
  - 多条模板，**轮询**或**随机抽取**二选一（当前规则引擎只有随机，见 §13.3）
  - **单人欢迎语与多人合并欢迎语是两套独立模板**，不是同一套。
    「欢迎 张三 回家」和「欢迎 张三、李四、王五 回家」句式本就不同，
    共用一套模板必然有一边别扭

**礼物答谢**

- 自定义答谢语，支持多人多礼物合并。模板形如：

  ```
  感谢 {{users}} 的 {{gifts}} 等，您的支持就是对主播最大的鼓励
  ```

  渲染成「感谢 张三、李四 的 小花花、人气票 等，……」

- 归类阈值：**X 秒内送出的礼物算作一轮**，滚动计算，但必须有截止点
  （否则持续送礼永远不结算——P2 的 `maxWait` 正是为此存在）
- **盲盒礼物单列一种类型，不并入常规礼物。** 盲盒爆出来的礼物价值不固定，
  混进常规答谢会让「感谢 X 的 Y」说错东西
- 盲盒盈亏统计：可开关。依赖 §13.4 的协议层补丁

**PK 播报**

- **PK 匹配信息**：PK 接通的**那一瞬间**截取对面数据——主播昵称、直播间人数、
  大航海总数、大航海在线数。只取这一瞬间，之后的变化不再播报
- **PK 串门欢迎**：对面直播间的观众过来时，用单独的欢迎语欢迎，
  与常规进房欢迎区分

依赖 §13.5 的协议层补丁。

**其他**

- 轮播消息：定时发送，多条轮询
- 关注答谢、分享答谢、上舰答谢

#### 页面 5：自定义弹幕姬

给主播完全的自由度：**触发器 + 模板**的组合，参考原项目的 VIP 功能。

- 触发器由几十种变量组合而成，例如
  `指定 UID 进房 且 大航海状态有效` → 发送「欢迎我最心爱的舰长回家」
- 变量清单由后端下发（`/api/meta/*`），前端渲染成可视化的条件构建器
- **必须支持「排除通用规则」**：一条自定义规则命中后，可以声明屏蔽掉哪些
  通用功能。典型场景是「给某位舰长配了专属进房欢迎，就不该再触发通用进房
  欢迎」——否则那位舰长进房会被欢迎两次

  这是规则引擎目前不具备的能力，见 §13.6

#### 页面 6：统计

分账号、分直播间展示：弹幕数、进房人数、礼物种类与数量、上舰数、
直播时长、盲盒盈亏。按场次与按日两个维度。

深度分析（趋势、留存、粉丝画像）仍属 P5，本页面只做当前可从
`activity_logs` 直接聚合出来的部分。

#### 页面 7：日志

分账号的业务日志。支持按关键词搜索、按类型/时间/用户过滤、清除。

「清除」是真的删库，需要二次确认。

#### 页面 8：管理

- 普通用户：只有「修改自己的密码」
- 管理员：另有用户管理面板（增删用户、重置密码、设/撤管理员）

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

## 9. 配置热重载：必须有，显式保存触发

**主播需要边播边调设置，不可能每次改欢迎语都重启主程序。** 热重载是 P4 的
必需功能，不是可选项。

### 9.1 显式保存，不做实时生效

界面右上角一个**保存**按钮。改动先留在前端的草稿状态，按下保存才写库并生效。

这样设计而不是每次输入都生效，有两个理由：

1. **半成品配置不该生效。** 正在把欢迎语从「欢迎{{user}}」改成
   「欢迎{{user}}回家」的中途，不该有观众收到「欢迎{{user}}回」
2. **重载有代价。** 每敲一个字符就重建一次引擎，既浪费又会打断合并窗口

### 9.2 重载的粒度是绑定

重载**以「账号-直播间」绑定为单位**，不是整个进程。改了甲房间的规则，
乙房间的合并窗口与冷却状态不受任何影响。

重载一个绑定的步骤：

1. 从数据库重新 `LoadRunConfig` 该绑定的部分
2. 用新规则构造一个新 `Engine`（构造即校验，非法配置在这一步就被拒绝，
   **旧引擎继续服役**，界面报错说明哪条规则不合法）
3. 新引擎就位后，把事件流切到它上面
4. **旧引擎调用 `Close()`**——这一步会结算它所有未决的合并窗口，
   那批攒着的欢迎语会正常发出去而不是被丢弃（P2 的 `Engine.Close`
   本就是这个语义）
5. 注销旧引擎的定时任务，注册新引擎的

**连接不重建。** WebSocket 连接、账号会话、限流器都属于账号或房间层面，
与规则无关，重载规则不该断连——断连会丢事件，还可能触发风控。

### 9.3 什么需要重启，什么不需要

| 改动 | 是否需要重启 |
|---|---|
| 规则、冷却组、欢迎语、答谢语 | 不需要，保存即重载该绑定 |
| 绑定的启停 | 不需要，停用即关引擎，启用即建引擎并连接 |
| 账号的发送间隔、字数上限 | 不需要，限流器可原地调整 |
| 账号 Cookie（重新扫码） | 不需要，下次请求就用新的 |
| 新增账号、新增绑定 | 不需要，建新引擎与新连接 |
| `MAGICD_HTTP_ADDR` 等环境变量 | 需要 |

只有环境变量需要重启。这是可以接受的——那些是部署参数，不是日常调整的东西。

### 9.4 接口

```
POST /api/bindings/{binding}/reload    重载单个绑定        rule:write
POST /api/reload                       重载全部可见绑定     每个绑定各自校验 rule:write
GET  /api/meta/runtime                 各绑定的运行状态与配置是否已同步
```

`/api/meta/runtime` 仍然返回「运行中的配置哈希」与「数据库里的配置哈希」，
但它的用途从「提示用户去重启」变成「提示用户有未保存/未重载的改动」。

### 9.5 失败必须可见

重载失败（规则非法、连接建不起来）时：

- 旧引擎继续服役，**绝不留下一个没有引擎的绑定**
- 接口返回 422/502 并说明原因
- 界面在该绑定上显示「重载失败，仍在用上一版配置」

悄悄回退到旧配置却显示成功，比重载失败本身糟糕得多。

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
| 9 | ~~不做配置热重载~~ **已被用户推翻**：热重载必需，显式保存触发 | 主播要边播边调设置，不可能每次改欢迎语都重启 | — |
| 10 | P4 拆成 P4-1（API）与 P4-2（前端）两部分 | 范围过大；且前端计划应在接口定型后再写 | 极小 |

**最需要用户复核的是 #1 与 #8** ——它们最难回头。

---

## 13. 前置缺口：P4 依赖但上游还没有的东西

用户要的几项功能，**当前的协议层与规则引擎给不出所需的数据或能力**。
它们必须先补上，P4 才做得出来。每一项都标了归属阶段。

### 13.1 账号登录态检测（新增，归 P4）

B 站登录态失效很快，不能等发弹幕失败才发现。

做法：后台定时（默认 5 分钟）用每个账号的 Cookie 调一个轻量的已登录接口
（`nav` 即可，`api.Client.RefreshNav` 已经在用它），据返回判断登录态。
结果写进内存供 `/api/accounts` 返回，并在失效时写一条系统日志。

新增字段：`accountView.loginState`（`valid` / `invalid` / `unknown`）与
`loginCheckedAt`。**不入库**——它是瞬时状态，进程重启后重新检测即可。

### 13.2 粉丝牌「佩戴」的判定口径（已具备，需写成规则）

数据层已经够用：`event.Medal` 有 `IsLighted`、`RoomID`、`AnchorUID`、`Level`。

判定口径（用户口述的原话是「进房一瞬间有亮起的主播粉丝牌就算佩戴」）：

```
佩戴 ≡ user.medal != nil
     && user.medal.isLighted          // 变灰的不算
     && user.medal.roomId == 本房间号  // 别家的牌子不算
```

两个容易搞错的边角，这个口径都覆盖了：

- 主播开启「当前房间粉丝牌优先展示」时，协议下发的就是本房间的牌子，
  所以 `roomId` 判断自然成立
- 非舰长几天不来粉丝牌会变灰，`isLighted` 为假，自然被排除

**要做的**：把它做成规则条件里的一个现成谓词（如 `user.medal.wearing`），
而不是让用户自己去拼三个条件——三个条件拼错一个就静默失效。

### 13.3 模板轮询与单人/多人分离（归 P2 补丁）

当前 `rules.Action.Template` 是一组模板，执行时**随机挑一条**
（`Renderer` 用 `rand`）。缺两项：

- **轮询模式**：按顺序循环，而不是随机。需要 `Action.Pick` 字段
  （`random` / `roundrobin`）与每条规则的游标状态
- **单人与多人两套模板**：合并窗口结算时，`count == 1` 与 `count > 1`
  应当取不同的模板集。需要 `Action.TemplateGroup` 之类的字段

这两项都要改 `internal/rules`，属于 P2 的补丁。

### 13.4 盲盒礼物（归 P0 补丁）

**当前 `event.Gift` 完全没有盲盒字段。** B 站 `SEND_GIFT` 的报文里有
`blind_gift` 对象（含 `original_gift_id`、`original_gift_name`、`gift_action`），
`cmdmap` 没有解析它。

要补：

- `event.Gift` 增加 `BlindBox *BlindBox` 字段，`BlindBox` 含原礼物名、原礼物 ID
- `cmdmap` 的 `SEND_GIFT` 映射解析该字段
- **新增事件类型 `TypeBlindBoxGift`**，或在规则条件里提供 `gift.isBlindBox`
  谓词——用户明确要求「盲盒单列一种礼物类型，不算常规礼物」
- 盈亏 = 爆出礼物的实际价值 − 盲盒本身的价格。两者都在报文里，但要
  **先抓真实样本确认字段名**，不能凭记忆写

盈亏统计还有一条已知的坑（记在用户的记忆文件里）：**必须按电池数量统计而非
礼物名，且礼物滚动合并时盈亏要跟着重算**。

### 13.5 PK 匹配信息与串门（归 P0 补丁）

**当前 `event.Battle` 只有一个 `SubCommand` 字符串，什么都没解析。**

用户要的「对面主播昵称、直播间人数、大航海总数、大航海在线数」，
需要解析 `PK_BATTLE_START_NEW` / `PK_BATTLE_PRE_NEW` 等 CMD 的完整报文。
**只截取 PK 接通那一瞬间的数据**，之后的变化不再播报。

串门欢迎需要判断「这个进场的观众来自对面直播间」。B 站的进场事件里可能带
来源房间信息，但**当前没解析，也没确认字段是否存在**——这一项必须先抓真实
的 PK 场景样本才能确定做不做得出来。

**这两项都需要在真实 PK 场景下抓包。** 计划里不该写死字段名，而是先安排
一个采样任务。

### 13.6 规则排除（归 P2 补丁）

用户要求：自定义规则命中后，可以屏蔽指定的通用规则。典型场景是给某位舰长
配了专属进房欢迎，就不该再触发通用进房欢迎。

当前的 `rules.Engine` 把每条匹配的规则各自触发，规则之间互不知道。

设计方向（细节留给 P2 补丁的设计）：

- 规则增加 `Suppresses []string` 字段，列出本规则命中时要屏蔽的规则名
- `Engine.Handle` 在匹配出规则集之后、触发之前，先算出被屏蔽的集合再触发
- **屏蔽只在同一个事件内生效**，不是持久开关
- 合并窗口下的屏蔽语义要单独想清楚：专属规则在窗口里命中了，通用规则的
  那个窗口该不该把这个用户排除掉？（倾向于：该，否则合并欢迎里还会带上他）

### 13.7 前置缺口的执行顺序

```
P0 补丁（盲盒字段、PK 报文、串门来源）  ← 需要先抓真实样本
P2 补丁（模板轮询、单人多人模板、规则排除）
        ↓
P4-1 HTTP API（含热重载、登录态检测）
        ↓
P4-2 WebUI
```

P0 与 P2 的补丁互不依赖，可以并行。**P4-1 的大部分接口不依赖这些补丁**——
只有盲盒、PK、排除这三块的接口要等。因此 P4-1 可以先做，缺口部分留到补丁
落地后再补接口，不必阻塞整个阶段。
