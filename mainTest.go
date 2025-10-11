package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
	"unsafe"

	"github.com/dosgo/castX/castxServer"
	"github.com/dosgo/castX/comm"
	"github.com/dwdcth/ffmpeg-go/v7/ffcommon"
	"github.com/dwdcth/ffmpeg-go/v7/libavcodec"
	"github.com/dwdcth/ffmpeg-go/v7/libavdevice"
	"github.com/dwdcth/ffmpeg-go/v7/libavformat"
	"github.com/dwdcth/ffmpeg-go/v7/libavutil"
	"github.com/dwdcth/ffmpeg-go/v7/libswscale"
	"github.com/go-vgo/robotgo"
	"github.com/kbinani/screenshot"
	ffmpeg "github.com/u2takey/ffmpeg-go"
)

var framerate = 30
var width, height int

func main() {

	bounds := screenshot.GetDisplayBounds(0)
	castx, _ := castxServer.Start(8088, bounds.Dx(), bounds.Dy(), "", false, "123456", 0)
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

	go captureDesktopFrames1()
	//go ffmpegAudio(9902, castx.WebrtcServer)
	fmt.Scanln()
}

/*启动录屏*/
func ffmpegDesktopold(port int, webrtcServer *comm.WebrtcServer) {
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

func ffmpegDesktop() {
	// 设置FFmpeg DLL路径
	ffcommon.SetAvutilPath("avutil-59.dll")
	ffcommon.SetAvcodecPath("avcodec-61.dll")
	ffcommon.SetAvdevicePath("avdevice-61.dll")
	ffcommon.SetAvfilterPath("avfilter-10.dll")
	ffcommon.SetAvformatPath("avformat-61.dll")
	ffcommon.SetAvpostprocPath("postproc-58.dll")
	ffcommon.SetAvswresamplePath("swresample-5.dll")
	ffcommon.SetAvswscalePath("swscale-8.dll")
	ffcommon.GetAvutilDll()
	// 注册所有设备
	libavdevice.AvdeviceRegisterAll()

	port := 9527

	// 打开屏幕捕获设备
	inputFmtCtx := libavformat.AvformatAllocContext()
	if inputFmtCtx == nil {
		log.Fatal("无法分配输入格式上下文")
	}

	// 设置屏幕捕获选项

	var options *libavutil.AVDictionary
	libavutil.AvDictSet(&options, "framerate", fmt.Sprintf("%d", framerate), 0)

	// 查找gdigrab输入格式
	inputFmt := libavformat.AvFindInputFormat("gdigrab")
	if inputFmt == nil {
		log.Fatal("找不到gdigrab输入格式")
	}

	// 打开屏幕捕获
	ret := libavformat.AvformatOpenInput(&inputFmtCtx, "desktop", inputFmt, &options)
	if ret < 0 {
		log.Fatalf("无法打开屏幕捕获设备: %d", ret)
	}

	// 查找流信息

	if inputFmtCtx.AvformatFindStreamInfo(&options) < 0 {
		log.Fatal("无法查找流信息")
	}

	// 查找视频流
	videoStreamIndex := 0

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
				if ret == libavutil.AVERROR_EOF {
					break
				} else if ret < 0 {
					log.Printf("接收包失败")
					break
				}
				pkt.StreamIndex = uint32(videoStreamIndex)

				// 设置包的时间戳和流索引
				pkt.Pts = int64(codecCtx.TimeBase.Num)

				outputFmtCtx.AvWriteFrame(pkt)

				// 写入包
				if outputFmtCtx.AvWriteFrame(pkt) < 0 {
					log.Printf("写入包失败")
				}
				pkt.AvPacketUnref()
			}
		}

		frameCount++

		// 每100帧打印一次进度
		if frameCount%100 == 0 {
			log.Printf("已处理 %d 帧", frameCount)
		}

		// 添加延迟以控制帧率
		time.Sleep(time.Second / time.Duration(framerate))
	}

	// 写入文件尾
	outputFmtCtx.AvWriteTrailer()

	// 清理资源
	codecCtx.AvcodecClose()
	libavformat.AvformatCloseInput(&inputFmtCtx)
	if outputFmtCtx.Pb != nil {
		libavformat.AvioClosep(&outputFmtCtx.Pb)
	}
	libavformat.AvioContextFree(&outputFmtCtx.Pb)
	libavutil.AvFrameFree(&frame)
	libavcodec.AvPacketFree(&pkt)

	log.Println("屏幕捕获完成")
}

