package wire

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"testing"

	"github.com/andybalholm/brotli"
)

// buildPacket 构造一个指定协议版本与操作码的完整包。
func buildPacket(protoVer uint16, op uint32, body []byte) []byte {
	buf := make([]byte, HeaderSize+len(body))
	binary.BigEndian.PutUint32(buf[0:4], uint32(HeaderSize+len(body)))
	binary.BigEndian.PutUint16(buf[4:6], HeaderSize)
	binary.BigEndian.PutUint16(buf[6:8], protoVer)
	binary.BigEndian.PutUint32(buf[8:12], op)
	binary.BigEndian.PutUint32(buf[12:16], 1)
	copy(buf[HeaderSize:], body)
	return buf
}

func TestSplitPlainJSON(t *testing.T) {
	body := []byte(`{"cmd":"LIVE"}`)
	frames, err := Split(buildPacket(ProtoJSON, OpMessage, body))
	if err != nil {
		t.Fatalf("Split 失败: %v", err)
	}
	if len(frames) != 1 {
		t.Fatalf("帧数 = %d, 期望 1", len(frames))
	}
	if !bytes.Equal(frames[0].Body, body) {
		t.Errorf("Body = %q, 期望 %q", frames[0].Body, body)
	}
	if frames[0].Operation != OpMessage {
		t.Errorf("Operation = %d, 期望 %d", frames[0].Operation, OpMessage)
	}
}

func TestSplitZlibMultiPacket(t *testing.T) {
	inner := append(
		buildPacket(ProtoJSON, OpMessage, []byte(`{"cmd":"A"}`)),
		buildPacket(ProtoJSON, OpMessage, []byte(`{"cmd":"BB"}`))...,
	)

	var compressed bytes.Buffer
	zw := zlib.NewWriter(&compressed)
	if _, err := zw.Write(inner); err != nil {
		t.Fatalf("zlib 写入失败: %v", err)
	}
	zw.Close()

	frames, err := Split(buildPacket(ProtoZlib, OpMessage, compressed.Bytes()))
	if err != nil {
		t.Fatalf("Split 失败: %v", err)
	}
	if len(frames) != 2 {
		t.Fatalf("帧数 = %d, 期望 2", len(frames))
	}
	if string(frames[0].Body) != `{"cmd":"A"}` {
		t.Errorf("第一帧 = %q", frames[0].Body)
	}
	if string(frames[1].Body) != `{"cmd":"BB"}` {
		t.Errorf("第二帧 = %q", frames[1].Body)
	}
}

func TestSplitBrotliMultiPacket(t *testing.T) {
	inner := append(
		buildPacket(ProtoJSON, OpMessage, []byte(`{"cmd":"X"}`)),
		buildPacket(ProtoJSON, OpMessage, []byte(`{"cmd":"YY"}`))...,
	)

	var compressed bytes.Buffer
	bw := brotli.NewWriter(&compressed)
	if _, err := bw.Write(inner); err != nil {
		t.Fatalf("brotli 写入失败: %v", err)
	}
	bw.Close()

	frames, err := Split(buildPacket(ProtoBrotli, OpMessage, compressed.Bytes()))
	if err != nil {
		t.Fatalf("Split 失败: %v", err)
	}
	if len(frames) != 2 {
		t.Fatalf("帧数 = %d, 期望 2", len(frames))
	}
	if string(frames[1].Body) != `{"cmd":"YY"}` {
		t.Errorf("第二帧 = %q", frames[1].Body)
	}
}

func TestSplitHeartbeatReply(t *testing.T) {
	body := []byte{0x00, 0x00, 0x04, 0xD2} // 人气值 1234
	frames, err := Split(buildPacket(ProtoHeartbeat, OpHeartbeatReply, body))
	if err != nil {
		t.Fatalf("Split 失败: %v", err)
	}
	if len(frames) != 1 {
		t.Fatalf("帧数 = %d, 期望 1", len(frames))
	}
	if frames[0].Operation != OpHeartbeatReply {
		t.Errorf("Operation = %d, 期望 %d", frames[0].Operation, OpHeartbeatReply)
	}
}

func TestSplitRejectsTruncated(t *testing.T) {
	if _, err := Split([]byte{0x00, 0x01}); err == nil {
		t.Error("截断输入应当报错")
	}
}

func TestSplitStopsOnBogusTotalSize(t *testing.T) {
	// TotalSize 声明为 8（小于 HeaderSize），必须终止而非死循环。
	raw := buildPacket(ProtoJSON, OpMessage, []byte(`{}`))
	binary.BigEndian.PutUint32(raw[0:4], 8)
	if _, err := Split(raw); err == nil {
		t.Error("非法 TotalSize 应当报错")
	}
}
