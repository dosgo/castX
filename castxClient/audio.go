package castxClient

import (
	"encoding/binary"
	"io"
	"math"
	"sync"

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
	speaker.Init(sr, 960) // 48kHz采样率，20ms缓冲区
	return p
}

// Play 开始播放音频流
func (p *Player) Play() {
	var mu sync.Mutex
	p.streamer = beep.StreamerFunc(func(samples [][2]float64) (n int, ok bool) {
		mu.Lock()
		defer mu.Unlock()
		// 读取Opus数据
		opusData := make([]byte, 960*2)
		size, err := p.reader.Read(opusData)
		if err != nil || size == 0 {
			return 0, false
		}
		bytesToSamples(opusData[:size], samples)
		return len(samples), true
	})
	speaker.Play(p.streamer)
}

func bytesToSamples(data []byte, samples [][2]float64) {
	// 每4个字节代表一个立体声样本（左右声道各2字节）
	sampleCount := len(data) / 4
	//samples := make([][2]float64, sampleCount)
	for i := 0; i < sampleCount; i++ {
		// 计算当前样本在字节切片中的位置
		pos := i * 4
		// 提取左声道样本（前2字节）
		left := int16(binary.LittleEndian.Uint16(data[pos : pos+2]))
		// 提取右声道样本（后2字节）
		right := int16(binary.LittleEndian.Uint16(data[pos+2 : pos+4]))

		// 将int16转换为float64并归一化到[-1, 1]范围
		samples[i] = [2]float64{
			float64(left) / math.MaxInt16,
			float64(right) / math.MaxInt16,
		}
	}
}

// Close 停止播放并释放资源
func (p *Player) Close() {
	speaker.Clear()
}
