# [神奇弹幕] 项目状态公告

**本仓库及所有相关项目已永久停止开发、维护和任何形式的分发。**

## 重要声明
1.  **停止运营**：自 `2025年12月25日` 起，本项目已完全终止。项目代码**仅供历史存档查阅**，不再提供任何功能支持。
2.  **法律原因**：为尊重 `哔哩哔哩（Bilibili）` 平台的知识产权与相关法律规定，避免潜在的法律风险，开发者已主动、永久地停止了本项目。
3.  **用户须知**：请勿再使用、分发或基于本项目的任何代码进行开发。请支持并使用 `哔哩哔哩` 官方客户端与服务。

## 致歉与感谢
我们对由此给用户带来的不便深表歉意。同时，也衷心感谢所有曾经关注、Star 或 Fork 本项目的朋友。

---
*本仓库现已被 GitHub 归档，处于只读状态。*

---

# 本分支：Go 重写版

原项目的 Qt/C++ 桌面程序被完整重写为 Go 编写的无头服务端，前后端分离，
移除全部收费限制。

## 快速开始

需要 PostgreSQL 14+。完整部署说明见 [docs/deployment.md](docs/deployment.md)。

最省事的是 Docker（自带 PostgreSQL）：

```bash
cp .env.example .env    # 改掉 POSTGRES_PASSWORD
docker compose up -d
docker compose logs migrate    # 取管理员的一次性密码
```

不用 Docker 的话，需要自己准备 PostgreSQL 14+：

```bash
export MAGICD_DATABASE_URL='postgres://magicd:magicd@localhost:5432/magicd?sslmode=disable'
magicd migrate                         # 建表，记下打印出的管理员密码
magicd login --save 小号 --owner admin  # 扫码登录一个 B 站账号
magicd binding add 小号 1706666491      # 让它连接一个直播间
magicd run                             # 启动
```

已有 `config.yaml` 的话：`magicd import -c config.yaml --owner admin`。

配置的唯一真相是数据库，`magicd run` 不读 YAML——YAML 只是导入入口。

## 开发：跑测试

```bash
cd server
export MAGICD_TEST_DATABASE_URL='postgres://magicd:magicd@localhost:5432/magicd?sslmode=disable'
go test ./... -count=1
```

**`MAGICD_TEST_DATABASE_URL` 不设的话，存储层测试会整包 skip 而退出码仍是 0**
——看起来全绿，实际什么都没跑。`-count=1` 同理：不加的话拿到的可能是缓存的
旧结果，不是这次改动的证据。

竞态检测（本仓库有两条并发的 WebSocket 连接与若干共享状态，改动这些地方时
务必跑）：

```bash
CGO_ENABLED=1 go test ./... -race -count=1
```

`-race` **需要 CGO 和一个 C 编译器**，而本项目为了六平台交叉编译默认
`CGO_ENABLED=0`，所以必须像上面这样显式打开。Windows 上装编译器：

```powershell
winget install -e --id MSYS2.MSYS2
C:\msys64\usr\bin\pacman.exe -S --noconfirm --needed mingw-w64-x86_64-gcc
$env:PATH = "C:\msys64\mingw64\bin;$env:PATH"
```

## 与原项目的差异

- **平台**：只做 B 站；win / macOS / Linux × amd64 / arm64 六个目标的静态
  二进制，外加 linux/amd64 与 linux/arm64 的 Docker 镜像
- **形态**：无头服务端，不依赖窗口；管理界面走 Web
- **收费限制**：全部移除
- **已删除**：`www/` OBS 浏览器源托管、五子棋等 extension、点歌姬与音乐
  播放器、ChatGPT AI 聊天、Qt 桌面 UI、直播间运营动作、语音播报
- **多账号**：是职责分工而非轮换。主播号可以只做统计与房管而不发言，
  小号负责欢迎答谢；某个账号失效时只记录错误日志，不会让别的账号顶替