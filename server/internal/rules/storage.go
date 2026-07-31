package rules

import "sync"

// MemStorage 是房间级键值存储的内存实现。
//
// P2 阶段够用；P3 会换成数据库实现，届时只需换掉本类型，
// 脚本侧的 storage.get/set 接口不变。
type MemStorage struct {
	mu sync.RWMutex
	m  map[string]string
}

var _ Storage = (*MemStorage)(nil)

// NewMemStorage 创建内存存储。
func NewMemStorage() *MemStorage {
	return &MemStorage{m: make(map[string]string)}
}

// Get 取值，不存在时返回 ("", false)。
func (s *MemStorage) Get(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.m[key]
	return v, ok
}

// Set 写入或覆盖。
func (s *MemStorage) Set(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[key] = value
}
