package comm

import (
	"sync"
	"time"
)

type ttlMap struct {
	valueInfo map[string]int
	keyTime   map[string]int64
	mu        sync.RWMutex // 读写锁
	ttl       int64
	run       bool
}

func (ttlmap *ttlMap) Store(key string, value int) {
	ttlmap.mu.Lock()
	defer ttlmap.mu.Unlock()
	ttlmap.valueInfo[key] = value
	ttlmap.keyTime[key] = time.Now().UnixMilli()
}
func (ttlmap *ttlMap) Incr(key string, num int) int {
	ttlmap.mu.Lock()
	defer ttlmap.mu.Unlock()
	var value = 0
	if _value, exists := ttlmap.valueInfo[key]; exists {
		value = _value
	}
	ttlmap.valueInfo[key] = value + num
	ttlmap.keyTime[key] = time.Now().UnixMilli()
	return ttlmap.valueInfo[key]
}
func (ttlmap *ttlMap) Get(key string) int {
	if _value, exists := ttlmap.valueInfo[key]; exists {
		return _value
	}
	return 0
}

func (ttlmap *ttlMap) IsExists(key string) bool {
	if _, exists := ttlmap.valueInfo[key]; exists {
		return true
	}
	return false
}

func NewTTLMap(ttl int64) *ttlMap {
	m := &ttlMap{
		valueInfo: make(map[string]int),
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
	for key, _ := range m.valueInfo {
		if now > m.keyTime[key]+m.ttl {
			delete(m.valueInfo, key)
			delete(m.keyTime, key)
		}
	}
}

func (c *ttlMap) Close() {
	c.run = false
}
