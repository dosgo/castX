package main

import (
	"fmt"
	"math"
	"os"
)

// AudioFrame 表示音频数据帧
type AudioFrame []float64

// WSOLA时间伸缩算法
func WSOLATimeStretch(input AudioFrame, speed float64, windowSize, searchWindow int) AudioFrame {
	if speed <= 0 {
		panic("speed must be greater than 0")
	}

	if windowSize <= 0 || searchWindow <= 0 {
		panic("invalid window size or search window")
	}

	// 创建汉宁窗
	window := createHanningWindow(windowSize)

	// 计算输出长度和步长
	outputLength := int(float64(len(input)) / speed)
	output := make(AudioFrame, outputLength+windowSize) // 额外空间防止越界

	// 计算合成步长（固定）
	synthesisHop := windowSize / 2
	// 计算分析步长（根据速度变化）
	analysisHop := int(float64(synthesisHop) * speed)

	// 初始化位置
	synthesisPos := 0
	analysisPos := 0

	for synthesisPos < len(output)-windowSize && analysisPos < len(input)-windowSize-searchWindow {
		// 1. 获取当前分析段（理论上应该的位置）
		targetSegment := getSegment(input, analysisPos, windowSize)

		// 2. 在搜索窗口内寻找最佳匹配段
		bestMatchPos := findBestMatch(input, analysisPos, targetSegment, windowSize, searchWindow)
		bestSegment := getSegment(input, bestMatchPos, windowSize)

		// 3. 应用窗函数
		windowedSegment := applyWindow(bestSegment, window)

		// 4. 重叠相加到输出
		overlapAdd(output, synthesisPos, windowedSegment, window)

		// 5. 更新位置
		synthesisPos += synthesisHop
		analysisPos += analysisHop
	}

	return output[:outputLength] // 返回精确长度的输出
}

// 创建汉宁窗
func createHanningWindow(size int) AudioFrame {
	window := make(AudioFrame, size)
	for i := 0; i < size; i++ {
		window[i] = 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/float64(size-1)))
	}
	return window
}

// 获取音频段
func getSegment(input AudioFrame, start, size int) AudioFrame {
	segment := make(AudioFrame, size)
	for i := 0; i < size; i++ {
		pos := start + i
		if pos < len(input) {
			segment[i] = input[pos]
		} else {
			segment[i] = 0
		}
	}
	return segment
}

// 寻找最佳匹配段（WSOLA核心）
func findBestMatch(input AudioFrame, centerPos int, targetSegment AudioFrame, windowSize, searchWindow int) int {
	bestPos := centerPos
	bestSimilarity := math.Inf(-1)

	// 在搜索窗口内寻找最佳匹配
	searchStart := centerPos - searchWindow/2
	searchEnd := centerPos + searchWindow/2

	if searchStart < 0 {
		searchStart = 0
	}
	if searchEnd > len(input)-windowSize {
		searchEnd = len(input) - windowSize
	}

	// 遍历搜索窗口内的所有可能位置
	for candidatePos := searchStart; candidatePos < searchEnd; candidatePos++ {
		candidateSegment := getSegment(input, candidatePos, windowSize)

		// 计算相似度（使用互相关系数）
		similarity := calculateSimilarity(targetSegment, candidateSegment)

		if similarity > bestSimilarity {
			bestSimilarity = similarity
			bestPos = candidatePos
		}
	}

	return bestPos
}

// 计算两个段的相似度（互相关系数）
func calculateSimilarity(segment1, segment2 AudioFrame) float64 {
	if len(segment1) != len(segment2) {
		return 0
	}

	// 计算互相关系数
	var sumXX, sumYY, sumXY float64

	for i := 0; i < len(segment1); i++ {
		x := segment1[i]
		y := segment2[i]
		sumXX += x * x
		sumYY += y * y
		sumXY += x * y
	}

	// 防止除以零
	if sumXX == 0 || sumYY == 0 {
		return 0
	}

	return sumXY / (math.Sqrt(sumXX) * math.Sqrt(sumYY))
}

// 应用窗函数
func applyWindow(segment AudioFrame, window AudioFrame) AudioFrame {
	result := make(AudioFrame, len(segment))
	for i := 0; i < len(segment); i++ {
		result[i] = segment[i] * window[i]
	}
	return result
}

// 重叠相加
func overlapAdd(output AudioFrame, pos int, segment AudioFrame, window AudioFrame) {
	for i := 0; i < len(segment); i++ {
		if pos+i < len(output) {
			// 使用窗函数进行平滑叠加
			output[pos+i] += segment[i]
		}
	}
}

// 生成测试信号
func generateTestSignal() AudioFrame {
	sampleRate := 44100.0
	duration := 1.0
	numSamples := int(sampleRate * duration)
	samples := make(AudioFrame, numSamples)

	// 生成包含多个频率的信号
	for i := 0; i < numSamples; i++ {
		t := float64(i) / sampleRate
		// 440Hz正弦波 + 880Hz正弦波
		samples[i] = 0.6*math.Sin(2*math.Pi*440*t) + 0.4*math.Sin(2*math.Pi*880*t)
		// 添加包络防止爆音
		envelope := 1.0
		if t < 0.1 {
			envelope = t / 0.1
		} else if t > 0.9 {
			envelope = (1.0 - t) / 0.1
		}
		samples[i] *= envelope
	}

	return samples
}

// 保存为WAV文件
func saveAsWAV(filename string, data AudioFrame, sampleRate int) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	numSamples := len(data)
	byteRate := sampleRate * 2
	fileSize := 36 + numSamples*2

	// WAV文件头
	file.Write([]byte("RIFF"))
	writeInt32(file, int32(fileSize))
	file.Write([]byte("WAVE"))
	file.Write([]byte("fmt "))
	writeInt32(file, 16)
	writeInt16(file, 1)
	writeInt16(file, 1)
	writeInt32(file, int32(sampleRate))
	writeInt32(file, int32(byteRate))
	writeInt16(file, 2)
	writeInt16(file, 16)
	file.Write([]byte("data"))
	writeInt32(file, int32(numSamples*2))

	// 音频数据
	for _, sample := range data {
		// 限制范围并归一化
		val := math.Max(-1.0, math.Min(1.0, sample))
		pcmVal := int16(val * 32767)
		file.Write([]byte{byte(pcmVal & 0xFF), byte((pcmVal >> 8) & 0xFF)})
	}

	return nil
}

func writeInt16(file *os.File, value int16) {
	file.Write([]byte{byte(value & 0xFF), byte((value >> 8) & 0xFF)})
}

func writeInt32(file *os.File, value int32) {
	file.Write([]byte{
		byte(value & 0xFF),
		byte((value >> 8) & 0xFF),
		byte((value >> 16) & 0xFF),
		byte((value >> 24) & 0xFF),
	})
}

func main() {
	fmt.Println("生成测试信号...")
	input := generateTestSignal()
	saveAsWAV("original.wav", input, 44100)

	// WSOLA时间伸缩测试
	fmt.Println("WSOLA 0.5倍减速...")
	slow := WSOLATimeStretch(input, 0.5, 2048, 512)
	saveAsWAV("slow_wsola.wav", slow, 44100)

	fmt.Println("WSOLA 1.5倍加速...")
	fast := WSOLATimeStretch(input, 1.5, 2048, 512)
	saveAsWAV("fast_wsola.wav", fast, 44100)

	fmt.Println("WSOLA 2.0倍加速...")
	faster := WSOLATimeStretch(input, 2.0, 2048, 512)
	saveAsWAV("faster_wsola.wav", faster, 44100)

	fmt.Println("处理完成！")
}
