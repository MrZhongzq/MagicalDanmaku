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

# 2. 配置连接串与管理员初始密码
export MAGICD_DATABASE_URL='postgres://magicd:改成你自己的密码@localhost:5432/magicd?sslmode=disable'
export MAGICD_ADMIN_PASSWORD='改成一个够强的密码，至少 8 位'

# 3. 建表。空库上会用 MAGICD_ADMIN_PASSWORD 创建管理员——
#    不设这个变量的话这一步会直接报错并告诉你怎么办。
magicd migrate

# 4. 扫码登录一个 B 站账号
magicd login --save 小号 --owner admin

# 5. 让它连接一个直播间
magicd binding add 小号 1706666491

# 6. 启动
magicd run
```

规则的配置目前有两条路：写 YAML 然后 `magicd import`，或者用下面的
Web 管理界面。示例见仓库根目录的 `config.example.yaml`。

## Web 管理界面

`magicd run` 默认在 `127.0.0.1:8080` 起 Web 管理界面的后端接口：账号、
规则、禁言名单的增删改查，手动发弹幕/禁言/解禁，以及实时事件流都在
上面。

```bash
export MAGICD_HTTP_ADDR=127.0.0.1:8080     # 默认值，只监听本机
magicd run
# 浏览器打开 http://127.0.0.1:8080
```

用管理员账户（用户名固定 `admin`，密码就是建库时设置的
`MAGICD_ADMIN_PASSWORD`，见上面「快速开始」第 2/3 步）登录。

登录后左侧是七个页面：

- **账号与直播间**：扫码添加/续期 B 站账号，绑定直播间，管理授权
- **房管**：快捷禁言/解禁、维护禁言名单
- **弹幕姬**：进房欢迎、礼物答谢、PK 播报、轮播消息、关注/分享/上舰答谢——
  常用玩法的"傻瓜模式"，翻译成规则引擎能执行的固定规则
- **自定义弹幕姬**：条件树可视化编辑器，给完全自由度的"触发器 + 模板"组合
- **统计**：按场次/按日看弹幕、礼物、上舰等数据
- **日志**：历史业务日志查询 + 实时事件流（SSE）
- **管理**：改自己密码、用户管理、按"账号-直播间"绑定的授权管理

部分控件依赖的后端能力还没做全（比如模板轮询、PK 对面数据、盲盒识别），
这些控件会照常渲染、能编辑，但带一个「待后端支持」的黄色标签，鼠标悬停
会说明具体缺什么、要不要抓包才能补上。控件本身**不会**被禁用或隐藏——
看得到、点得动，只是不产生效果，悬停提示里会写清楚是"存了但引擎不认"
还是"连状态都不会保存、刷新就复位"。完整清单见
`docs/superpowers/specs/2026-08-01-p4-2-悬空清单.md`。

**默认只监听本机**是有意的：管理界面能改 Cookie、发弹幕、禁言，
不该因为忘了配防火墙就暴露到公网。需要远程访问时：

- **推荐**：SSH 端口转发 `ssh -L 8080:127.0.0.1:8080 你的服务器`
- 或者反向代理加 TLS 与访问控制，然后设 `MAGICD_HTTP_ADDR=127.0.0.1:8080`
  让代理去连本机
- Docker 部署**不用手动设** `MAGICD_HTTP_ADDR`——`docker-compose.yml` 已经
  把容器内监听固定成 `0.0.0.0:8080`（容器里绑 127.0.0.1 等于谁都连不上），
  宿主侧默认映射到 `127.0.0.1:20992`。要从别的机器访问就在 `.env` 里设
  `MAGICD_HTTP_BIND=0.0.0.0`，并**务必**同时做访问控制（防火墙只放行受信任
  来源，或者套一层反向代理）

反向代理加了 TLS 时，设 `MAGICD_HTTP_SECURE_COOKIE=1` 让会话 Cookie 带
`Secure` 标志——默认关闭是因为默认监听 127.0.0.1 走的是明文 HTTP，这时
打开 `Secure` 反而会让浏览器根本不发这个 Cookie。

设 `MAGICD_HTTP_ADDR=off` 则完全不起 Web 服务，退化成纯机器人。

### 改完规则不需要重启主程序

「弹幕姬」「自定义弹幕姬」这类有草稿态的页面，改动只进内存草稿，右上角
点一下**「保存并生效」**才会真正写库并让规则引擎拿到新配置——离开页面
或刷新之前忘了点会被弹窗拦一下提醒有未保存的改动。这一步背后依次做了
三件事：写库（`PUT /api/bindings/{binding}/rules`）、触发重载
（`POST /api/bindings/{binding}/reload`）、把草稿标记为已保存基线，
**不需要重启 `magicd run`**，规则立刻在运行中的引擎里生效。

写库成功但重载失败时（比如引擎那一刻正在处理别的事件），界面会单独
弹出一条「已保存到数据库，但重载失败」的持久提示——库已经改了，但引擎
还在用旧配置，这时候重新点一次保存即可（写库这步是幂等的）。直接调
接口的话，`/api/meta/runtime` 会在保存的配置版本与运行中的版本不一致时
报告 `configStale=true`。

加一个全新的账号/绑定**也不需要重启**：P5-1 起，绑定的新增、删除、
启用、停用都在运行期即时生效——WebUI 里加完直播间，机器人会自己建立
连接。（这一条以前是要重启的，文档一度没跟上；真机上验证过：停用 →
日志出现「已移除定时任务」，启用 → 「已配置绑定」+「已连接直播间」，
容器全程没重启。）

## 用 Docker 部署（推荐）

镜像地址是 `ghcr.io/mrzhongzq/magicd`，公开、拉取不需要登录 GHCR。
支持 linux/amd64 与 linux/arm64，多架构 manifest 会自动选对的那个。

**群晖 Container Manager、TrueNAS 这类图形化容器平台**要求镜像先存在于
本地才能创建容器或套用模板，不会替你去拉，所以先手动拉一次：

```bash
docker pull ghcr.io/mrzhongzq/magicd:latest
```

版本 tag **没有 `v` 前缀**（`7.0.0-alpha1`，不是 `v7.0.0-alpha1`）——
GitHub 上的 Release tag 带 `v`，镜像 tag 不带，这两者不一致是 GoReleaser
的默认行为，照抄 Release 名字会拉不到。要钉死版本就在 `.env` 里设
`MAGICD_TAG=7.0.0-alpha1`。

```bash
git clone https://github.com/MrZhongzq/MagicalDanmaku.git
cd MagicalDanmaku
cp .env.example .env
# 编辑 .env，改掉 POSTGRES_PASSWORD 与 MAGICD_ADMIN_PASSWORD
docker compose up -d
```

管理员账号密码就是 `.env` 里设的 `MAGICD_ADMIN_PASSWORD`——不用再翻
`docker compose logs migrate` 找一次性密码了。忘了在 `.env` 里设置的话，
`docker compose up -d` 这一步会直接报错并说明怎么补。

然后扫码登录一个 B 站账号并加直播间：

```bash
docker compose run --rm magicd login --save 小号 --owner admin
docker compose run --rm magicd binding add 小号 1706666491
docker compose restart magicd
```

注意用的是 `docker compose run --rm`，不是 `exec`——扫码需要一个能显示
二维码的交互式终端，而 `magicd` 服务是后台跑的。

### 常用操作

```bash
docker compose logs -f magicd          # 看实时日志
docker compose ps                      # 看服务状态
docker compose restart magicd          # 加了新账号/绑定、或改了 .env 后重启生效
                                        # （规则改动走网页热重载，不需要重启）
