# magicd — 神奇弹幕服务端（Go 重写版）

原 Qt/C++ 项目 [Bilibili-MagicalDanmaku](https://github.com/iwxyi/Bilibili-MagicalDanmaku)
的 Go 重写，目标是前后端分离、多平台分发、全功能免费。

当前进度：**P0 协议内核已完成**，可把 B 站直播间变成一条归一化事件流。

## 快速开始

```bash
# 扫码登录，Cookie 写入 cookie.txt
magicd login -o cookie.txt

# 连接直播间，打印实时事件
magicd probe -room 21452505 -cookie-file cookie.txt

# 只看弹幕和礼物
magicd probe -room 21452505 -cookie-file cookie.txt -type danmaku,gift

# 抓取未识别的消息，用于补写映射
magicd probe -room 21452505 -cookie-file cookie.txt -dump unknown
```

## 从源码构建

需要 Go 1.24 以上。

```bash
cd server
go build -o magicd ./cmd/magicd
```

出全平台包（零外部依赖，只用 Go 工具链）：

```bash
scripts/build.sh              # 六个平台
scripts/build.sh linux/amd64  # 指定单个平台
```

产物落在仓库根的 `dist/`，附 `checksums.txt`。

## 开发

```bash
cd server
go test ./...           # 测试
go test ./... -race     # 竞态检测，需要 cgo 与 gcc
go vet ./...            # 静态检查
gofmt -l .              # 格式检查，应无输出
```

Windows 上跑竞态检测需要 MinGW-w64。可用 winget 安装：

```powershell
winget install --id BrechtSanders.WinLibs.POSIX.UCRT
```

装完需重开终端让 PATH 生效，然后设 `$env:CGO_ENABLED='1'`。

## 发版

打 tag 后由 GitHub Actions 自动出包并发布到 Releases：

```bash
git tag -a v7.0.0 -m "..."
git push origin v7.0.0
```

本地预演（不发布）：

```bash
goreleaser release --snapshot --clean
```

`scripts/build.sh` 与 `.goreleaser.yaml` 的目标平台和 ldflags 保持一致，
两者产物等价；前者用于日常验证，后者用于正式发版。

## 目录结构

```
server/
├── cmd/magicd/              CLI 入口
├── internal/
│   ├── buildinfo/           编译期注入的版本信息
│   ├── event/               归一化事件模型（18 种事件类型）
│   ├── ratelimit/           发送限流机制
│   └── connector/
│       ├── connector.go     Connector / Actions 接口
│       └── bilibili/
│           ├── auth/        扫码登录、Cookie、wbi 签名
│           ├── wire/        二进制包编解码、zlib/brotli 解压
│           ├── cmdmap/      CMD → 事件映射，一 CMD 一文件
│           ├── api/         HTTP 接口封装
│           ├── client.go    连接状态机
│           └── action.go    发弹幕、禁言
└── testdata/cmds/           黄金样本，用于回归测试
```

新增一个 CMD 只需在 `cmdmap/` 加一个文件并在 `init()` 里注册，
再往 `testdata/cmds/` 放一份样本即可自动纳入回归。

## 路线图

| 阶段 | 内容 | 状态 |
|---|---|---|
| P0 | 协议内核 | 已完成 |
| P1 | 分发流水线 | 进行中 |
| P2 | 规则引擎（goja 脚本） | 待开始 |
| P3 | 多租户数据层 | 待开始 |
| P4 | API + WebUI | 待开始 |
| P5 | 数据看板 | 待开始 |
| P6 | PK 大乱斗 | 待开始 |

设计与实施计划见 `docs/superpowers/`。
