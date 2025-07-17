package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"log"
	"math"
	"os/exec"
	"time"

	"github.com/dosgo/castX/castxClient"
	"github.com/dosgo/castX/castxClient/ffmpegapi"
	"github.com/dosgo/castX/comm"
	"github.com/gopxl/pixel"
	"github.com/gopxl/pixel/pixelgl"
	"github.com/kbinani/screenshot"
	ffmpeg "github.com/u2takey/ffmpeg-go"

	"golang.org/x/image/colornames"
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
}

func NewH264Player(ffmpegIo *ffmpegapi.FfmpegIo) (*H264Player, error) {
	player := &H264Player{
		inputPort:  ffmpegIo.GetPort(true),
		outputPort: ffmpegIo.GetPort(false),
		running:    true,
		ffmpegIo:   ffmpegIo,
	}

	if player.width == 0 || player.height == 0 {
		player.width, player.height = 1920, 1080 // 默认分辨率
	}
	if player.framerate == 0 {
		player.framerate = 25.0
	}
	log.Printf("视频信息: %dx%d @ %.2f fps, 时长: %v",
		player.width, player.height, player.framerate)
	return player, nil
}

func (p *H264Player) SetParam(width int, height int, framerate float64) {
	p.width = width
	p.height = height
	p.ffmpegIo.SetFrameSize(p.width * p.height * 4)
	p.framerate = framerate
}

func (p *H264Player) startNewFFmpeg() {
	// 创建全新的Cmd实例
	p.videoFfmpeg = ffmpeg.Input(fmt.Sprintf("tcp://127.0.0.1:%d", p.inputPort),
		ffmpeg.KwArgs{"f": "h264"}).
		Filter("scale", ffmpeg.Args{fmt.Sprintf("%d:%d", p.width, p.height)}).
		Output(fmt.Sprintf("tcp://127.0.0.1:%d", p.outputPort), // 输出到标准输出
			ffmpeg.KwArgs{
				"format":    "rawvideo",
				"pix_fmt":   "rgba",
				"flags":     "low_delay",
				"avioflags": "direct",
			}).Compile() //.ErrorToStdOut()
	// 启动进程
	if err := p.videoFfmpeg.Start(); err != nil {
		log.Printf("启动FFmpeg失败: %v", err)
		return
	}
}

func (p *H264Player) GetFrame() *image.RGBA {
	if !p.running {
		return nil
	}
	frameBuffer, err := p.ffmpegIo.RecvOutput()
	if err != nil || len(frameBuffer) < p.width*p.height*4 {
		return nil
	}
	img := image.NewRGBA(image.Rect(0, 0, p.width, p.height))
	img.Pix = frameBuffer
	return img
}

func (p *H264Player) Close() {
	p.running = false
}

/*重置解码器*/
func (p *H264Player) RebootFfmpeg() {
	if p.videoFfmpeg != nil {
		p.videoFfmpeg.Process.Kill()
	}
	time.Sleep(time.Millisecond * 500)
	go p.startNewFFmpeg()
}

