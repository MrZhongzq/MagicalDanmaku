package wire

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"

	"github.com/andybalholm/brotli"
)

// maxDecompressed 是解压后允许的最大字节数，用于防御压缩炸弹。
const maxDecompressed = 32 << 20 // 32 MiB

// Frame 是一个已解压、可直接消费的业务包。
type Frame struct {
	Operation uint32 // 操作码
	ProtoVer  uint16 // 原始协议版本
	Body      []byte // 包体，ProtoJSON 时为 JSON 明文
}

// Split 解析一个 WebSocket 二进制帧，按需解压，并切分出其中全部业务包。
//
// protover 2/3 的包体解压后是多个完整包串联，必须按每个包自身的
// TotalSize 循环切分，而不能假设只有一个包。
func Split(raw []byte) ([]Frame, error) {
	h, err := DecodeHeader(raw)
	if err != nil {
		return nil, err
	}
	if int(h.TotalSize) > len(raw) {
		return nil, fmt.Errorf("wire: 包头声明长度 %d 超过实际数据长度 %d", h.TotalSize, len(raw))
	}
	if h.TotalSize < HeaderSize {
		return nil, fmt.Errorf("wire: 非法的包长度 %d", h.TotalSize)
	}

	body := raw[HeaderSize:h.TotalSize]

	switch h.ProtoVer {
	case ProtoZlib:
		unc, err := decompressZlib(body)
		if err != nil {
			return nil, err
		}
		return splitConcatenated(unc)
	case ProtoBrotli:
		unc, err := decompressBrotli(body)
		if err != nil {
			return nil, err
		}
		return splitConcatenated(unc)
	default:
		return []Frame{{Operation: h.Operation, ProtoVer: h.ProtoVer, Body: body}}, nil
	}
}

// splitConcatenated 按包头声明的长度循环切分串联的多个包。
func splitConcatenated(buf []byte) ([]Frame, error) {
	var frames []Frame
	for offset := 0; offset+HeaderSize <= len(buf); {
		h, err := DecodeHeader(buf[offset:])
		if err != nil {
			return nil, err
		}
		if h.TotalSize < HeaderSize {
			return nil, fmt.Errorf("wire: 偏移 %d 处包长度非法: %d", offset, h.TotalSize)
		}
		end := offset + int(h.TotalSize)
		if end > len(buf) {
			return nil, fmt.Errorf("wire: 偏移 %d 处包越界，声明 %d 但仅剩 %d", offset, h.TotalSize, len(buf)-offset)
		}
		frames = append(frames, Frame{
			Operation: h.Operation,
			ProtoVer:  h.ProtoVer,
			Body:      buf[offset+HeaderSize : end],
		})
		offset = end
	}
	return frames, nil
}

func decompressZlib(b []byte) ([]byte, error) {
	zr, err := zlib.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("wire: zlib 初始化失败: %w", err)
	}
	defer zr.Close()
	out, err := io.ReadAll(io.LimitReader(zr, maxDecompressed))
	if err != nil {
		return nil, fmt.Errorf("wire: zlib 解压失败: %w", err)
	}
	return out, nil
}

func decompressBrotli(b []byte) ([]byte, error) {
	out, err := io.ReadAll(io.LimitReader(brotli.NewReader(bytes.NewReader(b)), maxDecompressed))
	if err != nil {
		return nil, fmt.Errorf("wire: brotli 解压失败: %w", err)
	}
	return out, nil
}
