# P3 多租户数据层 设计文档

> 前置：P0 协议内核、P1 分发流水线、P2 规则引擎均已完成。
> 本文只覆盖持久化与授权，HTTP API 与 WebUI 属于 P4。

## 1. 目标

把目前散落在文件里的状态搬进 PostgreSQL，并建立「谁能对哪个绑定做什么」的授权模型，为 P4 的多人协作 WebUI 打好地基。

具体地，P3 结束时：

- 配置的唯一真相是数据库，`magicd run` 不再需要 `config.yaml`
- 存在系统用户的概念，有用户名与密码
- 每条「账号-直播间」绑定可以被授权给若干用户，授权粒度是权限点
- 弹幕、礼物与机器人动作被记录下来，可按账号查询
- 现有的 `config.yaml` 可以一条命令导进数据库

## 2. 「用户」与「B站账号」是两个东西

这是整个 P3 的地基，此前从未在文档中厘清过。

| | 是什么 | 凭据 | 数量关系 |
|---|---|---|---|
| **User（系统用户）** | 使用本软件的人：张三、李四、王五 | 用户名 + 密码 | 登录 WebUI 的主体 |
| **Account（B站账号）** | 机器人操作的 B 站号：主播号、小号 | Cookie | 被用户拥有与操作 |

一个用户可以拥有多个 B 站账号；一个 B 站账号在某个直播间的操作权，也可以被授权给别的用户。典型场景：

```
张三（主播）    拥有 主播号A、小号B
李四（运营）    被授权改「小号B @ 房间甲」的规则
王五（房管）    只能看「主播号A @ 房间甲」的弹幕并执行禁言，改不了规则
```

## 3. 数据模型

### 3.1 表清单

| 表 | 职责 |
|---|---|
| `users` | 系统用户与密码 |
| `accounts` | B 站账号与 Cookie |
| `bindings` | 账号-直播间组合，即 P2 的运行单元 |
| `memberships` | 授权：某用户对某绑定拥有哪些权限点 |
| `rules` | 规则，规则体以 JSONB 存 |
| `cooldown_groups` | 命名冷却组 |
| `kv_store` | 脚本的 `storage.get/set` |
| `block_list` | 永久禁言名单 |
| `activity_logs` | 业务日志：事件与动作的统一时间线 |

### 3.2 DDL

```sql
CREATE TABLE users (
    id            BIGSERIAL PRIMARY KEY,
    username      TEXT        NOT NULL UNIQUE,
    password_hash TEXT        NOT NULL,          -- bcrypt
    is_admin      BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE accounts (
    id            BIGSERIAL PRIMARY KEY,
    name          TEXT        NOT NULL UNIQUE,   -- "主播号"
    uid           TEXT        NOT NULL DEFAULT '',
    cookie        TEXT        NOT NULL,          -- 明文，见 §3.4
    rate_limit_ms INTEGER     NOT NULL DEFAULT 1500,
    max_length    INTEGER     NOT NULL DEFAULT 40,
    owner_id      BIGINT      NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE bindings (
    id         BIGSERIAL PRIMARY KEY,
    account_id BIGINT      NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    room_id    TEXT        NOT NULL,
    enabled    BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (account_id, room_id)
);

CREATE TABLE memberships (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT      NOT NULL REFERENCES users(id)    ON DELETE CASCADE,
    binding_id  BIGINT      NOT NULL REFERENCES bindings(id) ON DELETE CASCADE,
    permissions TEXT[]      NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, binding_id)
);
CREATE INDEX memberships_permissions_idx ON memberships USING GIN (permissions);

CREATE TABLE rules (
    id         BIGSERIAL PRIMARY KEY,
    binding_id BIGINT      NOT NULL REFERENCES bindings(id) ON DELETE CASCADE,
    name       TEXT        NOT NULL,
    enabled    BOOLEAN     NOT NULL DEFAULT TRUE,
    position   INTEGER     NOT NULL DEFAULT 0,   -- 保持配置顺序
    spec       JSONB       NOT NULL,             -- 规则体，见 §4
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (binding_id, name)
);

CREATE TABLE cooldown_groups (
    id          BIGSERIAL PRIMARY KEY,
    binding_id  BIGINT  NOT NULL REFERENCES bindings(id) ON DELETE CASCADE,
    name        TEXT    NOT NULL,
    interval_ms INTEGER NOT NULL,
    UNIQUE (binding_id, name)
);

CREATE TABLE kv_store (
    binding_id BIGINT      NOT NULL REFERENCES bindings(id) ON DELETE CASCADE,
    key        TEXT        NOT NULL,
    value      TEXT        NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (binding_id, key)
);

CREATE TABLE block_list (
    id         BIGSERIAL PRIMARY KEY,
    binding_id BIGINT      NOT NULL REFERENCES bindings(id) ON DELETE CASCADE,
    uid        TEXT        NOT NULL,
    username   TEXT        NOT NULL DEFAULT '',
    reason     TEXT        NOT NULL DEFAULT '',
    created_by BIGINT      REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (binding_id, uid)
);

CREATE TABLE activity_logs (
    id          BIGSERIAL PRIMARY KEY,
    account_id  BIGINT      NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    binding_id  BIGINT      REFERENCES bindings(id) ON DELETE SET NULL,
    room_id     TEXT        NOT NULL DEFAULT '',
    kind        TEXT        NOT NULL,            -- 'event' | 'action'
    event_type  TEXT        NOT NULL DEFAULT '', -- kind='event' 时填
    action_type TEXT        NOT NULL DEFAULT '', -- kind='action' 时填
    rule_name   TEXT        NOT NULL DEFAULT '', -- kind='action' 时填，哪条规则触发的
    user_uid    TEXT        NOT NULL DEFAULT '',
    user_name   TEXT        NOT NULL DEFAULT '',
    detail      JSONB,                           -- 事件 Payload 或动作详情
    occurred_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX activity_logs_account_time_idx ON activity_logs (account_id, occurred_at DESC);
CREATE INDEX activity_logs_binding_time_idx ON activity_logs (binding_id, occurred_at DESC);
CREATE INDEX activity_logs_type_time_idx    ON activity_logs (event_type, occurred_at DESC);
```

