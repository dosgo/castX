package castxClient

import (
	"encoding/binary"
	"io"
	"sync"
	"time"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/speaker"
)

// Player 简单的Opus音频播放器
type Player struct {
	reader   io.Reader
	streamer beep.Streamer
}

// NewPlayer 创建新的Opus播放器
func NewPlayer(reader io.Reader) *Player {
	p := &Player{reader: reader}
	sr := beep.SampleRate(48000)
	speaker.Init(sr, sr.N(time.Second/20)) // 48kHz采样率，20ms缓冲区

	return p
}
func bytesToInt16(data []byte) []int16 {
	if len(data)%2 != 0 {
		panic("字节长度必须是2的倍数")
	}

	result := make([]int16, len(data)/2)
	for i := range result {
		result[i] = int16(binary.LittleEndian.Uint16(data[i*2 : (i+1)*2]))
	}
	return result
}

// Play 开始播放音频流
func (p *Player) Play() {
	var mu sync.Mutex
	p.streamer = beep.StreamerFunc(func(samples [][2]float64) (n int, ok bool) {
		mu.Lock()
		defer mu.Unlock()
		// 读取Opus数据
		opusData := make([]byte, 5000)
		size, err := p.reader.Read(opusData)
		if err != nil || size == 0 {
			return 0, false
		}
		pcmData := bytesToInt16(opusData[:size])
		// 转换为立体声
		for i := 0; i < int(size/2); i++ {
			if i >= len(samples) {
				break
			}
			val := float32(pcmData[i]) / 32768.0
			samples[i][0] = float64(val)
			samples[i][1] = float64(val)
			n = i + 1
		}
		return n, true
	})
	speaker.Play(p.streamer)
}

// Close 停止播放并释放资源
func (p *Player) Close() {
	speaker.Clear()
}
