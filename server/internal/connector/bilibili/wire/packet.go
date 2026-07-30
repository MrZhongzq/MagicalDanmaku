// Package wire 实现 B 站直播 WebSocket 的二进制包编解码。
package wire

import (
	"encoding/binary"
	"errors"
)

// HeaderSize 是包头固定长度。
const HeaderSize = 16

// 操作码。
const (
	OpHeartbeat      uint32 = 2 // 客户端心跳
	OpHeartbeatReply uint32 = 3 // 服务端心跳回复
	OpMessage        uint32 = 5 // 服务端业务消息
	OpAuth           uint32 = 7 // 客户端认证
	OpAuthReply      uint32 = 8 // 服务端认证回复
)

// 协议版本，决定 body 的编码方式。
const (
	ProtoJSON      uint16 = 0 // JSON 明文
	ProtoHeartbeat uint16 = 1 // 心跳回复，body 为 4 字节人气值
	ProtoZlib      uint16 = 2 // zlib 压缩
	ProtoBrotli    uint16 = 3 // brotli 压缩
)

// ErrShortPacket 表示数据长度不足以构成一个完整包头。
var ErrShortPacket = errors.New("wire: 数据长度不足 16 字节，无法解析包头")

// Header 是 B 站直播协议的包头。
type Header struct {
	TotalSize  uint32 // 含包头的总长度
	HeaderSize uint16 // 包头长度，恒为 16
	ProtoVer   uint16 // 协议版本
	Operation  uint32 // 操作码
	Sequence   uint32 // 序号，恒为 1
}

// Encode 构造一个待发送的数据包。
// 客户端发出的包一律使用 ProtoJSON 与 Sequence=1。
func Encode(op uint32, body []byte) []byte {
	buf := make([]byte, HeaderSize+len(body))
	binary.BigEndian.PutUint32(buf[0:4], uint32(HeaderSize+len(body)))
	binary.BigEndian.PutUint16(buf[4:6], HeaderSize)
	binary.BigEndian.PutUint16(buf[6:8], ProtoJSON)
	binary.BigEndian.PutUint32(buf[8:12], op)
	binary.BigEndian.PutUint32(buf[12:16], 1)
	copy(buf[HeaderSize:], body)
	return buf
}

// DecodeHeader 解析包头。所有字段均为大端序。
func DecodeHeader(b []byte) (Header, error) {
	if len(b) < HeaderSize {
		return Header{}, ErrShortPacket
	}
	return Header{
		TotalSize:  binary.BigEndian.Uint32(b[0:4]),
		HeaderSize: binary.BigEndian.Uint16(b[4:6]),
		ProtoVer:   binary.BigEndian.Uint16(b[6:8]),
		Operation:  binary.BigEndian.Uint32(b[8:12]),
		Sequence:   binary.BigEndian.Uint32(b[12:16]),
	}, nil
}
