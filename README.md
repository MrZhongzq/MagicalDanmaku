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

```bash
export MAGICD_DATABASE_URL='postgres://magicd:magicd@localhost:5432/magicd?sslmode=disable'
magicd migrate                         # 建表，记下打印出的管理员密码
magicd login --save 小号 --owner admin  # 扫码登录一个 B 站账号
magicd binding add 小号 1706666491      # 让它连接一个直播间
magicd run                             # 启动
```

已有 `config.yaml` 的话：`magicd import -c config.yaml --owner admin`。

配置的唯一真相是数据库，`magicd run` 不读 YAML——YAML 只是导入入口。

## 与原项目的差异

- **平台**：只做 B 站；win / macOS / Linux × amd64 / arm64，加 Docker
- **形态**：无头服务端，不依赖窗口；管理界面走 Web
- **收费限制**：全部移除
- **已删除**：`www/` OBS 浏览器源托管、五子棋等 extension、点歌姬与音乐
  播放器、ChatGPT AI 聊天、Qt 桌面 UI、直播间运营动作、语音播报
- **多账号**：是职责分工而非轮换。主播号可以只做统计与房管而不发言，
  小号负责欢迎答谢；某个账号失效时只记录错误日志，不会让别的账号顶替