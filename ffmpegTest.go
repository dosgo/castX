package main

import (
	"fmt"
	"os"
	"time"
	"unsafe"

	"github.com/moonfdd/ffmpeg-go/ffcommon"
	"github.com/moonfdd/ffmpeg-go/libavcodec"
	"github.com/moonfdd/ffmpeg-go/libavdevice"
	"github.com/moonfdd/ffmpeg-go/libavformat"
	"github.com/moonfdd/ffmpeg-go/libavutil"
	"github.com/moonfdd/ffmpeg-go/libswscale"
	sdl "github.com/moonfdd/sdl2-go/sdl2"
	"github.com/moonfdd/sdl2-go/sdlcommon"
)

// Output YUV420P
const OUTPUT_YUV420P = 0

// '1' Use Dshow
// '0' Use GDIgrab
const USE_DSHOW = 0

// Refresh Event
const SFM_REFRESH_EVENT = (sdl.SDL_USEREVENT + 1)
const SFM_BREAK_EVENT = (sdl.SDL_USEREVENT + 2)

var thread_exit ffcommon.FInt = 0
var ispush = true

func sfp_refresh_thread(opaque ffcommon.FVoidP) uintptr {
	// thread_exit = 0
	for thread_exit == 0 {
		var event sdl.SDL_Event
		event.Type = SFM_REFRESH_EVENT
		if ispush {
			event.SDL_PushEvent()
			ispush = false
		}
		// sdl.SDL_Delay(40)
	}
	fmt.Println("sfp_refresh_thread 发送退出事件")
	// thread_exit = 0
	//Break
	var event sdl.SDL_Event
	event.Type = SFM_BREAK_EVENT
	event.SDL_PushEvent()

	return 0
}

// Show Dshow Device
func show_dshow_device() {
	pFormatCtx := libavformat.AvformatAllocContext()
	var options *libavutil.AVDictionary
	libavutil.AvDictSet(&options, "list_devices", "true", 0)
	iformat := libavformat.AvFindInputFormat("dshow")
	fmt.Printf("========Device Info=============\n")
	libavformat.AvformatOpenInput(&pFormatCtx, "video=dummy", iformat, &options)
	fmt.Printf("================================\n")
}

// Show AVFoundation Device
func show_avfoundation_device() {
	pFormatCtx := libavformat.AvformatAllocContext()
	var options *libavutil.AVDictionary
	libavutil.AvDictSet(&options, "list_devices", "true", 0)
	iformat := libavformat.AvFindInputFormat("avfoundation")
	fmt.Printf("==AVFoundation Device Info===\n")
	libavformat.AvformatOpenInput(&pFormatCtx, "", iformat, &options)
	fmt.Printf("=============================\n")
}

