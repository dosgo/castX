package castxClient

import (
	"fmt"
	"io"
	"time"

	"github.com/ebitengine/oto/v3"
	"github.com/gopxl/beep/v2/speaker"
)

// Player 简单的Opus音频播放器
type Player struct {
	reader io.Reader
	player *oto.Player
}

// NewPlayer 创建新的Opus播放器
func NewPlayer(reader io.Reader) *Player {
	p := &Player{reader: reader}

	otoCtx, readyChan, err := oto.NewContext(&oto.NewContextOptions{
		SampleRate:   48000,
		ChannelCount: 2,
		Format:       oto.FormatSignedInt16LE,
		BufferSize:   2400,
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

// Play 开始播放音频流
func (p *Player) Play() {
	p.player.Play()
	for p.player.IsPlaying() {
		time.Sleep(time.Millisecond)
	}
}

// Close 停止播放并释放资源
func (p *Player) Close() {
	speaker.Clear()
}
