package castxServer

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/dosgo/castX/comm"
)

type ScrcpyReceiver struct {
	listener           net.Listener
	Counter            int
	run                bool
	audioSampleRate    int
	audioLastPts       int64
	VideoType          string
	controlConnectCall func(conn net.Conn) //控制消息回调
}

// Scrcpy 协议常量
const (
	SCRCPY_HEADER_SIZE = 4  // 协议头长度
	FRAME_HEADER_SIZE  = 12 // 帧头长度
)

// 协议头结构
type FrameHeader struct {
	IsConfig   bool   // 配置包标志 (1 bit)
	IsKeyFrame bool   // 关键帧标志 (1 bit)
	PTS        uint64 // 呈现时间戳 (62 bits)
	DataLength uint32 // 数据长度
}

func readFrameHeader(conn net.Conn, headBuf []byte, header *FrameHeader) error {
	if _, err := io.ReadFull(conn, headBuf); err != nil {
		return err
	}
	// 1. 解析前8字节为BigEndian的uint64
	headerU64 := binary.BigEndian.Uint64(headBuf[0:8])

	// 2. 提取标志位
	isConfig := (headerU64 >> 63) & 0x01   // 最高位(第63位)
	isKeyFrame := (headerU64 >> 62) & 0x01 // 次高位(第62位)

	// 3. 提取PTS (低62位)
	pts := headerU64 & 0x3FFFFFFFFFFFFFFF
	header.IsConfig = isConfig == 1
	header.IsKeyFrame = isKeyFrame == 1
	header.PTS = pts
	header.DataLength = binary.BigEndian.Uint32(headBuf[8:12])
	return nil
}

// 处理音频数据（示例仅打印信息）
func (castx *Castx) handleAudio(_conn net.Conn) error {
	conn := comm.NewBufferedReadWriteCloser(_conn, 64*1024)
	data := make([]byte, 65535)
	var pts int64 = 0
	frameHeader := &FrameHeader{}
	headerBuf := make([]byte, FRAME_HEADER_SIZE)
	for castx.ScrcpyReceiver.run {
		err := readFrameHeader(conn, headerBuf, frameHeader)
		if err != nil {
			return err
		}
		n, err := io.ReadFull(conn, data[:frameHeader.DataLength])
		if err != nil {
			return err
		}
		if frameHeader.IsConfig {
			//add AOPUSHD header
			buf := new(bytes.Buffer)
			// 1. AOPUSHD 块
			buf.WriteString("AOPUSHD")                        // Magic
			binary.Write(buf, binary.LittleEndian, uint64(n)) // Length
			buf.Write(data[:n])
			opusHead := comm.ParseOpusHead(data[:n])
			castx.ScrcpyReceiver.audioSampleRate = int(opusHead.SampleRate)
			castx.WebrtcServer.SendAudio(buf.Bytes(), int64(frameHeader.PTS))
		} else {
			pts = int64(frameHeader.PTS)
			//pts = scrcpyClient.fixAudioPts(int64(h.PTS))
			castx.WebrtcServer.SendAudio(data[:n], pts)
		}
	}
	return nil
}

// 处理视频数据（保存为H264文件）
func (castx *Castx) handleVideo(_conn net.Conn) error {
	conn := comm.NewBufferedReadWriteCloser(_conn, 1024*64)
	data := make([]byte, 1024*1024*5)

	startCode := []byte{0x00, 0x00, 0x00, 0x01}

	frameHeader := &FrameHeader{}
	headerBuf := make([]byte, FRAME_HEADER_SIZE)
	var h264Sps []byte
	var h264Pps []byte
	var lastPts uint64
	var sendPts uint64
	for {
		err := readFrameHeader(conn, headerBuf, frameHeader)
		if err != nil {
			return err
		}

		if _, err := io.ReadFull(conn, data[:frameHeader.DataLength]); err != nil {
			return err
		}

		nalType := data[4] & 0x1F // 取低5位
		if nalType == 7 {
			spsPpsInfo := bytes.Split(data[:frameHeader.DataLength], startCode)
			h264Sps = append(startCode, spsPpsInfo[1]...)
			h264Pps = append(startCode, spsPpsInfo[2]...)

			sendPts = lastPts
			if frameHeader.PTS > 0 {
				sendPts = frameHeader.PTS
			}
			castx.WebrtcServer.SendVideo(h264Sps, int64(sendPts))
			castx.WebrtcServer.SendVideo(h264Pps, int64(sendPts))
			pspInfo, _ := comm.ParseSPS(data[4:], true)
			if pspInfo.Width != castx.Config.VideoWidth {
				castx.UpdateConfig(pspInfo.Width, pspInfo.Height, 0)
			}
			continue
		}
		if frameHeader.IsKeyFrame {
			castx.WebrtcServer.SendVideo(h264Sps, int64(frameHeader.PTS))
			castx.WebrtcServer.SendVideo(h264Pps, int64(frameHeader.PTS))
		}
		lastPts = frameHeader.PTS
		//fmt.Printf("SendVideo pst:%d len:%d\r\n", frameHeader.PTS, frameHeader.DataLength)
		castx.WebrtcServer.SendVideo(data[:frameHeader.DataLength], int64(frameHeader.PTS))
	}
}

