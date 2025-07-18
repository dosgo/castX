package main

import (
	"encoding/json"
	"fmt"
	"image"
	"log"
	"math"
	"os/exec"
	"time"

	"github.com/dosgo/castX/castxClient"
	"github.com/dosgo/castX/castxClient/ffmpegapi"
	"github.com/dosgo/castX/comm"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	ffmpeg "github.com/u2takey/ffmpeg-go"
)

type H264Player struct {
	framerate   float64
	width       int
	height      int
	running     bool
	videoFfmpeg *exec.Cmd
	inputPort   int
	outputPort  int
	ffmpegIo    *ffmpegapi.FfmpegIo
	currentImg  *ebiten.Image // 直接存储Ebiten图像
}

func NewH264Player(ffmpegIo *ffmpegapi.FfmpegIo) (*H264Player, error) {
	player := &H264Player{
		inputPort:  ffmpegIo.GetPort(true),
		outputPort: ffmpegIo.GetPort(false),
		running:    true,
		ffmpegIo:   ffmpegIo,
	}

	if player.width == 0 || player.height == 0 {
		player.width, player.height = 1920, 1080
	}
	if player.framerate == 0 {
		player.framerate = 25.0
	}
	log.Printf("视频信息: %dx%d @ %.2f fps", player.width, player.height, player.framerate)
	return player, nil
}

func (p *H264Player) SetParam(width int, height int, framerate float64) {
	p.width = width
	p.height = height
	p.ffmpegIo.SetFrameSize(p.width * p.height * 4) // RGB24: 3字节/像素
	p.framerate = framerate
}

func (p *H264Player) startNewFFmpeg() {
	p.videoFfmpeg = ffmpeg.Input(fmt.Sprintf("tcp://127.0.0.1:%d", p.inputPort),
		ffmpeg.KwArgs{"f": "h264"}).
		Filter("scale", ffmpeg.Args{fmt.Sprintf("%d:%d", p.width, p.height)}).
		Output(fmt.Sprintf("tcp://127.0.0.1:%d", p.outputPort),
			ffmpeg.KwArgs{
				"format":    "rawvideo",
				"pix_fmt":   "rgba", // 关键修改：使用RGB24格式
				"flags":     "low_delay",
				"avioflags": "direct",
			}).Compile()

	if err := p.videoFfmpeg.Start(); err != nil {
		log.Printf("启动FFmpeg失败: %v", err)
	}
}

func (p *H264Player) GetFrame() {
	if !p.running {
		return
	}

	frameBuffer, err := p.ffmpegIo.RecvOutput()
	if err != nil || len(frameBuffer) < p.width*p.height*4 {
		return
	}

	// 零拷贝创建RGB24图像
	if p.currentImg == nil || p.currentImg.Bounds().Dx() != p.width || p.currentImg.Bounds().Dy() != p.height {
		p.currentImg = ebiten.NewImage(p.width, p.height)
	}

	// 直接写入像素数据（高性能方式）
	p.currentImg.WritePixels(frameBuffer)
}

func (p *H264Player) Close() {
	p.running = false
}

func (p *H264Player) RebootFfmpeg() {
	if p.videoFfmpeg != nil {
		p.videoFfmpeg.Process.Kill()
	}
	time.Sleep(time.Millisecond * 500)
	go p.startNewFFmpeg()
}

// Ebiten游戏结构
type Game struct {
	player                         *H264Player
	client                         *castxClient.CastXClient
	touchStartPos, currentTouchPos image.Point
	isTouching                     bool
}

func (g *Game) Update() error {
	// 获取新帧
	g.player.GetFrame()

	// 处理触摸/鼠标事件
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		x, y := ebiten.CursorPosition()
		g.isTouching = true
		g.touchStartPos = image.Point{x, y}
		g.currentTouchPos = g.touchStartPos

		if g.client != nil {
			g.sendTouchEvent("panstart")
		}
	}

	if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) {
		x, y := ebiten.CursorPosition()
		g.isTouching = false
		g.currentTouchPos = image.Point{x, y}

		var eventType = "panend"
		var duration = 0
		if math.Abs(float64(g.currentTouchPos.X-g.touchStartPos.X)) < 5 &&
			math.Abs(float64(g.currentTouchPos.Y-g.touchStartPos.Y)) < 5 {
			eventType = "click"
			duration = 15
		}

		if g.client != nil {
			g.sendTouchEvent(eventType, duration)
		}
	}

	if g.isTouching {
		x, y := ebiten.CursorPosition()
		g.currentTouchPos = image.Point{x, y}

		if g.client != nil {
			g.sendTouchEvent("pan")
		}
	}

	return nil
}

func (g *Game) sendTouchEvent(eventType string, duration ...int) {
	dur := 0
	if len(duration) > 0 {
		dur = duration[0]
	}

	args := map[string]interface{}{
		"x":           g.currentTouchPos.X,
		"y":           g.player.height - g.currentTouchPos.Y,
		"type":        eventType,
		"duration":    dur,
		"videoWidth":  g.player.width,
		"videoHeight": g.player.height,
	}
	argsStr, _ := json.Marshal(args)
	g.client.WsClient.SendCmd(comm.MsgTypeControl, string(argsStr))
}

func (g *Game) Draw(screen *ebiten.Image) {
	// 渲染当前帧
	if g.player.currentImg != nil {
		screen.DrawImage(g.player.currentImg, nil)
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	// 动态调整窗口大小
	if g.player.width > 0 && g.player.height > 0 {
		return g.player.width, g.player.height
	}
	return 1920, 1080
}

var player *H264Player
var client *castxClient.CastXClient

func main() {
	videoffmpegIo := ffmpegapi.NewFfmpegIo()
	var err error
	player, err = NewH264Player(videoffmpegIo)
	if err != nil {
		log.Fatal(err)
	}
	client = castxClient.NewCastXClient()
	client.SetStream(videoffmpegIo)

	client.WsClient.SetInfoNotifyFun(func(data map[string]interface{}) {
		if _height, ok := data["videoHeight"].(float64); ok {
			client.Height = int(_height)
		}
		if _width, ok := data["videoWidth"].(float64); ok {
			client.Width = int(_width)
		}
		if player != nil {
			player.SetParam(client.Width, client.Height, 30)
			go player.RebootFfmpeg()
		}
	})

	client.Start("ws://172.30.17.78:8081/ws", "666666", 1920)

	// 创建Ebiten游戏
	game := &Game{
		player: player,
		client: client,
	}

	// 设置窗口
	ebiten.SetWindowTitle("FFmpeg H.264 播放器 (Ebiten)")
	ebiten.SetWindowSize(1920, 1080)
	ebiten.SetWindowResizable(true)

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
