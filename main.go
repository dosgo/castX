package main

import (
	"fmt"
	"io"

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
		if controlData["type"] == "left" {
			if f, ok := controlData["x"].(float64); ok {
				x := int(f)
				y := int(controlData["y"].(float64))
				robotgo.Move(x, y)
				robotgo.Click("left", false)
			}
		}
	})

	go ffmpegDesktop(false, castx.WebrtcServer)
	fmt.Scanln()
}

/*启动录屏*/
func ffmpegDesktop(audio bool, webrtcServer *comm.WebrtcServer) {
	h264Buf := comm.NewMemoryWriter(webrtcServer, framerate)
	var stream []*ffmpeg.Stream
	var ioW []io.Writer
	// 使用ffmpeg-go捕获屏幕并编码为H264
	videoOutput := ffmpeg.Input("desktop",
		ffmpeg.KwArgs{
			"f":         "gdigrab", // Windows屏幕捕获
			"framerate": framerate, // 帧率
			//"video_size": fmt.Sprintf("%dx%d", width, height), // 分辨率
		}).
		Output("pipe:1", // 输出到标准输出
			ffmpeg.KwArgs{
				"crf":         "28",
				"map":         "0:v",
				"preset":      "ultrafast",     // 最快编码
				"tune":        "zerolatency",   // 零延迟模式
				"x264-params": "no-scenecut=1", // 零延迟模式
				//"profile:v": "baseline",                 // 基线档次
				"pix_fmt":  "yuv420p",                  // 像素格式
				"f":        "h264",                     // 原始H264输出
				"movflags": "frag_keyframe+empty_moov", // 流式优化
			})
	stream = append(stream, videoOutput)
	ioW = append(ioW, h264Buf)
	if audio {

		audioWriter := comm.NewAudioWriter(webrtcServer)
		audioOutput := ffmpeg.Input("audio=virtual-audio-capturer",
			ffmpeg.KwArgs{
				"f":           "dshow",
				"sample_rate": "48000",
				"channels":    "2",
			}).Output("pipe:2",
			ffmpeg.KwArgs{
				"map":           "1:a",
				"acodec":        "libopus",
				"audio_bitrate": "64k",
				"f":             "opus",
			})
		stream = append(stream, audioOutput)
		ioW = append(ioW, audioWriter)
	}

	ffmpeg.MergeOutputs(stream...).WithOutput(ioW...).OverWriteOutput().
		Run()
}