func captureDesktopFrames() {
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

	// 打开屏幕捕获设备
	inputFmtCtx := libavformat.AvformatAllocContext()
	if inputFmtCtx == nil {
		log.Fatal("无法分配输入格式上下文")
	}

	// 设置屏幕捕获选项

	var options *libavutil.AVDictionary
	libavutil.AvDictSet(&options, "framerate", fmt.Sprintf("%d", framerate), 0)

	// 查找gdigrab输入格式
	inputFmt := libavformat.AvFindInputFormat("gdigrab")
	if inputFmt == nil {
		log.Fatal("找不到gdigrab输入格式")
	}

	// 打开屏幕捕获
	ret := libavformat.AvformatOpenInput(&inputFmtCtx, "desktop", inputFmt, &options)
	if ret < 0 {
		log.Fatalf("无法打开屏幕捕获设备: %d", ret)
	}

	// 查找流信息
	if inputFmtCtx.AvformatFindStreamInfo(nil) < 0 {
		log.Fatal("无法查找流信息")
	}

	// 获取视频流
	videoStreamIndex := 0
	videoStream := inputFmtCtx.GetStream(uint32(videoStreamIndex))
	if videoStream == nil {
		log.Fatal("无法获取视频流")
	}

	// 分配包
	pkt := libavcodec.AvPacketAlloc()
	if pkt == nil {
		log.Fatal("无法分配包")
	}
	defer libavcodec.AvPacketFree(&pkt)

	log.Println("开始捕获屏幕帧...")

	// 开始捕获循环
	frameCount := 0
	startTime := time.Now()

	for {
		// 读取帧
		ret := inputFmtCtx.AvReadFrame(pkt)
		if ret < 0 {
			if ret == -int32(libavutil.EAGAIN) {
				time.Sleep(time.Millisecond * 10)
				continue
			}
			log.Printf("读取帧失败: %d", ret)
			break
		}

		// 只处理视频流
		if int(pkt.StreamIndex) == videoStreamIndex {
			frameCount++

			// 打印帧信息
			fmt.Printf("帧 #%d\n", frameCount)
			fmt.Printf("  大小: %d 字节\n", pkt.Size)
			fmt.Printf("  时间戳: %d\n", pkt.Pts)
			fmt.Printf("  持续时间: %d\n", pkt.Duration)

			// 打印帧类型

			if pkt.Flags&libavcodec.AV_PKT_FLAG_KEY != 0 {
				fmt.Println("  类型: 关键帧 (I帧)")
			} else {
				fmt.Println("  类型: 非关键帧")
			}

			// 打印前16字节的十六进制（可选）
			if pkt.Data != nil && pkt.Size > 0 {
				dataSlice := unsafe.Slice(pkt.Data, 64)
				fmt.Printf("  前16字节: %x\n", dataSlice)
			}

			fmt.Println("------------------------")
		}
		pkt.AvPacketUnref()

		// 计算并显示FPS
		if frameCount%10 == 0 {
			elapsed := time.Since(startTime).Seconds()
			fps := float64(frameCount) / elapsed
			fmt.Printf("当前FPS: %.2f\n", fps)
		}

		// 添加延迟以控制帧率
		time.Sleep(time.Second / time.Duration(framerate))
	}
}

