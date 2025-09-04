package castxClient

import (
	"fmt"
	"io"
	"sync"

	"github.com/dosgo/libopus/opus"
	"github.com/faiface/beep"
	"github.com/faiface/beep/speaker"
)

// Player 简单的Opus音频播放器
type Player struct {
	reader   io.Reader
	streamer beep.Streamer
}

// NewPlayer 创建新的Opus播放器
func NewPlayer(reader io.Reader) *Player {
	p := &Player{reader: reader}
	speaker.Init(48000, 48000/20) // 48kHz采样率，20ms缓冲区
	return p
}

// Play 开始播放音频流
func (p *Player) Play() {
	var mu sync.Mutex

	const sampleRate = 48000
	const channels = 1 // mono; 2 for stereo

	decoder, err := opus.NewOpusDecoder(sampleRate, channels)
	if err != nil {
		return
	}

	p.streamer = beep.StreamerFunc(func(samples [][2]float64) (n int, ok bool) {
		mu.Lock()
		defer mu.Unlock()
		fmt.Printf("StreamerFunc\r\n")
		// 读取Opus数据
		opusData := make([]byte, 1500)
		size, err := p.reader.Read(opusData)
		if err != nil || size == 0 {
			fmt.Printf("StreamerFunc size:%d err:%+v\r\n", err, size)
			return 0, false
		}

		// 解码Opus
		pcmData := make([]int16, len(samples)*2)

		decoded, err := decoder.Decode(opusData, 0, size, pcmData, 0, len(samples), false)
		if err != nil || decoded == 0 {
			fmt.Printf("StreamerFunc22 err:%+v\r\n", err)
			return 0, false
		}

		// 转换为立体声
		for i := 0; i < int(decoded/2); i++ {
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
