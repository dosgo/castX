package comm

import (
	"bytes"
	"sync"
	"time"
)

func NewMemoryWriter(webrtcServer *WebrtcServer, framerate int) *MemoryWriter {
	m := &MemoryWriter{
		framerate: framerate,
		run:       true,
	}
	m.webrtcServer = webrtcServer
	go m.toWebrtc()
	return m
}

type MemoryWriter struct {
	buffer       []byte
	mu           sync.RWMutex
	framerate    int
	run          bool
	webrtcServer *WebrtcServer
}

func (m *MemoryWriter) Write(p []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.buffer = append(m.buffer, p...)
	return len(p), nil
}

func (m *MemoryWriter) updata(i int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.buffer = m.buffer[i:]
}

func (m *MemoryWriter) Close() {
	m.run = false
}

/*
检测完整帧
*/
func (m *MemoryWriter) processBuffer() []byte {
	var nal []byte
	for m.run {
		i := findStartCode(m)
		if i == -1 {
			break
		}
		if i > 0 {
			nal = m.buffer[:i]
			m.updata(i)
			break
		} else {
			m.updata(4)
		}
	}
	return nal
}

// 转发到webrtc
func (m *MemoryWriter) toWebrtc() {
	for m.run {
		nal := m.processBuffer()
		if len(nal) > 0 {
			m.webrtcServer.SendWebrtc(nal, time.Now().Local().UnixMicro(), time.Second/time.Duration(m.framerate), false)
		} else {
			time.Sleep(time.Millisecond * 1)
		}
	}
}
func findStartCode(iow *MemoryWriter) int {
	iow.mu.Lock()
	defer iow.mu.Unlock()
	startCode := []byte{0x00, 0x00, 0x00, 0x01}
	idx := bytes.Index(iow.buffer, startCode)
	return idx
}
