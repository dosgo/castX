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

type Receiver struct {
	listener           net.Listener
	Counter            int
	run                bool
	audioSampleRate    int
	audioLastPts       int64
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

func readFrameHeader(conn net.Conn) (*FrameHeader, error) {
	buf := make([]byte, FRAME_HEADER_SIZE)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return nil, err
	}

	// 1. 解析前8字节为BigEndian的uint64
	headerU64 := binary.BigEndian.Uint64(buf[0:8])

	// 2. 提取标志位
	isConfig := (headerU64 >> 63) & 0x01   // 最高位(第63位)
	isKeyFrame := (headerU64 >> 62) & 0x01 // 次高位(第62位)

	// 3. 提取PTS (低62位)
	pts := headerU64 & 0x3FFFFFFFFFFFFFFF
	return &FrameHeader{
		IsConfig:   isConfig == 1,
		IsKeyFrame: isKeyFrame == 1,
		PTS:        pts,
		DataLength: binary.BigEndian.Uint32(buf[8:12]),
	}, nil
}

// 处理音频数据（示例仅打印信息）
func (castx *Castx) handleAudio(conn net.Conn) error {
	data := make([]byte, 65535)
	var pts int64 = 0

	for castx.Receiver.run {
		h, err := readFrameHeader(conn)
		if err != nil {
			return err
		}
		n, err := io.ReadFull(conn, data[:h.DataLength])
		if err != nil {
			return err
		}
		if h.IsConfig {
			//add AOPUSHD header
			buf := new(bytes.Buffer)
			// 1. AOPUSHD 块
			buf.WriteString("AOPUSHD")                        // Magic
			binary.Write(buf, binary.LittleEndian, uint64(n)) // Length
			buf.Write(data[:n])
			opusHead := comm.ParseOpusHead(data[:n])
			castx.Receiver.audioSampleRate = int(opusHead.SampleRate)
			castx.WebrtcServer.SendAudio(buf.Bytes(), int64(h.PTS))
		} else {
			pts = int64(h.PTS)
			//pts = scrcpyClient.fixAudioPts(int64(h.PTS))
			castx.WebrtcServer.SendAudio(data[:n], pts)
		}
	}
	return nil
}

// 处理视频数据（保存为H264文件）
func (castx *Castx) handleVideo(conn net.Conn) error {
	data := make([]byte, 1024*1024*5)
	sps := make([]byte, 0)
	pps := make([]byte, 0)
	startCode := []byte{0x00, 0x00, 0x00, 0x01}
	for {
		h, err := readFrameHeader(conn)
		if err != nil {
			return err
		}

		if _, err := io.ReadFull(conn, data[:h.DataLength]); err != nil {
			return err
		}

		nalType := data[4] & 0x1F // 取低5位
		if nalType == 7 {
			spsPpsInfo := bytes.Split(data[:h.DataLength], startCode)
			sps = append([]byte{}, spsPpsInfo[1]...)
			pps = append([]byte{}, spsPpsInfo[2]...)
			castx.WebrtcServer.SendVideo(append(startCode, sps...), int64(h.PTS))
			castx.WebrtcServer.SendVideo(append(startCode, pps...), int64(h.PTS))
			pspInfo, _ := comm.ParseSPS(sps)

			if pspInfo.Width != castx.Config.ScreenWidth {
				castx.UpdateConfig(pspInfo.Width, pspInfo.Height, pspInfo.Width, pspInfo.Height, 0)
			}
			continue
		}
		if h.IsKeyFrame {
			castx.WebrtcServer.SendVideo(append(startCode, sps...), int64(h.PTS))
			castx.WebrtcServer.SendVideo(append(startCode, pps...), int64(h.PTS))
			// 打印关键帧信息，实际使用时可以根据需要进行处理，这里仅打印示例
		}
		castx.WebrtcServer.SendVideo(data[:h.DataLength], int64(h.PTS))
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
		if castx.Receiver.controlConnectCall != nil {
			castx.Receiver.controlConnectCall(conn)
		}
	default:
		fmt.Printf("未知数据类型: 0x%x\n", socketType)
		return
	}
}

func (castx *Castx) startReceiver(port int) {
	// 启动 TCP 服务器
	var err error
	castx.Receiver.listener, err = net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		panic(fmt.Sprintf("监听失败: %v", err))
	}
	fmt.Println("Scrcpy 接收服务已启动，监听端口:%d...", port)
	castx.Receiver.Counter = 0
	castx.Receiver.run = true
	// 主接收循环
	go func() {
		for castx.Receiver.run {
			conn, err := castx.Receiver.listener.Accept()
			if err != nil {
				fmt.Printf("接受连接失败: %v\n", err)
				continue
			}
			fmt.Printf("接收到连接: %s\n", conn.RemoteAddr()) // 打印连接信息
			//adb使用scrcpy才有第一个连接发送设备名字
			if castx.Config.UseAdb && castx.Receiver.Counter == 0 {
				deviceName := make([]byte, 64)
				io.ReadFull(conn, deviceName)
				fmt.Printf("设备名称:%s\r\n", deviceName)
			}
			go castx.handleConnection(conn) // 为每个连接启动goroutine
			castx.Receiver.Counter++
		}
	}()
}

func (castx *Castx) CloseReceiver() {
	if castx.Receiver != nil && castx.Receiver.listener != nil {
		castx.Receiver.listener.Close()
	}
	if castx.Receiver != nil {
		castx.Receiver.run = false
	}
}

func (castx *Castx) fixAudioPts(_pts int64) int64 {
	if castx.Receiver.audioLastPts == 0 {
		castx.Receiver.audioLastPts = _pts
	} else {
		castx.Receiver.audioLastPts = castx.Receiver.audioLastPts + (1000000 / int64(castx.Receiver.audioSampleRate))
	}
	return castx.Receiver.audioLastPts
}

func (castx *Castx) SetControlConnectCall(_controlConnectCall func(net.Conn)) {
	castx.Receiver.controlConnectCall = _controlConnectCall
}

// 读取协议头
func (castx *Castx) readHeader(conn net.Conn) (int, error) {
	buf := make([]byte, 4)
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	conn.Read(buf)
	conn.SetReadDeadline(time.Time{})
	if string(buf) == "h264" || string(buf) == "h265" || string(buf) == "av1" {
		paramData := make([]byte, 8)
		io.ReadFull(conn, paramData)
		videoWidth := int(binary.BigEndian.Uint32(paramData[0:4]))
		videoHeight := int(binary.BigEndian.Uint32(paramData[4:8]))
		fmt.Printf("视频width:%d\n", binary.BigEndian.Uint32(paramData[0:4]))
		fmt.Printf("视频Height:%d\n", binary.BigEndian.Uint32(paramData[4:8])) // 打印视频参数，实际使用时需要解析并处理这些参数，这里仅打印示例
		castx.UpdateConfig(videoWidth, videoHeight, videoWidth, videoHeight, 0)
		return 1, nil
	} else if string(buf) == "opus" || string(buf) == "aac" || string(buf) == "raw" {
		return 2, nil
	}
	return 3, nil
}