func main0() (ret ffcommon.FInt) {
	var pFormatCtx *libavformat.AVFormatContext
	var i, videoindex ffcommon.FInt
	var pCodecCtx *libavcodec.AVCodecContext
	var pCodec *libavcodec.AVCodec
	var ifmt *libavformat.AVInputFormat

	libavformat.AvRegisterAll()
	libavformat.AvformatNetworkInit()
	pFormatCtx = libavformat.AvformatAllocContext()

	//Open File
	//char filepath[]="src01_480x272_22.h265";
	//avformat_open_input(&pFormatCtx,filepath,NULL,NULL)

	//Register Device
	libavdevice.AvdeviceRegisterAll()
	//Windows
	if USE_DSHOW != 0 {
		//Use dshow
		//
		//Need to Install screen-capture-recorder
		//screen-capture-recorder
		//Website: http://sourceforge.net/projects/screencapturer/
		//
		ifmt = libavformat.AvFindInputFormat("dshow")
		if libavformat.AvformatOpenInput(&pFormatCtx, "video=screen-capture-recorder", ifmt, nil) != 0 {
			fmt.Printf("Couldn't open input stream1.\n")
			return -1
		}
	} else {
		//Use gdigrab
		var options *libavutil.AVDictionary
		//Set some options
		//grabbing frame rate
		//av_dict_set(&options,"framerate","5",0);
		//The distance from the left edge of the screen or desktop
		//av_dict_set(&options,"offset_x","20",0);
		//The distance from the top edge of the screen or desktop
		//av_dict_set(&options,"offset_y","40",0);
		//Video frame size. The default is to capture the full screen
		//av_dict_set(&options,"video_size","640x480",0);
		// libavutil.AvDictSet(&options, "probesize", "100000000", 0)
		ifmt = libavformat.AvFindInputFormat("gdigrab")
		if libavformat.AvformatOpenInput(&pFormatCtx, "desktop", ifmt, &options) != 0 {
			fmt.Printf("Couldn't open input stream2.\n")
			return -1
		}

	}

	if pFormatCtx.AvformatFindStreamInfo(nil) < 0 {
		fmt.Println("Couldn't find stream information.")
		return -1
	}
	videoindex = -1
	for i = 0; i < int32(pFormatCtx.NbStreams); i++ {
		fmt.Printf(" pFormatCtx.GetStream(uint32(i)).Codec:%+v\r\n", pFormatCtx.GetStream(uint32(i)).Codec)
		if pFormatCtx.GetStream(uint32(i)).Codec.CodecType == libavutil.AVMEDIA_TYPE_VIDEO {
			videoindex = i
			break
		}
	}
	if videoindex == -1 {
		fmt.Printf("Didn't find a video stream.\n")
		return -1
	}
	pCodecCtx = pFormatCtx.GetStream(uint32(videoindex)).Codec
	pCodec = libavcodec.AvcodecFindDecoder(pCodecCtx.CodecId)
	if pCodec == nil {
		fmt.Printf("Codec not found.\n")
		return -1
	}
	if pCodecCtx.AvcodecOpen2(pCodec, nil) < 0 {
		fmt.Printf("Could not open codec.\n")
		return -1
	}

	var pFrame, pFrameYUV *libavutil.AVFrame
	pFrame = libavutil.AvFrameAlloc()
	pFrameYUV = libavutil.AvFrameAlloc()
	//unsigned char *out_buffer=(unsigned char *)av_malloc(avpicture_get_size(AV_PIX_FMT_YUV420P, pCodecCtx->width, pCodecCtx->height));
	//avpicture_fill((AVPicture *)pFrameYUV, out_buffer, AV_PIX_FMT_YUV420P, pCodecCtx->width, pCodecCtx->height);
	out_buffer := (*byte)(unsafe.Pointer(libavutil.AvMalloc(uint64(libavcodec.AvpictureGetSize(libavutil.AV_PIX_FMT_YUV420P, pCodecCtx.Width, pCodecCtx.Height)))))
	((*libavcodec.AVPicture)(unsafe.Pointer(pFrameYUV))).AvpictureFill(out_buffer, libavutil.AV_PIX_FMT_YUV420P, pCodecCtx.Width, pCodecCtx.Height)
	//SDL----------------------------
	// if sdl.SDL_Init(sdl.SDL_INIT_VIDEO|sdl.SDL_INIT_AUDIO|sdl.SDL_INIT_TIMER) != 0 {
	if sdl.SDL_Init(sdl.SDL_INIT_VIDEO) != 0 {
		fmt.Printf("Could not initialize SDL - %s\n", sdl.SDL_GetError())
		return -1
	}
	var screen_w, screen_h ffcommon.FInt = 640, 360
	var mode *sdl.SDL_DisplayMode = new(sdl.SDL_DisplayMode)
	if sdl.SDL_GetCurrentDisplayMode(0, mode) != 0 {
		fmt.Printf("SDL: could not get current display mode - exiting:%s\n", sdl.SDL_GetError())
		return -1
	}
	//Half of the Desktop's width and height.
	screen_w = mode.W / 2
	screen_h = mode.H / 2
	window := sdl.SDL_CreateWindow("Simplest FFmpeg Grab Desktop", sdl.SDL_WINDOWPOS_UNDEFINED, sdl.SDL_WINDOWPOS_UNDEFINED, screen_w, screen_h, 0)
	if window == nil {
		fmt.Printf("SDL: could not create window - exiting:%s\n", sdl.SDL_GetError())
		return -1
	}
	defer window.SDL_DestroyWindow()
	renderer := window.SDL_CreateRenderer(-1, 0)
	if renderer == nil {
		fmt.Printf("SDL: could not create renderer - exiting:%s\n", sdl.SDL_GetError())
		return -1
	}
	defer renderer.SDL_DestroyRenderer()

	texture := renderer.SDL_CreateTexture(sdl.SDL_PIXELFORMAT_YV12,
		sdl.SDL_TEXTUREACCESS_STREAMING,
		pCodecCtx.Width,
		pCodecCtx.Height)
	defer texture.SDL_DestroyTexture()
	window.SDL_ShowWindow()
	time.Sleep(2 * time.Second)

	var rect sdl.SDL_Rect
	rect.X = 0
	rect.Y = 0
	rect.W = screen_w
	rect.H = screen_h
	var rect2 sdl.SDL_Rect
	rect2.X = 0
	rect2.Y = 0
	rect2.W = mode.W
	rect2.H = mode.H

	//SDL End------------------------
	// var got_picture ffcommon.FInt

	//AVPacket *packet=(AVPacket *)av_malloc(sizeof(AVPacket));
	packet := &libavcodec.AVPacket{}

	var fp_yuv *os.File
	if OUTPUT_YUV420P != 0 {
		fp_yuv, _ = os.Create("output.yuv")
	}

	var img_convert_ctx *libswscale.SwsContext
	img_convert_ctx = libswscale.SwsGetContext(pCodecCtx.Width, pCodecCtx.Height, pCodecCtx.PixFmt, pCodecCtx.Width, pCodecCtx.Height, libavutil.AV_PIX_FMT_YUV420P, libswscale.SWS_BICUBIC, nil, nil, nil)
	//------------------------------
	//video_tid := sdl.SDL_CreateThread(sfp_refresh_thread, nil)
	//
	go sfp_refresh_thread(uintptr(0))
	//sdl.SDL_CreateThread(sfp_refresh_thread, "", uintptr(0))
	//Event Loop
	var event sdl.SDL_Event

	for {
		//Wait
		ispush = true
		event.SDL_WaitEvent()
		if event.Type == SFM_REFRESH_EVENT {
			//------------------------------
			if pFormatCtx.AvReadFrame(packet) >= 0 {
				if int32(packet.StreamIndex) == videoindex {
					if pCodecCtx.AvcodecSendPacket(packet) < 0 {
						packet.AvPacketUnref()
						continue

					}
					ret = pCodecCtx.AvcodecReceiveFrame(pFrame)
					if ret < 0 {
						fmt.Printf("Decode Error.\n")
						return -1
					}
					if ret >= 0 {
						// if got_picture != 0 {
						img_convert_ctx.SwsScale((**byte)(unsafe.Pointer(&pFrame.Data)), (*int32)(unsafe.Pointer(&pFrame.Linesize)), 0, uint32(pCodecCtx.Height), (**byte)(unsafe.Pointer(&pFrameYUV.Data)), (*int32)(unsafe.Pointer(&pFrameYUV.Linesize)))

						if OUTPUT_YUV420P != 0 {
							y_size := pCodecCtx.Width * pCodecCtx.Height
							fp_yuv.Write(ffcommon.ByteSliceFromByteP(pFrameYUV.Data[0], int(y_size)))   //Y
							fp_yuv.Write(ffcommon.ByteSliceFromByteP(pFrameYUV.Data[1], int(y_size)/4)) //U
							fp_yuv.Write(ffcommon.ByteSliceFromByteP(pFrameYUV.Data[2], int(y_size)/4)) //V
						}
						texture.SDL_UpdateYUVTexture(&rect2,
							pFrameYUV.Data[0], pFrameYUV.Linesize[0],
							pFrameYUV.Data[1], pFrameYUV.Linesize[1],
							pFrameYUV.Data[2], pFrameYUV.Linesize[2])

						renderer.SDL_RenderClear()
						renderer.SDL_RenderCopy(texture, nil, &rect)
						renderer.SDL_RenderPresent()

					}
				}
				packet.AvPacketUnref()
			} else {
				//Exit Thread
				thread_exit = 1
				fmt.Println("main 准备退出 1")
			}
		} else if event.Type == sdl.SDL_QUIT {
			thread_exit = 1
			fmt.Println("main 准备退出 2")
		} else if event.Type == SFM_BREAK_EVENT {
			fmt.Println("退出循环 3")
			break
		}

	}

	img_convert_ctx.SwsFreeContext()

	if OUTPUT_YUV420P != 0 {
		fp_yuv.Close()
	}

	sdl.SDL_Quit()

	libavutil.AvFree(uintptr(unsafe.Pointer(out_buffer)))
	libavutil.AvFree(uintptr(unsafe.Pointer(pFrame)))
	libavutil.AvFree(uintptr(unsafe.Pointer(pFrameYUV)))
	pCodecCtx.AvcodecClose()
	libavformat.AvformatCloseInput(&pFormatCtx)
	return 0
}

