package comm

import (
	"fmt"
	"sync"
	"time"
)

type AudioWriter struct {
	buffer       []byte
	mu           sync.Mutex
	sampleRate   int
	webrtcServer *WebrtcServer
}

func NewAudioWriter(webrtcServer *WebrtcServer) *AudioWriter {
	a := &AudioWriter{
		sampleRate: 48000,
	}
	a.webrtcServer = webrtcServer
	return a
}

func (a *AudioWriter) Write(p []byte) (n int, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.buffer = append(a.buffer, p...)
	// 按20ms分割音频帧（48000Hz * 20ms = 960采样）
	frameSize := 960 * 2 * 2 // 960采样 * 2声道 * 2字节(16bit)
	for len(a.buffer) >= frameSize {
		audioFrame := a.buffer[:frameSize]
		if err := a.webrtcServer.SendWebrtc(audioFrame, time.Now().Local().UnixMicro(), time.Millisecond*20, true); err != nil {
			fmt.Printf("AudioWriter SendWebrtc error:%v\r\n", err)
			return 0, err
		}
		a.buffer = a.buffer[frameSize:]
	}
	return len(p), nil
}
