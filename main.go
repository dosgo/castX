package main

import (
	"fmt"
	"net"
	"time"

	"github.com/dosgo/castX/castxServer"
	"github.com/dosgo/castX/comm"
	"github.com/go-vgo/robotgo"
	"github.com/kbinani/screenshot"
	ffmpeg "github.com/u2takey/ffmpeg-go"
)

var framerate = 30

func main() {

	bounds := screenshot.GetDisplayBounds(0)
	castx, _ := castxServer.Start(8081, bounds.Dx(), bounds.Dy(), "", false, "123456", 0)
	castx.WsServer.SetControlFun(func(controlData map[string]interface{}) {
		if controlData["type"] == "click" {
			if f, ok := controlData["x"].(float64); ok {
				x := int(f)
				y := int(controlData["y"].(float64))
				robotgo.Move(x, y)
				robotgo.Click("left", false)
			}
		}
		if controlData["type"] == "rightClick" {
			if f, ok := controlData["x"].(float64); ok {
				x := int(f)
				y := int(controlData["y"].(float64))
				robotgo.Move(x, y)
				robotgo.Click("right", false)
			}
		}
	})

	go ffmpegDesktop(9901, castx.WebrtcServer)
	go ffmpegAudio(9902, castx.WebrtcServer)
	fmt.Scanln()
}

/*启动录屏*/
func ffmpegDesktop(port int, webrtcServer *comm.WebrtcServer) {
	// 使用ffmpeg-go捕获屏幕并编码为H264
	videoOutput := ffmpeg.Input("desktop",
		ffmpeg.KwArgs{
			"f":         "gdigrab", // Windows屏幕捕获
			"framerate": framerate, // 帧率
			//"video_size": fmt.Sprintf("%dx%d", width, height), // 分辨率
		}).
		Output(fmt.Sprintf("tcp://127.0.0.1:%d?listen", port), // 输出到标准输出
			ffmpeg.KwArgs{
				"crf":         "28",
				"preset":      "ultrafast",                // 最快编码
				"tune":        "zerolatency",              // 零延迟模式
				"x264-params": "no-scenecut=1",            // 零延迟模式
				"profile:v":   "baseline",                 // 基线档次
				"pix_fmt":     "yuv420p",                  // 像素格式
				"f":           "h264",                     // 原始H264输出
				"movflags":    "frag_keyframe+empty_moov", // 流式优化
			})

	go func() {
		time.Sleep(time.Second * 5)
		// 连接到FFmpeg服务器
		videoConn, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", fmt.Sprintf("%d", port)))
		if err != nil {
			fmt.Printf("视频接收失败: %v\n", err)
			return
		}
		processor := comm.NewH264Stream(webrtcServer)
		// 启动处理
		processor.ProcessStream(videoConn)
	}()
	videoOutput.Run()
}

/*audio*/
func ffmpegAudio(port int, webrtcServer *comm.WebrtcServer) {

	audioOutput := ffmpeg.Input("audio=virtual-audio-capturer",
		ffmpeg.KwArgs{
			"f":           "dshow",
			"sample_rate": "48000",

			"channels": "2",
		}).Output(fmt.Sprintf("tcp://127.0.0.1:%d?listen", port),
		ffmpeg.KwArgs{
			"acodec":        "libopus",
			"ab":            "64k",
			"f":             "opus",
			"ar":            "48000",
			"ac":            "2",
			"page_duration": "20000",
		})

	go func() {
		time.Sleep(time.Second * 5)
		// 连接到FFmpeg服务器
		audioConn, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", fmt.Sprintf("%d", port)))
		if err != nil {
			fmt.Printf("音频接收失败: %v\n", err)
			return
		}
		audioWriter := comm.NewAudioWriter(webrtcServer)
		audioWriter.Strem(audioConn)
	}()
	audioOutput.Run()
}