### 3.3 几处刻意的选择

**`rules.spec` 用 JSONB 而不展开成表。** 条件是递归树（`all`/`any`/`not` 可任意嵌套），normalize 进关系表会得到一张自引用表和一堆递归查询，而收益是零——我们从不按条件内容做 SQL 查询。P4 的 API 收发的本来也是 JSON。校验留在 Go 侧，`rules.Rule.Validate()` 已经存在。

**`name` 与 `enabled` 是列，不进 JSONB。** 两处存同一个值必然漂移。列是权威：WebUI 切换启停不该重写整个 JSONB，且 `WHERE enabled` 要能走索引。存储层负责在读写时拆装，是唯一的拆装点。

**`memberships.permissions` 用 PG 原生 `TEXT[]` 而非 JSONB，配 GIN 索引。** 权限检查必须写成 `permissions @> ARRAY['rule:write']`，**不能写 `'rule:write' = ANY(permissions)`**——后者是逐行的数组展开，PostgreSQL 不会把它改写成可索引的形式，GIN 索引对它完全不起作用。20 万行的实测：`= ANY` 走 Parallel Seq Scan 扫完全表，`@>` 走 Bitmap Index Scan。两种写法语义完全相同，所以这是纯粹的写法纪律问题，容易写错且错了不报错，只是慢。

**`accounts.owner_id` 是 `ON DELETE RESTRICT`。** 删掉一个还拥有账号的用户，应该报错而不是留下无主的 Cookie。其余外键用 CASCADE：删绑定就该带走它的规则、冷却组、KV 与禁言名单。

**业务日志归属账号而非绑定。** `activity_logs.account_id` 是 CASCADE，`binding_id` 是 `SET NULL`——删掉某个房间的绑定后，该账号在那个房间的历史记录仍应保留（统计要用），只是不再挂在一个活着的绑定上；删掉账号本身则连带清空。

**`activity_logs` 不存原始报文。** P0 的 `Event.Raw` 是完整的 B 站 JSON，体量是 Payload 的数倍，而排障场景已经由 `magicd dump` 覆盖。不存。

### 3.4 Cookie 明文存储

按既定的威胁模型（WebUI 与本机只被受信任的人操作，可能是单人管理），Cookie 明文入库。

两条必须写进 README 的后果：

1. `pg_dump` 备份文件里是明文 Cookie，等同于账号密码，按密码的标准保管
2. 若 PostgreSQL 不在本机且连接未启用 TLS，Cookie 每次读取都会明文过网络

存储层为此留出唯一收口：`store.Account` 的 Cookie 读写各只有一处，将来要加密只改这两处。P3 不实现加密。

## 4. 规则的单一表示

现在 YAML 的线上格式（`configYAML`、`ruleYAML` 等）是 `internal/rules/config` 的私有类型。P3 之后会有三个通道需要同一份表示：

```
YAML 导入   ──┐
DB 的 JSONB ──┼──→ spec.Rule ──→ rules.Rule（领域模型）
P4 的 API   ──┘
```

