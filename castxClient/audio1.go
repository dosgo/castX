package castxClient

import (
	"encoding/binary"
	"io"
	"time"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/speaker"
)

//https://github.com/jfreymuth/pulse

// Player 简单的Opus音频播放器
type Player1 struct {
	reader io.Reader
	buf    []byte
	len    int
	pos    int
	format beep.Format
}

// NewPlayer 创建新的Opus播放器
func NewPlayer1(reader io.Reader) *Player1 {
	p := &Player1{reader: reader}

	p.format = beep.Format{
		SampleRate:  beep.SampleRate(44100),
		NumChannels: 2,
		Precision:   2,
	}
	p.buf = make([]byte, 512*p.format.Width())

	speaker.Init(p.format.SampleRate, p.format.SampleRate.N(time.Second/50))
	return p
}

// Play 开始播放音频流
func (p *Player1) Play() {
	//	resampled := beep.Resample(10, 48000, 44100, p.getNoise())
	speaker.Play(p.getStream1())
}

// Close 停止播放并释放资源
func (p *Player1) Close() {
	speaker.Clear()
}

func (p *Player1) getStream() beep.Streamer {
	var buf = make([]byte, 512*6)
	return beep.StreamerFunc(func(samples [][2]float64) (n int, ok bool) {
		io.ReadFull(p.reader, buf[:len(samples)*4])
		for i := range samples {
			pos := i * 4
			samples[i][0] = float64(int16(binary.LittleEndian.Uint16(buf[pos:pos+2]))) / 32768
			samples[i][1] = float64(int16(binary.LittleEndian.Uint16(buf[pos+2:pos+4]))) / 32768
		}
		return len(samples), true
	})
}

func (s *Player1) getStream1() beep.Streamer {
	var buf = make([]byte, 512*6)
	return beep.StreamerFunc(func(samples [][2]float64) (n int, ok bool) {
		width := s.format.Width()
		io.ReadFull(s.reader, buf[:len(samples)*width])
		//fmt.Printf("len:%d\r\n", len(samples)*width)
		for i := range samples {
			pos := i * width
			samples[i], _ = s.format.DecodeSigned(buf[pos:])
		}

		return len(samples), true
	})
}
