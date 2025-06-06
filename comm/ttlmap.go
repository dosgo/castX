package comm

import (
	"sync"
	"time"
)

type ttlMap struct {
	tokenInfo map[string]interface{}
	keyTime   map[string]int64
	mu        sync.RWMutex // 读写锁
	ttl       int64
	run       bool
}

func (ttlmap *ttlMap) Add(token string, value interface{}) {
	ttlmap.mu.Lock()
	defer ttlmap.mu.Unlock()
	ttlmap.tokenInfo[token] = value
	ttlmap.keyTime[token] = time.Now().UnixMilli()
}
func (ttlmap *ttlMap) IsExists(token string) bool {
	if _, exists := ttlmap.tokenInfo[token]; exists {
		return true
	}
	return false
}

func NewTTLMap(ttl int64) *ttlMap {
	m := &ttlMap{
		tokenInfo: make(map[string]interface{}),
		keyTime:   make(map[string]int64),
		ttl:       ttl,
		run:       true,
	}
	// 启动后台清理协程
	go m.cleanupLoop()
	return m
}
func (c *ttlMap) cleanupLoop() {
	for c.run {
		time.Sleep(time.Second * time.Duration(c.ttl-10))
		c.cleanupExpired()
	}
}
func (m *ttlMap) cleanupExpired() {
	now := time.Now().UnixMilli()
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, _ := range m.tokenInfo {
		if now > m.keyTime[key]+m.ttl {
			delete(m.tokenInfo, key)
			delete(m.keyTime, key)
		}
	}
}

func (c *ttlMap) Close() {
	c.run = false
}
