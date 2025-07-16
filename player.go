package main

import (
	"fmt"
	"image"
	"image/color"
	"io"
	"log"
	"math/rand"
	"net"
	"os/exec"
	"sync"
	"time"

	"github.com/dosgo/castX/castxClient"
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
	ready       bool
	videoFfmpeg *exec.Cmd
	inputPort   int
	outputPort  int
	ffmpegOut   io.ReadCloser
}

func NewH264Player(inputPort int, outputPort int) (*H264Player, error) {
	player := &H264Player{
		frameChan:  make(chan *VideoFrame, 3), // 缓冲30帧
		running:    true,
		ready:      false,
		inputPort:  inputPort,
		outputPort: outputPort,
		ffmpegOut:  &FFmpegStrem{port: outputPort},
	}

	if player.width == 0 || player.height == 0 {
		player.width, player.height = 1920, 1080 // 默认分辨率
	}
	if player.framerate == 0 {
		player.framerate = 25.0
	}

	log.Printf("视频信息: %dx%d @ %.2f fps, 时长: %v",
		player.width, player.height, player.framerate)

	go player.decodeRoutine(inputPort)
	return player, nil
}

func (p *H264Player) SetParam(width int, height int, framerate float64) {
	p.width = width
	p.height = height
	p.framerate = framerate
	p.ready = true
}

func (p *H264Player) startNewFFmpeg() {
	// 创建全新的Cmd实例
	p.videoFfmpeg = ffmpeg.Input(fmt.Sprintf("tcp://127.0.0.1:%d?listen", p.inputPort),
		ffmpeg.KwArgs{"f": "h264"}).
		Output(fmt.Sprintf("tcp://127.0.0.1:%d?listen", p.outputPort), // 输出到标准输出
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

func (p *H264Player) decodeRoutine(inputPort int) {
	defer close(p.frameChan)
	//	frameSize := p.width * p.height * 4 // RGBA 每像素3字节
	frameBuffer := make([]byte, 2400*1080*4)

	var newFrameSize int = 0
	for p.running {

		if !p.ready || p.ffmpegOut == nil {
			time.Sleep(time.Millisecond * 5)
			fmt.Printf("decodeRoutine p.ready:%+v\r\n", p.ready)
			continue
		}
		if newFrameSize != p.width*p.height*4 {
			fmt.Printf("newFrameSize:%d new:%d\r\n", newFrameSize, p.width*p.height*4)
		}
		newFrameSize = p.width * p.height * 4
		_, err := io.ReadFull(p.ffmpegOut, frameBuffer[:newFrameSize])
		//if err != nil || n != frameSize {
		if err != nil {
			break // 视频结束或错误
		}

		// 创建RGBA图像
		img := image.NewRGBA(image.Rect(0, 0, p.width, p.height))
		copy(img.Pix, frameBuffer[:newFrameSize]) // 直接使用FFmpeg输出的RGBA数据

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
	inputPort := 9635
	outputPort := 9636
	var err error
	player, err = NewH264Player(inputPort, outputPort)
	if err != nil {
		log.Fatal(err)
	}
	castxClient := castxClient.NewCastXClient()
	castxClient.SetStream(&FFmpegStrem{port: inputPort})
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
			//go player.RebootFfmpeg()
		}
	})
	castxClient.Start("ws://172.30.17.78:8081/ws", "666666", 1920)

	pixelgl.Run(runPlayer)
	fmt.Scanln()
}
func randomInt(min, max int) int {
	return min + rand.Intn(max-min+1)
}

type FFmpegStrem struct {
	conn          net.Conn
	port          int
	reconnectLock sync.Mutex
	reconnecting  bool
}

func (s *FFmpegStrem) startReconnect() {
	s.reconnectLock.Lock()
	defer s.reconnectLock.Unlock()
	// 如果已经在重连中，直接返回
	if s.reconnecting {
		return
	}
	// 标记为重连中
	s.reconnecting = true
	// 启动异步重连
	go s.reconect()
}

// Read 实现 io.Reader 接口
func (r *FFmpegStrem) Read(p []byte) (n int, err error) {
	if r.conn == nil {
		r.startReconnect()
	} else {
		r.conn.SetReadDeadline(time.Now().Add(time.Second * 1))
		n, err = r.conn.Read(p)
		if err != nil {
			fmt.Printf("read err:%+v\r\n", err)
			r.Close()
		}
	}
	return n, err
}

// Write 实现 io.Writer 接口
func (r *FFmpegStrem) Write(p []byte) (n int, err error) {
	if r.conn == nil {
		//fmt.Printf("reconect :%d\r\n", r.port)
		r.startReconnect()
	} else {
		r.conn.SetWriteDeadline(time.Now().Add(time.Second * 1))
		n, err = r.conn.Write(p)
		if err != nil {
			fmt.Printf("Write err:%+v\r\n", err)
			r.Close()
		}
	}
	return n, err
}

func (s *FFmpegStrem) reconect() {
	defer func() {
		s.reconnectLock.Lock()
		s.reconnecting = false
		s.reconnectLock.Unlock()
	}()
	fmt.Printf("reconect port:%d\r\n", s.port)
	time.Sleep(time.Millisecond * 1000)
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", fmt.Sprintf("%d", s.port)), time.Second*5)
	if err == nil {
		s.conn = conn
	} else {
		fmt.Printf("reconect err:%+v\r\n", err)
	}
}

// Close 实现 io.Closer 接口
func (r *FFmpegStrem) Close() error {
	defer func() {
		r.conn = nil
	}()
	if r.conn != nil {
		return r.conn.Close()
	}
	return nil
}
