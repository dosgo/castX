package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"os/exec"
	"sync"
	"time"

	"github.com/dosgo/castX/castxClient"
	"github.com/dosgo/castX/castxClient/ffmpegapi"
	"github.com/dosgo/castX/comm"
	"github.com/jupiterrider/purego-sdl3/sdl"
	ffmpeg "github.com/u2takey/ffmpeg-go"
)

const (
	YUV420P_FRAME_SIZE = 1.5 // YUV420p每像素字节数
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
	frameMutex  sync.Mutex
	frameData   []byte // YUV420p原始数据
}

func NewH264Player(ffmpegIo *ffmpegapi.FfmpegIo) (*H264Player, error) {
	player := &H264Player{
		inputPort:  ffmpegIo.GetPort(true),
		outputPort: ffmpegIo.GetPort(false),
		running:    true,
		ffmpegIo:   ffmpegIo,
	}

	if player.width == 0 || player.height == 0 {
		player.width, player.height = 1920, 864
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
	// YUV420p帧大小: w*h + (w/2 * h/2) * 2 = w*h*1.5
	p.ffmpegIo.SetFrameSize(int(float64(p.width*p.height) * YUV420P_FRAME_SIZE))
	p.framerate = framerate
}

func (p *H264Player) startNewFFmpeg() {
	p.videoFfmpeg = ffmpeg.Input(fmt.Sprintf("tcp://127.0.0.1:%d", p.inputPort),
		ffmpeg.KwArgs{"f": "h264"}).
		Filter("scale", ffmpeg.Args{fmt.Sprintf("%d:%d", p.width, p.height)}).
		Output(fmt.Sprintf("tcp://127.0.0.1:%d", p.outputPort),
			ffmpeg.KwArgs{
				"format":    "rawvideo",
				"pix_fmt":   "yuv420p", // YUV420p格式
				"flags":     "low_delay",
				"avioflags": "direct",
			}).Compile()

	if err := p.videoFfmpeg.Start(); err != nil {
		log.Printf("启动FFmpeg失败: %v", err)
	}
}

func (p *H264Player) GetFrame() []byte {
	if !p.running {
		return nil
	}

	frameBuffer, err := p.ffmpegIo.RecvOutput()
	if err != nil || len(frameBuffer) < int(float64(p.width*p.height)*YUV420P_FRAME_SIZE) {
		return nil
	}

	p.frameMutex.Lock()
	defer p.frameMutex.Unlock()

	// 直接复用缓冲区
	if p.frameData == nil || cap(p.frameData) < len(frameBuffer) {
		p.frameData = make([]byte, len(frameBuffer))
	}
	copy(p.frameData, frameBuffer)

	return p.frameData
}

func (p *H264Player) Close() {
	p.running = false
}

func (p *H264Player) RebootFfmpeg() {
	if p.videoFfmpeg != nil {
		fmt.Printf("kill ffmoeg\r\n")
		if p.videoFfmpeg.Process != nil {
			p.videoFfmpeg.Process.Kill()
		}
	}
	time.Sleep(time.Millisecond * 500)
	go p.startNewFFmpeg()
}

// SDL2渲染器
type SDLPlayer struct {
	window   *sdl.Window
	renderer *sdl.Renderer
	texture  *sdl.Texture
	player   *H264Player
	client   *castxClient.CastXClient
}

func NewSDLPlayer(player *H264Player, client *castxClient.CastXClient) (*SDLPlayer, error) {
	// 初始化SDL
	if !sdl.Init(sdl.InitVideo | sdl.InitEvents) {
		return nil, errors.New("初始化SDL失败")
	}

	// 创建窗口
	window := sdl.CreateWindow("FFmpeg H.264 播放器 (SDL2)",
		int32(player.width), int32(player.height),
		sdl.WindowResizable)

	// 创建渲染器
	renderer := sdl.CreateRenderer(window, "")

	texture := sdl.CreateTexture(renderer, sdl.PixelFormatIYUV, sdl.TextureAccessStreaming, int32(player.width), int32(player.height))
	// 创建YUV纹理

	return &SDLPlayer{
		window:   window,
		renderer: renderer,
		texture:  texture,
		player:   player,
		client:   client,
	}, nil
}

func (s *SDLPlayer) Run() {
	defer s.Destroy()

	var isTouching bool
	var touchStartPos sdl.Point
	//var currentTouchPos sdl.Point

	running := true
	//	frameDelay := time.Second / time.Duration(s.player.framerate)

	for running {
		//frameStart := time.Now()
		var event sdl.Event
		// 处理事件
		for sdl.PollEvent(&event) {
			switch event.Type() {
			case sdl.EventQuit:
				running = false

			case sdl.EventMouseButtonDown:
				if sdl.MouseButtonFlags(event.Button().Button) == sdl.ButtonLeft {

					isTouching = true
					touchStartPos = sdl.Point{X: int32(event.Button().X), Y: int32(event.Button().Y)}
					//currentTouchPos = touchStartPos
					s.sendTouchEvent("panstart", int32(event.Button().X), int32(event.Button().Y))
					fmt.Printf("x:%d y:%d\r\n", int32(event.Button().X), int32(event.Button().Y))
				}
			case sdl.EventMouseButtonUp:

				isTouching = false
				// 计算移动距离判断点击
				dist := math.Sqrt(math.Pow(float64(int32(event.Button().X)-touchStartPos.X), 2) +
					math.Pow(float64(int32(event.Button().Y)-touchStartPos.Y), 2))
				eventType := "panend"
				duration := 0
				if dist < 5 { // 小于5像素视为点击
					eventType = "click"
					duration = 15
				}
				s.sendTouchEvent(eventType, int32(event.Button().X), int32(event.Button().Y), duration)

			case sdl.EventMouseMotion:
				if isTouching {
					//	currentTouchPos = sdl.Point{X: int32(event.Button().X), Y: int32(event.Button().Y)}
					s.sendTouchEvent("pan", int32(event.Button().X), int32(event.Button().Y))
				}

			}
		}

		// 获取新帧
		frameData := s.player.GetFrame()

		if frameData != nil {
			// 直接更新YUV纹理（零拷贝）
			sdl.UpdateYUVTexture(s.texture, nil,
				(*byte)(&frameData[0]), int32(s.player.width),
				(*byte)(&frameData[s.player.width*s.player.height]), int32(s.player.width/2),
				(*byte)(&frameData[s.player.width*s.player.height*5/4]), int32(s.player.width/2))
		}

		// 渲染

		sdl.RenderClear(s.renderer)
		sdl.RenderTexture(s.renderer, s.texture, nil, nil)
		sdl.RenderPresent(s.renderer)

		// 控制帧率

		sdl.DelayNS(uint64(1000 * 5000))

	}
}

func (s *SDLPlayer) sendTouchEvent(eventType string, x, y int32, duration ...int) {
	if s.client == nil {
		return
	}

	dur := 0
	if len(duration) > 0 {
		dur = duration[0]
	}

	args := map[string]interface{}{
		"x":           float64(x),
		"y":           float64(y),
		"type":        eventType,
		"duration":    dur,
		"videoWidth":  s.player.width,
		"videoHeight": s.player.height,
	}
	argsStr, _ := json.Marshal(args)
	s.client.WsClient.SendCmd(comm.MsgTypeControl, string(argsStr))
}

func (s *SDLPlayer) RebuildTexture() {
	if s.texture != nil {
		sdl.DestroyTexture(s.texture)
	}

	s.texture = sdl.CreateTexture(s.renderer, sdl.PixelFormatIYUV, sdl.TextureAccessStreaming, int32(player.width), int32(player.height))
	sdl.SetWindowSize(s.window, int32(s.player.width), int32(s.player.height))
}

func (s *SDLPlayer) SetWindowSize() {
	sdl.SetWindowSize(s.window, int32(s.player.width), int32(s.player.height))
}

func (s *SDLPlayer) Destroy() {
	if s.texture != nil {
		sdl.DestroyTexture(s.texture)
	}
	if s.renderer != nil {
		sdl.DestroyRenderer(s.renderer)
	}
	if s.window != nil {
		sdl.DestroyWindow(s.window)
	}
	sdl.Quit()
}

var player *H264Player
var client *castxClient.CastXClient
var sdlPlayer *SDLPlayer

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

	client.Start("ws://172.30.16.70:8081/ws", "666666", 1920)

	// 创建SDL播放器
	sdlPlayer, err = NewSDLPlayer(player, client)
	if err != nil {
		log.Fatalf("SDL初始化失败: %v", err)
	}
	sdlPlayer.Run()
}