若不统一，三处会各自演化出不同的字段名与默认值——这正是 P0 里 `Vars` 那条教训（全项目只允许有一个字段展开处）的同类问题。

**做法**：把线上格式提取到新包 `internal/rules/spec`，类型同时带 `yaml` 与 `json` 标签，转换逻辑（`convertRule`、`convertCondition`、操作符别名归一化、正则预编译校验）一并搬过去。`config` 包退化成薄薄一层：读文件 → `yaml.Unmarshal` 到 `spec.Config` → 调 `spec` 的转换。

`spec.Duration` 需要实现四个方法——`UnmarshalYAML`、`MarshalYAML`、`UnmarshalJSON`、`MarshalJSON`——两边都用 `"1.5s"` 这种人可读的字符串形式。JSONB 里存 `{"window": "3m"}` 而不是纳秒整数，是为了让人能直接看懂库里的行。

### 4.1 运行配置的载入

`config.Binding`（P2 摊平出来的运行单元）在 P3 之后只服务于 `import`。`run` 改从存储层取等价结构：

```go
// store 包
type RunConfig struct {
    AccountName    string
    Cookie         string
    RateLimit      time.Duration
    MaxLength      int
    RoomID         string
    CooldownGroups map[string]time.Duration
    Rules          []rules.Rule
}

func (s *Store) LoadRunConfig(ctx context.Context) ([]RunConfig, error)
```

字段与 `config.Binding` 一一对应，只有 `CookieFile` 换成了 `Cookie` 本身。`run.go` 的装配逻辑因此几乎不动。

## 5. 权限模型

### 5.1 授权单位是绑定

授权挂在「账号-直播间」绑定上，与 P2 的运行单元对齐。这样「李四能改小号B在房间甲的规则，但碰不到房间乙」可以直接表达。

### 5.2 权限点

不设固定角色，直接给权限点：

| 权限点 | 含义 |
|---|---|
| `rule:read` | 查看规则 |
| `rule:write` | 增删改规则、启停规则 |
| `danmaku:send` | 手动发送弹幕 |
| `user:block` | 禁言与解禁，含维护禁言名单 |
| `member:manage` | 授权他人、撤销授权 |
| `event:read` | 查看实时事件流与历史业务日志 |

`users.is_admin` 为真时绕过全部检查。首次运行 `magicd migrate` 时若 `users` 表为空，自动创建管理员并打印一次性随机密码。

### 5.3 预设不进存储层

P4 的 WebUI 会提供「运营」「房管」这类一键按钮，展开成一组权限点。**展开在 UI 层完成，数据库里永远只有权限点。** 这样既不牺牲灵活性，也不让普通用户面对一排勾选框。P3 只在 `internal/perm` 里定义权限点常量与校验，预设留给 P4。

### 5.4 检查入口

```go
// perm 包
type Permission string
const (
    RuleRead      Permission = "rule:read"
    RuleWrite     Permission = "rule:write"
    DanmakuSend   Permission = "danmaku:send"
    UserBlock     Permission = "user:block"
    MemberManage  Permission = "member:manage"
    EventRead     Permission = "event:read"
)

// store 包
func (s *Store) Can(ctx context.Context, userID, bindingID int64, p perm.Permission) (bool, error)
```

`Can` 一次查询解决三条通路，任一成立即放行：管理员、该绑定所属账号的所有者（`member:manage` 除外）、`memberships` 里有行且包含这个权限点。P4 的 API 中间件是它真正的消费者；P3 里由 `magicd can` 调用——一个核心函数搁到下个阶段才第一次被真正用上，中间很容易漂掉。

**所有者那条通路是 P4 Task 8 补的。** 原本只有管理员与 `memberships` 两条，结果是：非管理员用户扫码建了自己的账号、绑了房间之后对自己的绑定寸步难行，而给自己授权又需要他同样没有的 `member:manage`——等于自己把自己锁在门外。这不是收紧安全，是不一致：所有者本来就能删掉整个账号、删掉绑定（连带全部规则、冷却组、KV 与授权）、替换账号的 Cookie，唯独不能把自己的绑定停用。补上的这一项严格弱于他已经握着的那些。

**`member:manage` 不在这条通路里。** 上面那个论证的形式是「严格弱于已有权力」，而所有者已有的权力全是收缩性的——能清空别人的访问，不能凭空赋予一个新人访问。「能删光所有协作者」推不出「能新增一个协作者」。这条例外定义在 `perm.OwnerBypass(p)`，`Can` 与 P4 的 `permissionSet` 都引它，只此一处。

## 6. 两类日志

