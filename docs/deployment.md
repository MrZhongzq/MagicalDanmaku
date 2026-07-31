# 部署

## 依赖

- PostgreSQL 14 或更高
- 一个 `magicd` 二进制（六平台预编译包见 Releases）

PostgreSQL 是硬依赖。这是 P3 明确权衡后的选择：Go 侧仍是单文件、六平台
交叉编译不受影响，但「下载即跑」没有了——单人自用也需要先有一个数据库。

## 快速开始

```bash
# 1. 起一个 PostgreSQL（已有的话跳过）
docker run -d --name magicd-pg \
  -e POSTGRES_USER=magicd -e POSTGRES_PASSWORD=改成你自己的密码 \
  -e POSTGRES_DB=magicd -p 5432:5432 postgres:16-alpine

# 2. 配置连接串
export MAGICD_DATABASE_URL='postgres://magicd:改成你自己的密码@localhost:5432/magicd?sslmode=disable'

# 3. 建表。空库上会创建管理员并打印一次性密码，记下来
magicd migrate

# 4. 扫码登录一个 B 站账号
magicd login --save 小号 --owner admin

# 5. 让它连接一个直播间
magicd binding add 小号 1706666491

# 6. 启动
magicd run
```

规则的配置目前有两条路：写 YAML 然后 `magicd import`，或者等 P4 的
WebUI。示例见仓库根目录的 `config.example.yaml`。

## 环境变量

| 变量 | 默认 | 说明 |
|---|---|---|
| `MAGICD_DATABASE_URL` | 无 | PostgreSQL 连接串，必填 |
| `MAGICD_LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `MAGICD_LOG_FILE` | 空 | 系统日志文件路径，留空则只写 stderr |
| `MAGICD_LOG_RETENTION_DAYS` | `30` | 业务日志保留天数，0 表示不清理 |

## 两类日志

**系统日志**走 stderr 与可选的滚动文件：启动、连接、重连、报错。
数据库连不上时，「数据库连不上」这条日志本身还得写得出来，所以它不进库。

**业务日志**进数据库的 `activity_logs` 表：收到的事件与机器人执行的动作
在同一条时间线上，可按账号查询。

```sql
-- 小号今天干了什么
SELECT occurred_at, kind, event_type, action_type, rule_name, user_name
FROM activity_logs
WHERE account_id = (SELECT id FROM accounts WHERE name = '小号')
  AND occurred_at > now() - interval '1 day'
ORDER BY occurred_at DESC;
```

排行榜与房间统计事件不入库——它们每 8 秒一条且没有分析价值。

极端流量下业务日志会被主动丢弃，丢弃量汇总进系统日志。**这是设计行为**：
丢日志可以接受，漏欢迎不行。

## 安全须知

**Cookie 以明文存储在 `accounts.cookie` 列里。** 它等同于账号密码。

两条直接后果：

1. `pg_dump` 的备份文件里是明文 Cookie，按密码的标准保管
2. 若 PostgreSQL 不在本机且连接未启用 TLS，Cookie 每次读取都会明文过网络。
   远程数据库请在连接串里加 `sslmode=require`

这是按「WebUI 与本机只被受信任的人操作」的威胁模型做的选择。

## 升级

```bash
magicd migrate   # 换二进制后先跑这个
magicd run
```

`run` 发现 schema 版本落后会**拒绝启动**而非自动迁移——多实例部署下，
让每个实例各自决定何时改表是危险的。

迁移只做前向，不提供回滚脚本。升级前请备份：

```bash
pg_dump -U magicd magicd > magicd-backup.sql
```

## 权限

授权单位是「账号-直播间」绑定，不设固定角色，直接给权限点：

```bash
magicd grant -list                                 # 看有哪些权限点
magicd grant 李四 小号@1706666491 rule:read,rule:write
magicd perms 李四                                   # 看某人有什么权限
magicd revoke 李四 小号@1706666491
```

`magicd grant` 是**替换**而非累加：重新授权的语义是「设定为这些」。

管理员（`magicd user add <名字> --admin`）绕过全部检查。
