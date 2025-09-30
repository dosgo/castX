package main

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/dosgo/castX/castxServer"
	"github.com/dosgo/castX/comm"
	"github.com/dwdcth/ffmpeg-go/v7/ffcommon"
	"github.com/dwdcth/ffmpeg-go/v7/libavcodec"
	"github.com/dwdcth/ffmpeg-go/v7/libavdevice"
	"github.com/dwdcth/ffmpeg-go/v7/libavutil"

	"github.com/dwdcth/ffmpeg-go/v7/libavformat"
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
		time.Sleep(time.Second * 2)
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
			"channels":    "2",
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
		time.Sleep(time.Second * 2)
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

func ffmpegDesktop1() {
	// 设置FFmpeg DLL路径
	ffcommon.SetAvutilPath("avutil-59.dll")
	ffcommon.SetAvcodecPath("avcodec-61.dll")
	ffcommon.SetAvdevicePath("avdevice-61.dll")
	ffcommon.SetAvfilterPath("avfilter-10.dll")
	ffcommon.SetAvformatPath("avformat-61.dll")
	ffcommon.SetAvpostprocPath("postproc-58.dll")
	ffcommon.SetAvswresamplePath("swresample-5.dll")
	ffcommon.SetAvswscalePath("swscale-8.dll")

	// 注册所有设备
	libavdevice.AvdeviceRegisterAll()

	// 设置屏幕捕获参数
	framerate := 30
	width := 1920
	height := 1080
	port := 1234

	// 打开屏幕捕获设备
	inputFmtCtx := libavformat.AvformatAllocContext()
	if inputFmtCtx == nil {
		log.Fatal("无法分配输入格式上下文")
	}

	// 设置屏幕捕获选项
	options := &libavutil.AVDictionary{}
	libavutil.AvDictSet(&options, "framerate", fmt.Sprintf("%d", framerate), 0)
	libavutil.AvDictSet(&options, "video_size", fmt.Sprintf("%dx%d", width, height), 0)

	// 打开屏幕捕获
	ret := libavformat.AvformatOpenInput(&inputFmtCtx, "desktop", nil, &options)
	if ret < 0 {
		log.Fatalf("无法打开屏幕捕获设备: %d", ret)
	}

	// 查找流信息

	if inputFmtCtx.AvformatFindStreamInfo(&options) < 0 {
		log.Fatal("无法查找流信息")
	}

	// 查找视频流
	videoStreamIndex := -1
	for i := 0; i < int(inputFmtCtx.NbStreams); i++ {

		if inputFmtCtx.GetStream(uint32(i)).Codecpar.CodecType == libavutil.AVMEDIA_TYPE_VIDEO {
			videoStreamIndex = i
			break
		}
	}
	if videoStreamIndex == -1 {
		log.Fatal("找不到视频流")
	}

	// 创建输出格式上下文
	outputFmtCtx := libavformat.AvformatAllocContext()
	if outputFmtCtx == nil {
		log.Fatal("无法分配输出格式上下文")
	}

	// 查找输出格式 (H264 over TCP)
	outputFmt := libavformat.AvGuessFormat("h264", fmt.Sprintf("tcp://127.0.0.1:%d?listen", port), "")
	if outputFmt == nil {
		log.Fatal("无法找到输出格式")
	}
	outputFmtCtx.Oformat = outputFmt

	// 创建视频流

	videoStream := outputFmtCtx.AvformatNewStream(nil)
	if videoStream == nil {
		log.Fatal("无法创建视频流")
	}

	// 配置编码器参数
	codec := libavcodec.AvcodecFindEncoder(libavcodec.AV_CODEC_ID_H264)
	if codec == nil {
		log.Fatal("找不到H.264编码器")
	}

	codecCtx := codec.AvcodecAllocContext3()
	if codecCtx == nil {
		log.Fatal("无法分配编码器上下文")
	}

	// 设置编码参数
	codecCtx.BitRate = 400000
	codecCtx.Width = int32(width)

	codecCtx.Height = int32(height)
	codecCtx.TimeBase = libavutil.AvMakeQ(1, int32(framerate))
	codecCtx.Framerate = libavutil.AvMakeQ(int32(framerate), 1)
	codecCtx.GopSize = 10
	codecCtx.MaxBFrames = 0
	codecCtx.PixFmt = libavutil.AV_PIX_FMT_YUV420P

	// 设置编码器预设
	libavutil.AvOptSet(codecCtx.PrivData, "preset", "ultrafast", 0)
	libavutil.AvOptSet(codecCtx.PrivData, "tune", "zerolatency", 0)
	libavutil.AvOptSet(codecCtx.PrivData, "crf", "28", 0)

	// 打开编码器
	if ret := codecCtx.AvcodecOpen2(codec, nil); ret < 0 {
		log.Fatalf("无法打开编码器: %d", ret)
	}

	// 复制编码器参数到输出流

	if codecCtx.AvcodecParametersToContext(videoStream.Codecpar) < 0 {
		log.Fatal("无法复制编码器参数")
	}

	// 打开输出
	if ret := libavformat.AvioOpen(&outputFmtCtx.Pb, fmt.Sprintf("tcp://127.0.0.1:%d?listen", port), libavformat.AVIO_FLAG_WRITE); ret < 0 {
		log.Fatalf("无法打开输出: %d", ret)
	}

	// 写入文件头
	if outputFmtCtx.AvformatWriteHeader(nil) < 0 {
		log.Fatal("无法写入文件头")
	}

	// 分配帧和包
	frame := libavutil.AvFrameAlloc()
	if frame == nil {
		log.Fatal("无法分配帧")
	}
	frame.Width = int32(width)
	frame.Height = int32(height)
	frame.Format = int32(codecCtx.PixFmt)
	if frame.AvFrameGetBuffer(0) < 0 {
		log.Fatal("无法分配帧缓冲区")
	}

	pkt := libavcodec.AvPacketAlloc()
	if pkt == nil {
		log.Fatal("无法分配包")
	}

	// 开始捕获和编码循环
	frameCount := 0
	for {
		// 读取帧
		if inputFmtCtx.AvReadFrame(pkt) < 0 {
			break
		}

		// 只处理视频流
		if int(pkt.StreamIndex) == videoStreamIndex {
			// 发送帧到编码器
			if codecCtx.AvcodecSendFrame(frame) < 0 {
				log.Printf("发送帧到编码器失败")
				continue
			}

			// 接收编码后的包
			for ret >= 0 {
				ret = codecCtx.AvcodecReceivePacket(pkt)
				if ret == libavutil.AVERROR_EAGAIN || ret == libavutil.AVERROR_EOF {
					break
				} else if ret < 0 {
					log.Printf("接收包失败")
					break
				}

				// 设置包的时间戳和流索引
				pkt.SetStreamIndex(int32(videoStreamIndex))
				pkt.RescaleTs(codecCtx.TimeBase(), videoStream.TimeBase())

				// 写入包
				if libavformat.AvWriteFrame(outputFmtCtx, pkt) < 0 {
					log.Printf("写入包失败")
				}

				libavcodec.AvPacketUnref(pkt)
			}
		}

		libavcodec.AvPacketUnref(pkt)
		frameCount++

		// 每100帧打印一次进度
		if frameCount%100 == 0 {
			log.Printf("已处理 %d 帧", frameCount)
		}

		// 添加延迟以控制帧率
		time.Sleep(time.Second / time.Duration(framerate))
	}

	// 写入文件尾
	libavformat.AvWriteTrailer(outputFmtCtx)

	// 清理资源
	libavcodec.AvcodecClose(codecCtx)
	libavformat.AvformatCloseInput(&inputFmtCtx)
	if outputFmtCtx.Pb() != nil {
		libavformat.AvioClosep(&outputFmtCtx.Pb())
	}
	libavformat.AvformatFreeContext(outputFmtCtx)
	libavutil.AvFrameFree(&frame)
	libavcodec.AvPacketFree(&pkt)

	log.Println("屏幕捕获完成")
}