用户的要求是把「事件记录」和运行日志合并处理，分成系统日志与每账号的业务日志。

| | 去处 | 内容 | 为什么 |
|---|---|---|---|
| **系统日志** | stderr + 滚动文件 | 启动、连接、重连、报错、规则加载 | 数据库连不上时，「数据库连不上」这条日志本身还得写得出来；排障需要 `tail -f` 与 `grep` |
| **业务日志** | `activity_logs` 表 | 收到的事件 + 触发的动作 | P4 要展示、P5 要统计，必须结构化可查 |

### 6.1 系统日志

`log/slog` 的 JSON handler，同时写 stderr 与文件。文件用 `natefinch/lumberjack` 做轮转（按大小切分 + 保留份数），单文件依赖、无传递依赖。长期运行的机器人写 INFO 日志会无限增长，轮转不是可选项。

配置：`MAGICD_LOG_FILE`（留空则只写 stderr）、`MAGICD_LOG_LEVEL`。

### 6.2 业务日志

**「每个账号一份」在库里是 `account_id` 列加索引**——查询视角上每个账号有独立时间线，物理上一张表。

**事件与动作合并进同一张表**（`kind` 区分）。这样一条时间线上能直接看到因果：

```
10:31:02  event   danmaku   张三: 求歌单
10:31:02  action  danmaku   规则「关键词回复」→ 歌单在主播的动态里哦~
```

分成两张表就得靠时间戳自己拼。

**写入必须异步。** 活跃房间每秒几十条事件，同步 INSERT 会把数据库延迟压到规则引擎的关键路径上。做法：

- 带缓冲 channel（容量 4096）
- 后台 goroutine 批量写入：攒够 500 条或满 200ms 就发一次 `COPY`
- **channel 满时丢弃并计数**，每 30 秒向系统日志汇总一条 WARN

丢日志可以接受，漏欢迎不行。这个优先级要写死在代码注释里。

**记录范围**：只记业务事件——弹幕、礼物、连击、上舰、醒目留言、进场、关注、分享、点赞、禁言；排行榜（`ONLINE_RANK_UPDATE`）与房间统计（`ROOM_STATS_UPDATE`）不记，它们每 8 秒一条且没有分析价值。名单写死在代码里，不做成配置项——这十种就是全部有分析价值的事件，为一个没人会改的选项加环境变量不划算。代码里留了替换名单的入口，P4 真需要时再接出来。

**保留期**：`MAGICD_LOG_RETENTION_DAYS`，默认 30。后台每小时清理一次超期行。0 表示不清理。

## 7. 迁移器

自己写，约 80 行，不引入 `goose`/`golang-migrate`：

- `schema_migrations (version INTEGER PRIMARY KEY, applied_at TIMESTAMPTZ)`
- SQL 文件按 `001_xxx.sql`、`002_xxx.sql` 命名，用 `embed.FS` 编进二进制
- 执行前取 **PostgreSQL advisory lock**（`pg_advisory_lock`），防止多实例同时启动并发建表
- 每个文件在单独事务里执行，失败即回滚并报错退出
- **只做前向迁移，不实现回滚**。回滚脚本在实践中几乎从不被执行，却要一直维护；出问题时恢复备份更可靠

`magicd run` 启动时检查 schema 版本，落后则拒绝启动并提示运行 `magicd migrate`——自动迁移在多实例部署下是危险的。

## 8. 包结构

```
internal/perm/                权限点常量与校验（P4 也会 import，不依赖 store）
  perm.go

internal/store/               存储层
  store.go                    Store 结构与连接池
  migrate.go                  迁移器
  migrations/001_init.sql     embed
  user.go                     用户 CRUD 与密码校验
  account.go                  账号 CRUD，Cookie 读写的唯一收口
  binding.go                  绑定与冷却组 CRUD
  rule.go                     规则 CRUD，spec JSONB 与 name/enabled 列的拆装
  membership.go               授权 CRUD 与 Can()
  kv.go                       脚本 storage
  blocklist.go                永久禁言名单
  activity.go                 业务日志的写入、查询与清理
  runconfig.go                LoadRunConfig
  import.go                   YAML 导入的落库逻辑

internal/logging/
  system.go                   slog 装配 + lumberjack 轮转
  activity.go                 业务日志异步批量写入器
  sink.go                     rules.ActivitySink 的实现，按绑定附上归属 ID

internal/rules/spec/          规则的单一序列化表示
  spec.go                     类型定义（yaml + json 标签）
  duration.go                 Duration 的四个 marshal 方法
  convert.go                  spec.Rule → rules.Rule，从 config 包搬来

internal/rules/config/        退化为 YAML 薄加载器
```

