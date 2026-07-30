package wire

import (
	"bytes"
	"testing"
)

func TestEncodeProducesCorrectHeader(t *testing.T) {
	body := []byte("hello")
	got := Encode(OpAuth, body)

	if len(got) != HeaderSize+len(body) {
		t.Fatalf("总长度 = %d, 期望 %d", len(got), HeaderSize+len(body))
	}

	h, err := DecodeHeader(got)
	if err != nil {
		t.Fatalf("DecodeHeader 失败: %v", err)
	}
	if h.TotalSize != uint32(HeaderSize+len(body)) {
		t.Errorf("TotalSize = %d, 期望 %d", h.TotalSize, HeaderSize+len(body))
	}
	if h.HeaderSize != HeaderSize {
		t.Errorf("HeaderSize = %d, 期望 %d", h.HeaderSize, HeaderSize)
	}
	if h.ProtoVer != ProtoJSON {
		t.Errorf("ProtoVer = %d, 期望 %d", h.ProtoVer, ProtoJSON)
	}
	if h.Operation != OpAuth {
		t.Errorf("Operation = %d, 期望 %d", h.Operation, OpAuth)
	}
	if h.Sequence != 1 {
		t.Errorf("Sequence = %d, 期望 1", h.Sequence)
	}
	if !bytes.Equal(got[HeaderSize:], body) {
		t.Errorf("body = %q, 期望 %q", got[HeaderSize:], body)
	}
}

func TestDecodeHeaderRejectsShortInput(t *testing.T) {
	for _, n := range []int{0, 1, 15} {
		if _, err := DecodeHeader(make([]byte, n)); err == nil {
			t.Errorf("长度 %d 应当报错，实际返回 nil", n)
		}
	}
}

func TestDecodeHeaderReadsBigEndian(t *testing.T) {
	// 手工构造：TotalSize=0x00000020, HeaderSize=0x0010,
	// ProtoVer=0x0002, Operation=0x00000005, Sequence=0x00000001
	raw := []byte{
		0x00, 0x00, 0x00, 0x20,
		0x00, 0x10,
		0x00, 0x02,
		0x00, 0x00, 0x00, 0x05,
		0x00, 0x00, 0x00, 0x01,
	}
	h, err := DecodeHeader(raw)
	if err != nil {
		t.Fatalf("DecodeHeader 失败: %v", err)
	}
	if h.TotalSize != 32 || h.ProtoVer != ProtoZlib || h.Operation != OpMessage {
		t.Errorf("解析结果错误: %+v", h)
	}
}
