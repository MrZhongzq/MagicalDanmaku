# P3 收尾说明与遗留项

P3 多租户数据层已完成并通过全分支终审。本文记录**终审后仍然存在的东西**，
以及 P4 会继承的约束——这些内容原本在 SDD 工作区的 ledger 里，工作区已删除，
故固化到这里。

## 交付内容

19 个任务，34 个提交（含终审修复波次的 6 个）。

- `internal/perm` —— 七个权限点
- `internal/rules/spec` —— 规则的唯一序列化表示（YAML / JSONB / 未来的 API 共用）
- `internal/store` —— PostgreSQL 存储层：用户、账号、绑定、规则、授权、脚本 KV、
  禁言名单、业务日志、运行配置载入、YAML 导入
- `internal/logging` —— 系统日志（stderr + 滚动文件）与业务日志异步写入器
- `magicd` 的十个子命令：`migrate` / `user` / `account` / `binding` / `grant` /
  `revoke` / `perms` / `can` / `import` / `run`

`magicd run` 不再读 `config.yaml`，数据库是配置的唯一真相。

## 终审确认成立的设计不变量

| 不变量 | 状态 |
|---|---|
| Cookie 读写各收在一处（`accountColumns`+`scanAccount` 读、`encodeCookie` 写） | 成立 |
| 规则的 `name`/`enabled` 是列，JSONB 里不含这两个字段 | 成立 |
| 授权单位是绑定；管理员绕过；无授权记录一律拒绝 | 成立 |
| 业务日志写入永不阻塞事件处理 | 成立 |
| 业务日志丢弃要计数 | 终审时只覆盖两条路径，已在修复波次补全第三条（落库失败） |
| 系统日志走文件、业务日志进库 | 成立 |
| 只有一个迁移文件、前向、无回滚 | 成立 |

## Park 的遗留项（终审后仍在，均为 Low）

按流程不做第二轮修复波次，记录在此供后续处理。

1. **`bindingStub.Block` 与 `SendDanmaku` 不对称**（`cmd/magicd/run_test.go`）。
   `SendDanmaku` 的桩会检查 `ctx.Err()`，`Block` 不会。如果有人把 `roomBot.Block`
   里的 `ctx.Load()` 改回捕获旧值，没有测试会抓到。修法：给 `Block` 加同样两行。

2. **关停期的 5 秒补发窗口内 Ctrl+C 被吞掉且无日志**（`cmd/magicd/run.go`）。
   `signal.NotifyContext` 的 `stop()` 注册在 closeAll 之前，LIFO 下它在 closeAll
   **之后**才执行。于是补发的这最多 5 秒里，第二次 Ctrl+C 无法强退，且没有任何
   输出（`正在退出...` 早已打完），观感是卡死。**修法只需在 closeAll 前加一行
   「正在补发未决弹幕（最多 5 秒）」的日志。** 这是四条里最值得先做的。

3. **早返回路径上 ctx 切换的方向是反的**（`cmd/magicd/run.go`）。
   注释声称切换「永远朝更宽松方向」，但早返回时那些 goroutine 原本持有的是未取消的
   主 ctx，切换后拿到的 `shutdownCtx` 会在 closeAll 返回时立刻被 cancel。因为此时
   进程正要带错误退出，实际无影响；但注释比代码的实际保证强。

4. **CI 守卫对同前缀改名仍会静默通过**（`.github/workflows/ci.yml`）。
   已从「断言没有 SKIP」改成「断言有 `--- PASS: TestMigrateCreatesAllTables`」，
   堵住了任意改名的漏洞；但 `TestMigrateCreatesAllTablesAndIndexes` 这种同前缀改名
   仍会匹配。修法：`-run '^TestMigrateCreatesAllTables$'` 或 grep 带尾随空格。

## P4 必须知道的四件事

1. **`Store` 没有对外的事务承载能力。** `pgx.BeginFunc` 只在 `SetCooldownGroups` 与
   `ReplaceRules` 内部使用。P4 的写接口一旦需要跨方法原子性（例如「新建绑定并同时
   授权」），第一件事就是给 `Store` 加一层 tx 承载。

2. **`Can` 的管理员分支不校验绑定是否存在。** `Can(adminID, 999999, p)` 返回 true。
   P3 的 CLI 因为先查过绑定所以无碍；**P4 的中间件若直接拿 URL 里的 ID 调 `Can`，
   必须自己先确认对象存在**，否则管理员能对不存在的资源"通过"鉴权。

3. **只有 `VerifyPassword` 是防用户名枚举的边界。** `GetUserByName` 与 `SetPassword`
   的错误信息里带用户名。P3 只在 CLI 里用它们，正确；**P4 一旦把这两个接到未认证
   路径上，就成了用户名枚举器**。登录与找回密码路径只能走 `VerifyPassword`。

4. **`AccountInput` 的 `RateLimit`/`MaxLength` 是二态而非三态。** 零值即默认，
   所以「YAML 里没写」和「写了 0」不可区分。后果：在库里用 API 调过限流的账号，
   导一次 YAML 就会被静默改回 1500ms。修法需要 `*int` 之类的三态表示。

## 已知的跨阶段缺口

**P1 的 Docker 分发从未实现，但 P1 被标记为完成。** 核实：`.goreleaser.yaml` 无
`dockers:` 段、仓库无任何 Dockerfile、release workflow 无 docker 步骤。而用户的原始
需求明确列了 docker。

需要一个 P1 补丁。注意 **P3 把 PostgreSQL 变成了硬依赖**，所以这个补丁的范围比原计划
大：除了 Dockerfile 与 goreleaser 的 `dockers:` 段，还要配一份把 PostgreSQL 一起带上的
compose。`docker-compose.dev.yml` 可以当模板，但它用 5433 端口且密码是 `magicd/magicd`，
生产版本不能照抄。

## 其他值得记一笔的

- **`ActivityWriter.Dropped()` 现在混合了两类丢弃**（缓冲溢出 / 落库失败），
  `Close()` 的汇总日志不区分。运维排障时「丢了 300 条」到底是压力大还是数据库拒收，
  含义完全不同。P5 做日志面板时建议拆成两个计数器。
- **goja 沙箱的 200ms 硬超时现在有一个例外**：`storage.get`/`storage.set` 的数据库
  调用有 3 秒上限（修复了原先的无界阻塞），慢数据库下单次脚本执行可能达到 3 秒。
  比修复前的「永不返回」严格更好，但沙箱文档承诺的 200ms 不再是绝对上界。
- **`wg.Wait()` 本身仍无界**：连接 goroutine 若卡在非 ctx 敏感的位置，Ctrl+C 依旧
  等不到头。P3 只封住了脚本存储这一条路径。若要给退出一个真正的总上限，
  需要给 `wg.Wait()` 加 select + 超时。
- **`activity_logs.room_id` 存的是解析后的真实房间号，`bindings.room_id` 存的是
  用户输入**（可能是短号）。今天不炸（查询接口没有 room_id 过滤器），
  P4 若按 room_id 关联要注意。
