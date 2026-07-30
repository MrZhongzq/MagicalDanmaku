package event

import (
	"crypto/rand"
	"encoding/json"
	"time"

	"github.com/oklog/ulid/v2"
)

// Event 是归一化后的直播事件信封。
type Event struct {
	ID         string          // ULID，用于去重与全链路追踪
	RoomID     string          // 事件所属直播间号
	Platform   Platform        // 来源平台
	Type       Type            // 归一化事件类型
	Timestamp  time.Time       // 平台时间；缺失时等于 ReceivedAt
	ReceivedAt time.Time       // 本地接收时间
	Payload    Payload         // 强类型载荷
	Raw        json.RawMessage // 原始 CMD，永不为 nil
}

// NewID 生成一个 ULID 字符串，按时间单调递增可排序。
func NewID() string {
	return ulid.MustNew(ulid.Now(), rand.Reader).String()
}
