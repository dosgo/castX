package castxClient

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"time"

	"github.com/ebitengine/oto/v3"
	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/speaker"
	"github.com/jfreymuth/pulse"
)

//https://github.com/jfreymuth/pulse

// Player 简单的Opus音频播放器
type Player struct {
	reader   io.Reader
	streamer beep.Streamer
	player   *oto.Player
}

// NewPlayer 创建新的Opus播放器
func NewPlayer(reader io.Reader) *Player {
	p := &Player{reader: reader}

	otoCtx, readyChan, err := oto.NewContext(&oto.NewContextOptions{
		SampleRate:   48000,
		ChannelCount: 2,
		Format:       oto.FormatSignedInt16LE,
		BufferSize:   960 * 1000,
	})
	if err != nil {
		panic("oto.NewContext failed: " + err.Error())
	}

	// 等待音频设备准备就绪
	fmt.Println("等待音频设备准备...")
	<-readyChan
	fmt.Println("音频设备已就绪")

	// 创建播放器
	p.player = otoCtx.NewPlayer(reader)

	return p
}

var start = time.Now()

// Play 开始播放音频流
func (p *Player) Play() {

	/*
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
	*/
	p.player.Play()
	p.player.SetVolume(0.4)

	for p.player.IsPlaying() {
		time.Sleep(time.Millisecond * 10)
	}
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

func (p *Player) WriteData(data []byte) {
	//p.player.Write(data)
}

var t, phase float32

func testpulse() {
	c, err := pulse.NewClient()
	if err != nil {
		fmt.Println(err)
		return
	}
	defer c.Close()

	stream, err := c.NewPlayback(pulse.Float32Reader(synth), pulse.PlaybackLatency(.1))
	if err != nil {
		fmt.Println(err)
		return
	}

	stream.Start()
	stream.Drain()
	fmt.Println("Underflow:", stream.Underflow())
	if stream.Error() != nil {
		fmt.Println("Error:", stream.Error())
	}
	stream.Close()
}
func synth(out []float32) (int, error) {
	for i := range out {
		if t > 4 {
			return i, pulse.EndOfData
		}
		x := float32(math.Sin(2 * math.Pi * float64(phase)))
		out[i] = x * 0.1
		f := [...]float32{440, 550, 660, 880}[int(2*t)&3]
		phase += f / 44100
		if phase >= 1 {
			phase--
		}
		t += 1. / 44100
	}
	return len(out), nil
}