`internal/store` 不做接口抽象。只有一个实现（PostgreSQL），提前抽接口是为不存在的第二实现付成本。需要替身的上层测试在自己的测试文件里定义所需的最小接口——Go 的隐式接口让消费者定义接口，这正是它的用法。

## 9. 命令行

```
magicd migrate                              建表/升级；users 为空时创建管理员
magicd user add <用户名> [--admin]           交互式设密码
magicd user passwd <用户名>
magicd user list
magicd account list
magicd login --save <账号名> --owner <用户名>  扫码登录并直接入库
magicd login -o cookie.txt                   保持不变，用于 YAML 路径
magicd binding add <账号名> <房间号>          让账号连接一个直播间
magicd binding list
magicd binding rm|enable|disable <账号名>@<房间号>
magicd grant <用户名> <账号名>@<房间号> <权限点,...>
magicd revoke <用户名> <账号名>@<房间号>
magicd perms <用户名>                         查看某人的授权
magicd can <用户名> <账号名>@<房间号> <权限点>  检查某人有没有某个权限
magicd import -c config.yaml --owner <用户名>  导入 YAML
magicd run                                   从数据库读配置运行
```

`binding` 与 `can` 是写计划时补的。没有 `binding`，扫码登录一个新账号后想加直播间就得先写一份 YAML；没有 `can`，`Can()` 在 P3 里就只有测试没有生产调用者，要搁到 P4 才第一次被真正用上。

数据库连接：`MAGICD_DATABASE_URL` 环境变量，`--db` 标志可覆盖。

`import` 的语义是**幂等 upsert**：按账号名与房间号匹配，已存在则更新，规则按名字匹配。同一份 YAML 反复导入结果一致。Cookie 从 `cookieFile` 读出后写进 `accounts.cookie`。

## 10. YAML 的去留

保留，降级为导入入口。`magicd run` 只读数据库。

理由：P4 的 WebUI 做好之前，没有 YAML 就只能手写 SQL 配规则；而且批量初始化、把配置纳入版本管理这些场景，YAML 一直有用。

明确排除「YAML 优先、回落数据库」的双真相方案——「我在 WebUI 改了为什么没生效」是这种设计的必然产物。

## 11. 测试策略

**存储层测试要求真 PostgreSQL。** 写一份内存实现来避开数据库，等于把整个 SQL 层排除在测试之外，而 SQL 层正是 P3 唯一的新增风险面。

- 读 `MAGICD_TEST_DATABASE_URL`；未设置时 `t.Skip` 并打印如何启动本地库
- 提供 `docker-compose.dev.yml`，一条 `docker compose -f docker-compose.dev.yml up -d` 起一个测试用 PG
- **CI 里必须真跑**：GitHub Actions 加 PostgreSQL service container，否则等于没测
- 每个测试用例在独立 schema 里跑（`CREATE SCHEMA test_xxx` + `search_path`），测完 drop，保证可并行且互不干扰

**不需要数据库的部分**照常：`internal/perm`、`internal/rules/spec` 的转换与序列化、业务日志写入器的批量与丢弃逻辑（用假的 flush 函数）。

**回归保证**：`internal/rules/config` 的现有测试全部保留。`spec` 包提取后，那些测试必须一行不改地继续通过——这是「只是搬家，没有改行为」的证明。

## 12. 本阶段不做

| 项 | 归属 |
|---|---|
| HTTP API、认证中间件、会话/JWT | P4 |
| WebUI 与权限预设的展开 | P4 |
| 统计聚合、报表、盈亏计算 | P5 |
| Cookie 加密 | 按既定威胁模型不做，接口已留好 |
| 迁移回滚 | 见 §7 |
| SQLite 支持 | 已确认只支持 PostgreSQL |

## 13. 已知取舍

**PostgreSQL 是必需依赖。** 需求里的「多平台单二进制分发」在 Go 侧不受影响（`jackc/pgx/v5` 是纯 Go，六平台交叉编译照旧），但「下载即跑」没有了——单人自用也得先有一个 PostgreSQL。Docker 分发用 compose 把 PG 一起带上；裸二进制用户需自行准备。这是明确权衡后的选择。

**业务日志会丢。** 见 §6.2。极端流量下丢弃是设计行为而非缺陷，丢弃量会汇总进系统日志。

**`activity_logs` 会长得很大。** 一个活跃房间一天几万行。默认 30 天保留期加三个索引，单房间量级完全在 PG 的舒适区，但若同时跑几十个房间且把保留期设成 0，需要自行关注磁盘。
