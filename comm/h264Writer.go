package comm

import (
	"bytes"
	"io"
	"sync"
	"time"
)

const (
	readBufferSize = 1024 * 1024 // 1MB 读取缓冲区
	startCode      = "\x00\x00\x00\x01"
	startCodeLen   = 4
)

func NewH264Stream(webrtcServer *WebrtcServer) *H264Stream {
	return &H264Stream{
		webrtcServer: webrtcServer,
		stopChan:     make(chan struct{}),
	}
}

type H264Stream struct {
	webrtcServer *WebrtcServer
	stopChan     chan struct{}
	wg           sync.WaitGroup
}

// 启动流处理
func (s *H264Stream) ProcessStream(stream io.Reader) {
	s.wg.Add(1)
	go s.processStream(stream)
}

// 核心流处理逻辑
func (s *H264Stream) processStream(stream io.Reader) {
	defer s.wg.Done()

	// 使用固定大小的读取缓冲区
	readBuf := make([]byte, readBufferSize)
	// 用于存放未处理完的数据
	remainBuf := make([]byte, 0, 2*readBufferSize)

	for {
		select {
		case <-s.stopChan:
			return
		default:
			n, err := stream.Read(readBuf)
			if n > 0 {
				// 处理新读取的数据块
				nalUnits, newRemain := s.splitNALUnits(remainBuf, readBuf[:n])
				remainBuf = newRemain

				// 发送所有NAL单元
				for _, nal := range nalUnits {
					if len(nal) > 0 {
						s.webrtcServer.SendVideo(nal, time.Now().UnixMicro())
					}
				}
			}

			if err != nil {
				if err != io.EOF {
					// 处理错误
				}
				return
			}
		}
	}
}

// 高性能分帧函数
func (s *H264Stream) splitNALUnits(previous, newChunk []byte) ([][]byte, []byte) {
	// 合并之前的剩余数据和新的数据块
	fullData := append(previous, newChunk...)

	// 查找所有起始码位置
	positions := make([]int, 0, 16)
	searchStart := 0

	for searchStart <= len(fullData)-startCodeLen {
		idx := bytes.Index(fullData[searchStart:], []byte(startCode))
		if idx == -1 {
			break
		}

		// 计算绝对位置
		absIdx := searchStart + idx
		positions = append(positions, absIdx)
		searchStart = absIdx + startCodeLen
	}

	// 如果没有找到帧边界，返回整个数据块作为剩余数据
	if len(positions) == 0 {
		return nil, fullData
	}

	// 提取所有完整的NAL单元
	nalUnits := make([][]byte, 0, len(positions))

	// 第一个NAL单元（可能包含部分前一帧数据）
	if positions[0] > 0 {
		nalUnits = append(nalUnits, fullData[:positions[0]])
	}

	// 中间NAL单元
	for i := 0; i < len(positions)-1; i++ {
		start := positions[i] + startCodeLen
		end := positions[i+1]
		nalUnits = append(nalUnits, fullData[start:end])
	}

	// 最后一个NAL单元（可能不完整）
	lastStart := positions[len(positions)-1] + startCodeLen
	lastUnit := fullData[lastStart:]

	return nalUnits, lastUnit
}

// 停止处理
func (s *H264Stream) Stop() {
	close(s.stopChan)
	s.wg.Wait()
}