docker compose down                    # 停止（保留数据）
docker compose pull; docker compose up -d   # 升级到新版镜像
```

### 升级

镜像更新后，schema 可能也需要升级：

```bash
docker compose pull
docker compose up -d          # migrate service 会自动跑一次
```

`magicd` 依赖 `migrate` 成功完成才启动，所以顺序是自动保证的。
升级前建议备份：

```bash
docker compose exec -T postgres pg_dump -U magicd magicd > backup.sql
```

### ⚠️ 关于 `docker compose down -v`

`-v` 会**连数据卷一起删掉**——账号、规则、日志全没。测试环境用它清干净
没问题，生产环境不要加这个参数。

### 时区

`TZ` 默认 `Asia/Shanghai`。**定时规则按本地时区计算**，时区设错了规则会
整体偏移几个小时。在 `.env` 里按你所在时区设置。

## 环境变量

| 变量 | 默认 | 说明 |
|---|---|---|
| `MAGICD_DATABASE_URL` | 无 | PostgreSQL 连接串，必填 |
| `MAGICD_ADMIN_PASSWORD` | 无 | 空库首次 `migrate` 建管理员时用的密码，必填（至少 8 位）；库里已有管理员后不生效，改密码用 `magicd user passwd` |
| `MAGICD_LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `MAGICD_LOG_FILE` | 空 | 系统日志文件路径，留空则只写 stderr |
| `MAGICD_LOG_RETENTION_DAYS` | `30` | 业务日志保留天数，0 表示不清理 |
| `MAGICD_HTTP_ADDR` | `127.0.0.1:8080` | Web 管理界面监听地址；只监听本机。设为 `0.0.0.0:8080` 对外监听（Docker 部署需要，务必自行做访问控制），设为空串或 `off` 则不起 Web 服务 |
| `MAGICD_HTTP_SECURE_COOKIE` | 关闭 | 设为 `1` 时会话 Cookie 带 `Secure` 标志，仅当反向代理已加 TLS 时才打开 |

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

Docker 部署下补充两点：

- **`.env` 里有数据库密码**，它已在 `.gitignore` 里，但别手动提交、别放进
  任何共享目录
- **默认不暴露数据库端口**。`docker-compose.yml` 里那行 `ports` 是注释掉的，
  只有需要从宿主直连调试时才取消注释，且已经限定在 `127.0.0.1`

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
