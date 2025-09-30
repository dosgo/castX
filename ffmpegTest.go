package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/dosgo/castX/castxClient"
)

func main() {
	// 创建解码器
	decoder, err := castxClient.NewH264Decoder()
	if err != nil {
		fmt.Printf("创建解码器失败: %v\n", err)
		return
	}
	defer decoder.Close()

	// 读取H.264文件到内存
	filePath := "input.h264" // 替换为您的H.264文件路径
	h264Data, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Printf("读取文件失败: %v\n", err)
		return
	}

	// 创建输出文件
	outFile, err := os.Create("output.yuv")
	if err != nil {
		fmt.Printf("创建输出文件失败: %v\n", err)
		return
	}
	defer outFile.Close()

	// 解码并写入YUV数据
	offset := 0
	frameCount := 0

	for offset < len(h264Data) {
		// 查找NAL单元起始码
		nalStart := findNALStart(h264Data[offset:])
		if nalStart < 0 {
			break
		}
		offset += nalStart

		// 查找下一个NAL单元起始码作为当前NAL单元的结束
		nextNalStart := findNALStart(h264Data[offset+4:])
		var nalEnd int
		if nextNalStart < 0 {
			nalEnd = len(h264Data)
		} else {
			nalEnd = offset + nextNalStart + 4
		}

		// 提取NAL单元数据
		nalData := h264Data[offset:nalEnd]
		offset = nalEnd

		// 解码NAL单元
		yuvData, err := decoder.DecodeFrame(nalData)
		if err != nil {
			fmt.Printf("解码失败: %v\n", err)
			continue
		}

		if yuvData != nil {
			// 写入YUV文件
			if _, err := outFile.Write(yuvData); err != nil {
				fmt.Printf("写入文件失败: %v\n", err)
			}
			frameCount++
			fmt.Printf("已解码 %d 帧\n", frameCount)
		}
	}

	fmt.Printf("解码完成，共解码 %d 帧\n", frameCount)

	// 使用ffplay播放结果
	width, height := decoder.GetResolution()
	if width > 0 && height > 0 {
		cmd := exec.Command("ffplay", "-f", "rawvideo",
			"-pixel_format", "yuv420p",
			"-video_size", fmt.Sprintf("%dx%d", width, height),
			"output.yuv")
		if err := cmd.Run(); err != nil {
			fmt.Printf("播放失败: %v\n", err)
		}
	}
}

// findNALStart 查找NAL单元起始码 (0x00000001 或 0x000001)
func findNALStart(data []byte) int {
	for i := 0; i < len(data)-4; i++ {
		if data[i] == 0 && data[i+1] == 0 {
			if data[i+2] == 1 {
				return i // 0x000001
			} else if i+3 < len(data) && data[i+2] == 0 && data[i+3] == 1 {
				return i // 0x00000001
			}
		}
	}
	return -1
}
