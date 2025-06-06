package comm

import (
	"sync"

	"github.com/gorilla/websocket"
)

type ConnectionManager struct {
	connections map[*websocket.Conn]bool
	rwMutex     sync.RWMutex // 改为读写锁
}

// 添加连接时使用写锁
func (cm *ConnectionManager) Add(conn *websocket.Conn) {
	cm.rwMutex.Lock()
	defer cm.rwMutex.Unlock()
	cm.connections[conn] = true
}

// 移除连接时使用写锁
func (cm *ConnectionManager) Remove(conn *websocket.Conn) {
	cm.rwMutex.Lock()
	defer cm.rwMutex.Unlock()
	delete(cm.connections, conn)
}

// 广播时使用读锁
func (cm *ConnectionManager) Broadcast(msg WSMessage) {
	cm.rwMutex.RLock()
	defer cm.rwMutex.RUnlock()
	for conn := range cm.connections {
		go func(c *websocket.Conn) { // 每个连接独立goroutine发送
			c.WriteJSON(msg)
		}(conn)
	}
}
