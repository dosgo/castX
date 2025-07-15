package main

import (
	"fmt"
	"image"
	"image/color"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/dosgo/castX/castxClient"
	"github.com/faiface/pixel"
	"github.com/faiface/pixel/pixelgl"
	ffmpeg "github.com/u2takey/ffmpeg-go"
	"golang.org/x/image/colornames"
)

type VideoFrame struct {
	Image *image.RGBA
	Time  time.Duration
}

type H264Player struct {
	frameChan chan *VideoFrame
	framerate float64
	width     int
	height    int
	running   bool
	ready     bool
	wg        sync.WaitGroup
}

func NewH264Player(inputPort int) (*H264Player, error) {
	player := &H264Player{
		frameChan: make(chan *VideoFrame, 30), // 缓冲30帧
		running:   true,
		ready:     false,
	}

	if player.width == 0 || player.height == 0 {
		player.width, player.height = 1920, 1080 // 默认分辨率
	}
	if player.framerate == 0 {
		player.framerate = 25.0
	}

	log.Printf("视频信息: %dx%d @ %.2f fps, 时长: %v",
		player.width, player.height, player.framerate)

	player.wg.Add(1)
	go player.decodeRoutine(inputPort)
	return player, nil
}

func (p *H264Player) SetParam(width int, height int, framerate float64) {
	p.width = width
	p.height = height
	p.framerate = framerate
	p.ready = true
}
func (p *H264Player) decodeRoutine(inputPort int) {
	defer p.wg.Done()
	defer close(p.frameChan)

	reader, writer := io.Pipe()
	videoOutput := ffmpeg.Input(fmt.Sprintf("tcp://127.0.0.1:%d?listen", inputPort),
		ffmpeg.KwArgs{}).
		Output("pipe:1", // 输出到标准输出
			ffmpeg.KwArgs{
				"format":  "rawvideo",
				"pix_fmt": "rgb24",
			}).WithOutput(writer)
	go videoOutput.Run()

	fmt.Printf("decodeRoutine11\r\n")
	time.Sleep(time.Second * 5)

	//	frameSize := p.width * p.height * 3 // RGB24 每像素3字节
	frameBuffer := make([]byte, 2400*1080*3)

	for p.running {

		if !p.ready {
			time.Sleep(time.Millisecond * 5)
			continue
		}
		newFrameSize := p.width * p.height * 3
		_, err := io.ReadFull(reader, frameBuffer[:newFrameSize])

		//if err != nil || n != frameSize {
		if err != nil {
			break // 视频结束或错误
		}
		fmt.Printf("newFrameSize:%d\r\n", newFrameSize)

		// 创建RGBA图像
		img := image.NewRGBA(image.Rect(0, 0, p.width, p.height))
		for y := 0; y < p.height; y++ {
			for x := 0; x < p.width; x++ {
				srcIdx := (y*p.width + x) * 3
				/*
					img.SetRGBA(x, p.height-1-y, toRGBA( // 需要垂直翻转
						frameBuffer[srcIdx],
						frameBuffer[srcIdx+1],
						frameBuffer[srcIdx+2],
					))*/

				img.SetRGBA(x, y, toRGBA( // 需要垂直翻转
					frameBuffer[srcIdx],
					frameBuffer[srcIdx+1],
					frameBuffer[srcIdx+2],
				))
			}
		}

		// 发送帧
		select {
		case p.frameChan <- &VideoFrame{Image: img}:
			// 正常发送
		default:
			// 通道满，丢弃帧
		}

	}
}

func toRGBA(r, g, b byte) color.RGBA {
	return color.RGBA{r, g, b, 255}
}

func (p *H264Player) Close() {
	p.running = false
	p.wg.Wait()
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

var inputPort int
var outputStream io.WriteCloser
var player *H264Player

func main() {
	inputPort = 9635
	go func() {
		time.Sleep(time.Second * 5)
		castxClient := castxClient.NewCastXClient()
		conn, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", fmt.Sprintf("%d", inputPort)))
		if err != nil {
			fmt.Printf("视频接收失败: %v\n", err)
			return
		}
		castxClient.SetStream(conn)
		castxClient.WsClient.SetInfoNotifyFun(func(data map[string]interface{}) {
			fmt.Printf("info  data:%+v\r\n", data)
			if _height, ok := data["videoHeight"].(float64); ok {
				castxClient.Height = int(_height)
			}
			if _width, ok := data["videoWidth"].(float64); ok {
				castxClient.Width = int(_width)
			}
			player.SetParam(castxClient.Width, castxClient.Height, 30)
		})
		castxClient.Start("ws://172.30.17.78:8081/ws", "666666", 1920)
	}()
	var err error
	player, err = NewH264Player(inputPort)
	if err != nil {
		log.Fatal(err)
	}
	pixelgl.Run(runPlayer)
	fmt.Scanln()
}