func runPlayer() {
	// 创建窗口
	width, height := 1920, 1080
	cfg := pixelgl.WindowConfig{
		Title:  "FFmpeg H.264 播放器",
		Bounds: pixel.R(0, 0, float64(width), float64(height)),
		VSync:  true,
	}

	win, err := pixelgl.NewWindow(cfg)
	if err != nil {
		log.Fatal(err)
	}

	var canvas *pixelgl.Canvas
	lastFrame := &pixel.PictureData{
		Pix:    make([]color.RGBA, width*height*4),
		Stride: width * 4,
		Rect:   pixel.R(0, 0, float64(width), float64(height)),
	}

	var isTouching bool
	var touchStartPos pixel.Vec
	var currentTouchPos pixel.Vec

	for !win.Closed() {

		//按下
		if win.JustPressed(pixelgl.MouseButtonLeft) {
			// 获取当前鼠标位置（转换为画布坐标）
			mousePos := win.MousePosition()
			canvasCenter := win.Bounds().Center()
			currentTouchPos = mousePos.Sub(canvasCenter).Add(canvas.Bounds().Center())

			// 触摸开始
			isTouching = true
			touchStartPos = currentTouchPos
			fmt.Printf("onTouchDown: X=%.0f, Y=%.0f\n", currentTouchPos.X, canvas.Bounds().H()-currentTouchPos.Y)

			if client != nil {
				args := map[string]interface{}{
					"x":           currentTouchPos.X,
					"y":           canvas.Bounds().H() - currentTouchPos.Y,
					"type":        "panstart",
					"duration":    0,
					"videoWidth":  player.width,
					"videoHeight": player.height,
				}
				argsStr, _ := json.Marshal(args)
				client.WsClient.SendCmd(comm.MsgTypeControl, string(argsStr))
			}
		}
		//弹起
		if win.JustReleased(pixelgl.MouseButtonLeft) {

			isTouching = false
			// 获取抬起时的位置
			mousePos := win.MousePosition()
			canvasCenter := win.Bounds().Center()
			currentTouchPos = mousePos.Sub(canvasCenter).Add(canvas.Bounds().Center())
			fmt.Printf("onTouchUp: X=%.0f, Y=%.0f\n", currentTouchPos.X, canvas.Bounds().H()-currentTouchPos.Y)
			var _type = "panend"
			var duration = 0
			var touchNum float64 = 0
			if math.Abs(currentTouchPos.X-touchStartPos.X) < touchNum && math.Abs(currentTouchPos.Y-touchStartPos.Y) < touchNum {
				_type = "click"
				duration = 15
			}
			if client != nil {
				args := map[string]interface{}{
					"x":           currentTouchPos.X,
					"y":           canvas.Bounds().H() - currentTouchPos.Y,
					"type":        _type,
					"duration":    duration,
					"videoWidth":  player.width,
					"videoHeight": player.height,
				}
				argsStr, _ := json.Marshal(args)
				client.WsClient.SendCmd(comm.MsgTypeControl, string(argsStr))
			}
		}
		//移动
		if isTouching {
			// 即使没有抬起，也可能在移动
			mousePos := win.MousePosition()
			canvasCenter := win.Bounds().Center()
			currentTouchPos = mousePos.Sub(canvasCenter).Add(canvas.Bounds().Center())
			if client != nil {
				args := map[string]interface{}{
					"x":           currentTouchPos.X,
					"y":           canvas.Bounds().H() - currentTouchPos.Y,
					"type":        "pan",
					"duration":    0,
					"videoWidth":  player.width,
					"videoHeight": player.height,
				}
				argsStr, _ := json.Marshal(args)
				client.WsClient.SendCmd(comm.MsgTypeControl, string(argsStr))
			}
		}

		// 检查是否需要更新窗口大小
		if (player.width > 0 && player.height > 0) &&
			(player.width != width || player.height != height) {
			width, height = player.width, player.height
			win.SetBounds(pixel.R(0, 0, float64(width), float64(height)))
			canvas = pixelgl.NewCanvas(pixel.R(0, 0, float64(width), float64(height)))
			lastFrame = &pixel.PictureData{
				Pix:    make([]color.RGBA, width*height*4),
				Stride: width * 4,
				Rect:   pixel.R(0, 0, float64(width), float64(height)),
			}
		}

		// 根据帧率更新帧

		frame := player.GetFrame()
		if frame != nil {
			lastFrame = pixel.PictureDataFromImage(frame)
		}

		// 渲染
		win.Clear(colornames.Black)
		if canvas == nil && width > 0 && height > 0 {
			canvas = pixelgl.NewCanvas(pixel.R(0, 0, float64(width), float64(height)))
		}

		if canvas != nil {
			canvas.Clear(colornames.Black)
			sprite := pixel.NewSprite(lastFrame, lastFrame.Bounds())
			sprite.Draw(canvas, pixel.IM.Moved(canvas.Bounds().Center()))
			canvas.Draw(win, pixel.IM.Moved(win.Bounds().Center()))
		}

		// 处理输入
		if win.JustPressed(pixelgl.KeySpace) {
			// 暂停/播放逻辑
		}
		if win.JustPressed(pixelgl.KeyRight) {
			// 快进
		}
		if win.JustPressed(pixelgl.KeyLeft) {
			// 快退
		}

		win.Update()
	}
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
		fmt.Printf("info  data:%+v\r\n", data)
		if _height, ok := data["videoHeight"].(float64); ok {
			client.Height = int(_height)
		}
		if _width, ok := data["videoWidth"].(float64); ok {
			client.Width = int(_width)
		}
		if player != nil {

			width := client.Width
			height := client.Height
			bounds := screenshot.GetDisplayBounds(0)
			//宽度超宽
			if width > bounds.Dx() {
				fmt.Printf("eeee\r\n")
				height = int(float64(bounds.Dx()) / float64(width) * float64(height))
				width = bounds.Dx()
			}
			//宽度超宽
			if height > bounds.Dy() {
				dy := bounds.Dy() - 33*2 //窗口状态栏+底部状态栏
				width = int(float64(dy) / float64(height) * float64(width))
				height = dy
			}

			player.SetParam(width, height, 30)
			go player.RebootFfmpeg()
		}
	})
	client.Start("ws://192.168.221.147:8081/ws", "666666", 1920)
	pixelgl.Run(runPlayer)
	fmt.Scanln()
}