func main() {

	os.Setenv("Path", os.Getenv("Path")+";./lib7/windows/ffmpeg")
	ffcommon.SetAvutilPath("./lib7/windows/ffmpeg/avutil-59.dll")
	ffcommon.SetAvcodecPath("./lib7/windows/ffmpeg/avcodec-61.dll")
	ffcommon.SetAvdevicePath("./lib7/windows/ffmpeg/avdevice-61.dll")
	ffcommon.SetAvfilterPath("./lib7/windows/ffmpeg/avfilter-10.dll")
	ffcommon.SetAvformatPath("./lib7/windows/ffmpeg/avformat-61.dll")
	ffcommon.SetAvpostprocPath("./lib7/windows/ffmpeg/postproc-58.dll")
	ffcommon.SetAvswresamplePath("./lib7/windows/ffmpeg/swresample-5.dll")
	ffcommon.SetAvswscalePath("./lib7/windows/ffmpeg/swscale-8.dll")

	sdlcommon.SetSDL2Path("./lib/windows/sdl/SDL2.0.16.dll")

	genDir := "./out"
	_, err := os.Stat(genDir)
	if err != nil {
		if os.IsNotExist(err) {
			os.Mkdir(genDir, 0777) //  Everyone can read write and execute
		}
	}

	// go func() {
	// 	time.Sleep(1000)
	// 	exec.Command("./lib/ffplay.exe", "rtmp://localhost/publishlive/livestream").Output()
	// 	if err != nil {
	// 		fmt.Println("play err = ", err)
	// 	}
	// }()

	main0()
}
