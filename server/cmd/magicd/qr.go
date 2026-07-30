package main

import (
	"fmt"
	"strings"

	qrcode "github.com/skip2/go-qrcode"
)

// 终端渲染用的半块字符。一个字符纵向表示两行像素，
// 使二维码在字符网格中接近正方形，扫码识别率更高。
const (
	blockBoth  = '█' // █ 上下都是深色
	blockUpper = '▀' // ▀ 仅上半深色
	blockLower = '▄' // ▄ 仅下半深色
	blockNone  = ' ' // 上下都是浅色
)

// quietZone 是二维码四周必需的空白边距，单位为模块数。
// QR 规范要求至少 4 个模块，少于此值部分扫码器无法识别；
// 代价是终端里多占 8 列宽度，实测仍能塞进标准窗口。
const quietZone = 4

// renderQR 把内容编码为可在终端显示的二维码字符画。
//
// 采用半块字符而非两个空格：终端字符高宽比约 2:1，用半块可以让
// 二维码保持接近正方形，同时行数减半，能塞进标准终端窗口。
//
// 颜色约定为「深色模块 = 前景」，配合浅色背景终端。多数终端默认
// 深色背景，因此这里输出的是反相图案——实测手机扫码器对两种极性
// 都能识别。
func renderQR(content string) (string, error) {
	q, err := qrcode.New(content, qrcode.Medium)
	if err != nil {
		return "", fmt.Errorf("生成二维码失败: %w", err)
	}
	q.DisableBorder = true

	grid := q.Bitmap()
	if len(grid) == 0 {
		return "", fmt.Errorf("二维码位图为空")
	}
	size := len(grid)

	// isDark 带静区偏移取模块值，越界视为浅色。
	isDark := func(row, col int) bool {
		r, c := row-quietZone, col-quietZone
		if r < 0 || r >= size || c < 0 || c >= size {
			return false
		}
		return grid[r][c]
	}

	total := size + quietZone*2
	var b strings.Builder
	// 每次处理两行像素，压成一行字符。
	for row := 0; row < total; row += 2 {
		for col := 0; col < total; col++ {
			upper := isDark(row, col)
			lower := isDark(row+1, col)
			switch {
			case upper && lower:
				b.WriteRune(blockBoth)
			case upper:
				b.WriteRune(blockUpper)
			case lower:
				b.WriteRune(blockLower)
			default:
				b.WriteRune(blockNone)
			}
		}
		b.WriteByte('\n')
	}
	return b.String(), nil
}