// 处理单个Scrcpy连接
func (castx *Castx) handleConnection(conn net.Conn) {
	defer conn.Close()
	socketType, err := castx.readHeader(conn)
	if err != nil {
		if errors.Is(err, io.EOF) {
			fmt.Println("连接正常关闭")
			return
		}
		fmt.Printf("读取头失败: %v\n", err)
		return
	}

	// 根据数据类型处理
	switch socketType {
	case 1:
		castx.handleVideo(conn)
	case 2:
		castx.handleAudio(conn)
	case 3:
		if castx.ScrcpyReceiver.controlConnectCall != nil {
			castx.ScrcpyReceiver.controlConnectCall(conn)
		}
	default:
		fmt.Printf("未知数据类型: 0x%x\n", socketType)
		return
	}
}

func (castx *Castx) startReceiver(port int) {
	// 启动 TCP 服务器
	var err error
	castx.ScrcpyReceiver.listener, err = net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		panic(fmt.Sprintf("监听失败: %v", err))
	}
	fmt.Printf("Scrcpy 接收服务已启动，监听端口:%d...\r\n", port)
	castx.ScrcpyReceiver.Counter = 0
	castx.ScrcpyReceiver.run = true
	// 主接收循环
	go func() {
		for castx.ScrcpyReceiver.run {
			conn, err := castx.ScrcpyReceiver.listener.Accept()
			if err != nil {
				fmt.Printf("接受连接失败: %v\n", err)
				break
			}
			fmt.Printf("接收到连接: %s\n", conn.RemoteAddr()) // 打印连接信息
			//adb使用scrcpy才有第一个连接发送设备名字
			if castx.Config.UseAdb && castx.ScrcpyReceiver.Counter == 0 {
				deviceName := make([]byte, 64)
				io.ReadFull(conn, deviceName)
				fmt.Printf("设备名称:%s\r\n", deviceName)
			}
			go castx.handleConnection(conn) // 为每个连接启动goroutine
			castx.ScrcpyReceiver.Counter++
		}
	}()
}

func (castx *Castx) CloseScrcpyReceiver() {
	if castx.ScrcpyReceiver != nil && castx.ScrcpyReceiver.listener != nil {
		castx.ScrcpyReceiver.listener.Close()
	}
	if castx.ScrcpyReceiver != nil {
		castx.ScrcpyReceiver.run = false
	}
}

func (castx *Castx) fixAudioPts(_pts int64) int64 {
	if castx.ScrcpyReceiver.audioLastPts == 0 {
		castx.ScrcpyReceiver.audioLastPts = _pts
	} else {
		castx.ScrcpyReceiver.audioLastPts = castx.ScrcpyReceiver.audioLastPts + (1000000 / int64(castx.ScrcpyReceiver.audioSampleRate))
	}
	return castx.ScrcpyReceiver.audioLastPts
}

func (castx *Castx) SetControlConnectCall(_controlConnectCall func(net.Conn)) {
	castx.ScrcpyReceiver.controlConnectCall = _controlConnectCall
}

// 读取协议头
func (castx *Castx) readHeader(conn net.Conn) (int, error) {
	buf := make([]byte, 4)
	conn.SetReadDeadline(time.Now().Add(1500 * time.Millisecond))
	conn.Read(buf)
	conn.SetReadDeadline(time.Time{})
	if string(buf) == "h264" || string(buf) == "h265" || string(buf) == "av1" {
		castx.ScrcpyReceiver.VideoType = string(buf)
		paramData := make([]byte, 8)
		io.ReadFull(conn, paramData)
		videoWidth := int(binary.BigEndian.Uint32(paramData[0:4]))
		videoHeight := int(binary.BigEndian.Uint32(paramData[4:8]))
		fmt.Printf("视频width:%d\n", binary.BigEndian.Uint32(paramData[0:4]))
		fmt.Printf("视频Height:%d\n", binary.BigEndian.Uint32(paramData[4:8])) // 打印视频参数，实际使用时需要解析并处理这些参数，这里仅打印示例
		castx.UpdateConfig(videoWidth, videoHeight, 0)
		return 1, nil
	} else if string(buf) == "opus" || string(buf) == "aac" || string(buf) == "raw" {
		return 2, nil
	}
	return 3, nil
}
