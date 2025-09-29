package main

import (
	"fmt"
	"log"
	"os"
	"unsafe"

	"github.com/moonfdd/ffmpeg-go/ffcommon"
	"github.com/moonfdd/ffmpeg-go/libavcodec"
	"github.com/moonfdd/ffmpeg-go/libavutil"
)

func main() {
	// 输入文件（H.264 裸流）
	inputFilename := "input.h264"
	// 输出文件（原始YUV帧）
	outputFilename := "output.yuv"

	// 打开输入文件
	file, err := os.Open(inputFilename)
	if err != nil {
		log.Fatalf("无法打开输入文件: %v", err)
	}
	defer file.Close()

	// 创建输出文件
	outFile, err := os.Create(outputFilename)
	if err != nil {
		log.Fatalf("无法创建输出文件: %v", err)
	}
	defer outFile.Close()

	// 查找 H.264 解码器
	codec := libavcodec.AvcodecFindDecoder(libavcodec.AV_CODEC_ID_H264)
	if codec == nil {
		log.Fatal("找不到 H.264 解码器")
	}

	// 分配解码器上下文
	codecCtx := libavcodec.AvcodecAllocContext3(codec)
	if codecCtx == nil {
		log.Fatal("无法分配解码器上下文")
	}
	defer libavcodec.AvcodecFreeContext(&codecCtx)

	// 打开解码器
	if libavcodec.AvcodecOpen2(codecCtx, codec, nil) < 0 {
		log.Fatal("无法打开解码器")
	}
	defer libavcodec.AvcodecClose(codecCtx)

	// 分配 packet 和 frame
	pkt := libavcodec.AvPacketAlloc()
	if pkt == nil {
		log.Fatal("无法分配 packet")
	}
	defer libavcodec.AvPacketFree(&pkt)

	frame := libavutil.AvFrameAlloc()
	if frame == nil {
		log.Fatal("无法分配 frame")
	}
	defer libavutil.AvFrameFree(&frame)

	// 读取文件并解码
	buffer := make([]byte, 4096) // 读取缓冲区
	var fileOffset int64 = 0

	for {
		// 读取数据
		n, err := file.Read(buffer)
		if err != nil || n == 0 {
			break // 文件结束或读取错误
		}

		// 将数据填充到 AVPacket
		// 注意：实际应用中需要更复杂的逻辑来分割 NALU 单元
		libavcodec.AvNewPacket(pkt, ffcommon.Int32(n))
		copy((*[1 << 30]byte)(unsafe.Pointer(pkt.Data))[:n:n], buffer[:n])
		pkt.Size = ffcommon.Int32(n)

		// 发送 packet 到解码器
		ret := libavcodec.AvcodecSendPacket(codecCtx, pkt)
		if ret < 0 {
			log.Printf("发送 packet 失败: %s", libavutil.ErrorFromCode(ret))
			continue
		}

		// 释放当前 packet
		libavcodec.AvPacketUnref(pkt)

		// 接收解码后的帧
		for ret >= 0 {
			ret = libavcodec.AvcodecReceiveFrame(codecCtx, frame)
			if ret == -int(libavutil.EAGAIN) || ret == int(libavutil.AVERROR_EOF) {
				break
			} else if ret < 0 {
				log.Printf("解码错误: %s", libavutil.ErrorFromCode(ret))
				break
			}

			// 成功解码一帧
			fmt.Printf(
				"解码帧: width=%d, height=%d, format=%d, pts=%d\n",
				frame.Width, frame.Height, frame.Format, frame.Pts,
			)

			// 处理解码后的帧（这里简单地将YUV数据写入文件）
			// 注意：实际应用中可能需要根据像素格式进行不同的处理
			for i := ffcommon.Int32(0); i < ffcommon.Int32(frame.Height); i++ {
				line := unsafe.Slice((*byte)(unsafe.Pointer(frame.Data[0])), int(frame.Linesize[0])*int(frame.Height))
				outFile.Write(line[i*ffcommon.Int64(frame.Linesize[0]) : (i+1)*ffcommon.Int64(frame.Linesize[0])])
			}

			for i := ffcommon.Int32(0); i < ffcommon.Int32(frame.Height)/2; i++ {
				line := unsafe.Slice((*byte)(unsafe.Pointer(frame.Data[1])), int(frame.Linesize[1])*int(frame.Height)/2)
				outFile.Write(line[i*ffcommon.Int64(frame.Linesize[1]) : (i+1)*ffcommon.Int64(frame.Linesize[1])])
			}

			for i := ffcommon.Int32(0); i < ffcommon.Int32(frame.Height)/2; i++ {
				line := unsafe.Slice((*byte)(unsafe.Pointer(frame.Data[2])), int(frame.Linesize[2])*int(frame.Height)/2)
				outFile.Write(line[i*ffcommon.Int64(frame.Linesize[2]) : (i+1)*ffcommon.Int64(frame.Linesize[2])])
			}

			// 释放帧引用
			libavutil.AvFrameUnref(frame)
		}

		fileOffset += int64(n)
	}

	fmt.Println("解码完成")
}