func captureDesktopFrames1() {
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
	framerate := 25
	width := 1920
	height := 1080

	// 打开屏幕捕获设备
	inputFmtCtx := libavformat.AvformatAllocContext()
	if inputFmtCtx == nil {
		log.Fatal("无法分配输入格式上下文")
	}

	// 设置屏幕捕获选项
	var options *libavutil.AVDictionary
	libavutil.AvDictSet(&options, "framerate", fmt.Sprintf("%d", framerate), 0)

	// 查找gdigrab输入格式
	inputFmt := libavformat.AvFindInputFormat("gdigrab")
	if inputFmt == nil {
		log.Fatal("找不到gdigrab输入格式")
	}

	// 打开屏幕捕获
	ret := libavformat.AvformatOpenInput(&inputFmtCtx, "desktop", inputFmt, &options)
	if ret < 0 {
		log.Fatalf("无法打开屏幕捕获设备: %d", ret)
	}

	// 查找流信息
	if inputFmtCtx.AvformatFindStreamInfo(nil) < 0 {
		log.Fatal("无法查找流信息")
	}

	// 获取视频流
	videoStreamIndex := 0

	// 获取视频流
	videoStream := inputFmtCtx.GetStream(uint32(videoStreamIndex))
	if videoStream == nil {
		log.Fatal("无法获取视频流")
	}

	// 分配包
	pkt := libavcodec.AvPacketAlloc()
	if pkt == nil {
		log.Fatal("无法分配包")
	}
	defer libavcodec.AvPacketFree(&pkt)

	// 创建H.264编码器
	codec := libavcodec.AvcodecFindEncoder(libavcodec.AV_CODEC_ID_H264)
	if codec == nil {
		log.Fatal("找不到H.264编码器")
	}

	codecCtx := codec.AvcodecAllocContext3()
	if codecCtx == nil {
		log.Fatal("无法分配编码器上下文")
	}
	defer codecCtx.AvcodecClose()

	// 设置编码参数
	codecCtx.BitRate = 4000000 // 4 Mbps
	codecCtx.Width = int32(width)
	codecCtx.Height = int32(height)
	codecCtx.Framerate = libavutil.AVRational{Num: int32(framerate), Den: 1}
	codecCtx.GopSize = 10
	codecCtx.MaxBFrames = 1
	codecCtx.PixFmt = libavutil.AV_PIX_FMT_YUV420P
	codecCtx.TimeBase = libavutil.AVRational{Num: 1, Den: int32(framerate)}
	codecCtx.TimecodeFrameStart = 0
	codecCtx.SampleAspectRatio = libavutil.AVRational{Num: 1, Den: 1}

	// 设置编码器预设
	libavutil.AvOptSet(codecCtx.PrivData, "preset", "ultrafast", 0)
	libavutil.AvOptSet(codecCtx.PrivData, "tune", "zerolatency", 0)
	libavutil.AvOptSet(codecCtx.PrivData, "crf", "23", 0)

	libavutil.AvOptSet(codecCtx.PrivData, "profile", "baseline", 0)

	//codecPara := videoStream.Codecpar
	fmt.Printf("videoStream:%+v\r\n", videoStream)
	//	codecPara.CodecType = libavutil.AVMEDIA_TYPE_VIDEO

	//	if codecCtx.AvcodecParametersToContext(codecPara) < 0 {
	//	fmt.Printf("Cannot alloc codec ctx from para.\n")

	//}
	// 打开编码器
	if ret := codecCtx.AvcodecOpen2(codec, nil); ret < 0 {
		errBuf := make([]byte, 256)
		libavutil.AvStrerror(ret, (*byte)(unsafe.Pointer(&errBuf[0])), 256)
		log.Fatalf("无法打开编码器: %d, %s", ret, string(errBuf))
	}

	// 创建帧用于存储原始数据
	rawFrame := libavutil.AvFrameAlloc()
	if rawFrame == nil {
		log.Fatal("无法分配原始帧")
	}
	defer libavutil.AvFrameFree(&rawFrame)

	// 创建帧用于存储YUV420P数据
	yuvFrame := libavutil.AvFrameAlloc()
	if yuvFrame == nil {
		log.Fatal("无法分配YUV帧")
	}
	defer libavutil.AvFrameFree(&yuvFrame)

	yuvFrame.Width = int32(width)
	yuvFrame.Height = int32(height)
	yuvFrame.Format = int32(libavutil.AV_PIX_FMT_YUV420P)
	if ret := yuvFrame.AvFrameGetBuffer(0); ret < 0 {
		log.Fatal("无法分配YUV帧缓冲区")
	}

	// 创建图像转换上下文
	swsCtx := libswscale.SwsGetContext(
		int32(width), int32(height), libavutil.AV_PIX_FMT_BGRA,
		int32(width), int32(height), libavutil.AV_PIX_FMT_YUV420P,
		libswscale.SWS_BILINEAR, nil, nil, nil,
	)
	if swsCtx == nil {
		log.Fatal("无法创建图像转换上下文")
	}
	defer swsCtx.SwsFreeContext()

	// 设置中断信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 启动goroutine处理中断信号
	go func() {
		<-sigChan
		log.Println("\n收到中断信号，停止捕获...")
		os.Exit(0)
	}()

	log.Println("开始捕获屏幕帧并编码为H.264...")
	log.Println("按Ctrl+C停止捕获...")

	// 开始捕获循环
	frameCount := 0
	startTime := time.Now()

	for {
		// 读取帧
		ret := inputFmtCtx.AvReadFrame(pkt)
		if ret < 0 {
			if ret == -int32(libavutil.EAGAIN) {
				time.Sleep(time.Millisecond * 10)
				continue
			}
			log.Printf("读取帧失败: %d", ret)
			break
		}

		// 只处理视频流
		if int(pkt.StreamIndex) == videoStreamIndex {
			frameCount++

			// 准备原始帧
			rawFrame.Width = int32(width)
			rawFrame.Height = int32(height)
			rawFrame.Format = int32(libavutil.AV_PIX_FMT_BGRA)
			rawFrame.Data[0] = pkt.Data
			rawFrame.Linesize[0] = int32(width * 4) // BGRA每像素4字节

			// 转换像素格式为YUV420P

			swsCtx.SwsScale(
				(**uint8)(unsafe.Pointer(&rawFrame.Data)),
				(*int32)(unsafe.Pointer(&rawFrame.Linesize)),
				0, uint32(height),
				(**uint8)(unsafe.Pointer(&yuvFrame.Data)),
				(*int32)(unsafe.Pointer(&yuvFrame.Linesize)),
			)

			// 设置帧时间戳
			yuvFrame.Pts = int64(frameCount)

			// 发送帧到编码器
			if ret := codecCtx.AvcodecSendFrame(yuvFrame); ret < 0 {
				log.Printf("发送帧到编码器失败: %d", ret)
				continue
			}

			// 接收编码后的包
			encPkt := libavcodec.AvPacketAlloc()
			for {
				ret := codecCtx.AvcodecReceivePacket(encPkt)
				if ret == -int32(libavutil.EAGAIN) || ret == int32(libavutil.AVERROR_EOF) {
					break
				} else if ret < 0 {
					log.Printf("接收编码包失败: %d", ret)
					break
				}

				// 打印H.264帧信息
				fmt.Printf("H.264帧 #%d\n", frameCount)
				fmt.Printf("  大小: %d 字节\n", encPkt.Size)
				fmt.Printf("  时间戳: %d\n", encPkt.Pts)
				fmt.Printf("  持续时间: %d\n", encPkt.Duration)

				// 打印帧类型
				if encPkt.Flags&libavcodec.AV_PKT_FLAG_KEY != 0 {
					fmt.Println("  类型: 关键帧 (I帧)")
				} else {
					fmt.Println("  类型: 非关键帧")
				}

				// 打印前16字节的十六进制
				if encPkt.Data != nil && encPkt.Size > 0 {
					dataSlice := unsafe.Slice(encPkt.Data, 16)
					fmt.Printf("  前16字节: %x\n", dataSlice)
				}

				fmt.Println("------------------------")

				encPkt.AvPacketUnref()
			}
			libavcodec.AvPacketFree(&encPkt)
		}
		pkt.AvPacketUnref()

		// 计算并显示FPS
		if frameCount%10 == 0 {
			elapsed := time.Since(startTime).Seconds()
			fps := float64(frameCount) / elapsed
			fmt.Printf("当前FPS: %.2f\n", fps)
		}

		// 添加延迟以控制帧率
		time.Sleep(time.Second / time.Duration(framerate))
	}
}
