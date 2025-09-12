package castxClient

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
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
	speaker.Init(sr, 960) // 48kHz采样率，20ms缓冲区
	return p
}

var start = time.Now()

// Play 开始播放音频流
func (p *Player) Play() {
	var mu sync.Mutex
	p.streamer = beep.StreamerFunc(func(samples [][2]float64) (n int, ok bool) {
		mu.Lock()
		defer mu.Unlock()

		elapsed := time.Since(start)
		fmt.Printf("elapsed:%+v len(samples):%d\r\n", elapsed, len(samples))
		start = time.Now()
		// 读取Opus数据
		opusData := make([]byte, len(samples)*4) // 每个样本4字节（16位立体声）
		start1 := time.Now()
		size, err := io.ReadFull(p.reader, opusData)
		elapsed1 := time.Since(start1)
		fmt.Printf("read elapsed:%+v\r\n", elapsed1)
		if err != nil || size == 0 {
			return 0, false
		}
		_len := bytesToSamples(opusData[:size], samples)
		return _len, true
	})
	speaker.Play(p.streamer)
}

func bytesToSamples(data []byte, samples [][2]float64) int {
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
	return sampleCount
}

// Close 停止播放并释放资源
func (p *Player) Close() {
	speaker.Clear()
}
