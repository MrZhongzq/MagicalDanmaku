package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/cmdmap"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

// dumper 把选定 CMD 的原始 JSON 落盘，用于采集真实样本。
//
// B 站会不定期新增或改动 CMD，此工具让使用者能在几分钟内抓到真实
// 载荷，据此补写映射与黄金样本，无需依赖抓包工具。
type dumper struct {
	f           *os.File
	w           *bufio.Writer
	all         bool
	onlyUnknown bool
	want        map[string]bool
	count       int
}

// newDumper 创建一个抓包器。spec 为空表示不抓，返回的实例可安全调用。
//
// spec 支持三种写法：
//   - all      抓全部消息
//   - unknown  只抓未被识别的消息，即需要补写映射的那些
//   - 逗号分隔的 CMD 名，如 ONLINE_RANK_V3,DM_INTERACTION
func newDumper(spec, path string) (*dumper, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return &dumper{}, nil
	}

	d := &dumper{want: make(map[string]bool)}
	switch {
	case strings.EqualFold(spec, "all"):
		d.all = true
	case strings.EqualFold(spec, "unknown"):
		d.onlyUnknown = true
	default:
		for _, part := range strings.Split(spec, ",") {
			if part = strings.TrimSpace(part); part != "" {
				d.want[strings.ToUpper(part)] = true
			}
		}
		if len(d.want) == 0 {
			return &dumper{}, nil
		}
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("打开抓包文件 %s 失败: %w", path, err)
	}
	d.f = f
	d.w = bufio.NewWriter(f)

	switch {
	case d.all:
		fmt.Printf("抓包已开启：全部消息 → %s\n", path)
	case d.onlyUnknown:
		fmt.Printf("抓包已开启：仅未识别消息 → %s\n", path)
	default:
		fmt.Printf("抓包已开启：%s → %s\n", spec, path)
	}
	return d, nil
}

// Write 按需记录一个事件的原始 JSON。
func (d *dumper) Write(ev event.Event) {
	if d == nil || d.w == nil {
		return
	}
	switch {
	case d.all:
		// 全抓
	case d.onlyUnknown:
		if ev.Type != event.TypeUnknown {
			return
		}
	case !d.want[cmdmap.CommandOf(ev.Raw)]:
		return
	}
	// Raw 本身就是一行合法 JSON，直接写出即可构成 JSON Lines。
	if !json.Valid(ev.Raw) {
		return
	}
	d.w.Write(ev.Raw)
	d.w.WriteByte('\n')
	d.count++
}

// Close 刷盘并关闭文件。
func (d *dumper) Close() {
	if d == nil || d.w == nil {
		return
	}
	d.w.Flush()
	d.f.Close()
	fmt.Printf("\n已抓取 %d 条原始消息\n", d.count)
}
