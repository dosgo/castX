package comm

import (
	"sync"

	"github.com/gorilla/websocket"
)

type WsSafeConn struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (sc *WsSafeConn) WriteJSON(msg interface{}) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.conn.WriteJSON(msg)
}
func (sc *WsSafeConn) ReadJSON(msg interface{}) error {
	return sc.conn.ReadJSON(msg)
}
func (sc *WsSafeConn) Close() error {
	return sc.conn.Close()
}
