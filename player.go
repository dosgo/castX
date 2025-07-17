package main

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"os/exec"
	"time"

	"github.com/dosgo/castX/castxClient"
	"github.com/dosgo/castX/castxClient/ffmpegapi"
	"github.com/gopxl/pixel"
	"github.com/gopxl/pixel/pixelgl"
	ffmpeg "github.com/u2takey/ffmpeg-go"

	"golang.org/x/image/colornames"
)

type VideoFrame struct {
	Image *image.RGBA
	Time  time.Duration
}

type H264Player struct {
	frameChan   chan *VideoFrame
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
		frameChan:  make(chan *VideoFrame, 3), // 缓冲30帧
		running:    true,
		inputPort:  ffmpegIo.GetPort(true),
		outputPort: ffmpegIo.GetPort(false),
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

	go player.decodeRoutine()
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

func (p *H264Player) decodeRoutine() {
	defer close(p.frameChan)
	for p.running {
		frameBuffer, err := p.ffmpegIo.RecvOutput()
		if err != nil {
			continue
		}
		// 创建RGBA图像
		img := image.NewRGBA(image.Rect(0, 0, p.width, p.height))
		img.Pix = frameBuffer // 直接使用FFmpeg输出的RGBA数据
		// 发送帧
		select {
		case p.frameChan <- &VideoFrame{Image: img}:
			// 正常发送
		default:
			// 通道满，丢弃帧
		}
	}
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

	for !win.Closed() {
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

		select {
		case frame := <-player.frameChan:
			if frame != nil {
				lastFrame = pixel.PictureDataFromImage(frame.Image)
			}
		default:
			// 没有新帧，保留上一帧
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

func main() {
	videoffmpegIo := ffmpegapi.NewFfmpegIo()
	var err error
	player, err = NewH264Player(videoffmpegIo)
	if err != nil {
		log.Fatal(err)
	}
	castxClient := castxClient.NewCastXClient()
	castxClient.SetStream(videoffmpegIo)
	castxClient.WsClient.SetInfoNotifyFun(func(data map[string]interface{}) {
		fmt.Printf("info  data:%+v\r\n", data)
		if _height, ok := data["videoHeight"].(float64); ok {
			castxClient.Height = int(_height)
		}
		if _width, ok := data["videoWidth"].(float64); ok {
			castxClient.Width = int(_width)
		}
		if player != nil {
			player.SetParam(castxClient.Width, castxClient.Height, 30)
			go player.RebootFfmpeg()
		}
	})
	castxClient.Start("ws://172.30.17.78:8081/ws", "666666", 1920)
	pixelgl.Run(runPlayer)
	fmt.Scanln()
}
