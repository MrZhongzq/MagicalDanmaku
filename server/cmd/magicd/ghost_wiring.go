package main

import (
	"log/slog"
	"os"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/ghost"
)

// defaultGhostWindow 是等待自己那条弹幕回显的默认窗口。
//
// 真机实测回显延迟稳定在 0.2~1.5 秒，10 秒约为上界的 7 倍，留给网络
// 抖动和瞬时重连足够余量。
const defaultGhostWindow = 10 * time.Second

// newGhostDetector 按环境变量装配幽灵弹幕检测器，返回 nil 表示关闭检测。
//
// 判定结果只打日志、不落库：落库要先补上动作日志的两个缺口（既没记
// 实际发出的文本，也没记发送结果），那是另一件事。
func newGhostDetector(log *slog.Logger) *ghost.Detector {
	if v := os.Getenv("MAGICD_GHOST_DETECT"); v == "0" || v == "off" {
		log.Info("幽灵弹幕检测已关闭")
		return nil
	}

	window := defaultGhostWindow
	if v := os.Getenv("MAGICD_GHOST_WINDOW"); v != "" {
		d, err := time.ParseDuration(v)
		switch {
		case err != nil:
			log.Warn("MAGICD_GHOST_WINDOW 不是合法时长，改用默认值",
				"value", v, "default", defaultGhostWindow, "err", err)
		case d <= 0:
			log.Warn("MAGICD_GHOST_WINDOW 必须为正，改用默认值",
				"value", v, "default", defaultGhostWindow)
		default:
			window = d
		}
	}

	d := ghost.New(window)
	d.OnGhost = func(r ghost.Report) {
		log.Warn("疑似幽灵弹幕：发送接口返回成功，但直播间广播里收不到自己这条",
			"room", r.RoomID,
			"uid", r.UID,
			"text", r.Text,
			"waited", r.Waited.Round(time.Millisecond))
	}
	d.OnMismatch = func(m ghost.Mismatch) {
		log.Warn("弹幕回显与发送内容不一致，疑似被关键词规则改写",
			"room", m.RoomID,
			"uid", m.UID,
			"sent", m.Sent,
			"echo", m.Echo)
	}
	log.Info("幽灵弹幕检测已启用", "window", window)
	return d
}
