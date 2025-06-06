package scrcpy

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"time"

	"github.com/dosgo/castX/castxServer"
)

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

func mtRand(min int, max int) int {
	return rand.Intn(max-min+1) + min
}

type ScrcpyClient struct {
	port            int
	videoWidth      int
	videoHeight     int
	audioSampleRate int
	audioLastPts    int64
	deviceName      []byte
	controlConn     net.Conn
	listener        net.Listener
	counter         int
	castx           *castxServer.Castx
}

func NewScrcpyClient(webPort int, peerName string, savaPath string, password string) *ScrcpyClient {
	scrcpyClient := &ScrcpyClient{}
	scrcpyClient.deviceName = make([]byte, 64)
	scrcpyClient.castx, _ = castxServer.Start(webPort, 0, 0, "", true, password)
	scrcpyClient.port = scrcpyClient.InitAdb(peerName, savaPath)
	return scrcpyClient
}
func (scrcpyClient *ScrcpyClient) getControlConn() net.Conn {
	return scrcpyClient.controlConn
}

func (scrcpyClient *ScrcpyClient) StartClient() {
	scrcpyClient.castx.WsServer.SetControlFun(func(controlData map[string]interface{}) {
		controlConn := scrcpyClient.getControlConn()
		if controlConn != nil {
			controlCall(controlConn, scrcpyClient.castx.Config, controlData)
		}
	})

	// 启动 TCP 服务器
	var err error
	scrcpyClient.listener, err = net.Listen("tcp", fmt.Sprintf(":%d", scrcpyClient.port))
	if err != nil {
		panic(fmt.Sprintf("监听失败: %v", err))
	}
	fmt.Println("Scrcpy 接收服务已启动，监听端口:%d...", scrcpyClient.port)

	// 主接收循环
	go func() {
		for {
			conn, err := scrcpyClient.listener.Accept()
			if err != nil {
				fmt.Printf("接受连接失败: %v\n", err)
				continue
			}
			fmt.Printf("接收到连接: %s\n", conn.RemoteAddr()) // 打印连接信息
			if scrcpyClient.counter == 0 {
				io.ReadFull(conn, scrcpyClient.deviceName)
				fmt.Printf("设备名称:%s\r\n", scrcpyClient.deviceName)
			}

			go scrcpyClient.handleConnection(conn) // 为每个连接启动goroutine
			scrcpyClient.counter++
		}
	}()
}

func (scrcpyClient *ScrcpyClient) Shutdown() {
	if scrcpyClient.listener != nil {
		scrcpyClient.listener.Close()
		scrcpyClient.listener = nil
	}
	if scrcpyClient.castx != nil {
		scrcpyClient.castx.HttpServer.Shutdown()
	}
	if scrcpyClient.castx.WsServer != nil {
		scrcpyClient.castx.WsServer.Shutdown()
	}
}

// 处理单个Scrcpy连接
func (scrcpyClient *ScrcpyClient) handleConnection(conn net.Conn) {
	defer conn.Close()
	socketType, err := scrcpyClient.readHeader(conn)
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
		scrcpyClient.handleVideo(conn)
	case 2:
		scrcpyClient.handleAudio(conn)
	case 3:
		handleControl(conn)
	default:
		fmt.Printf("未知数据类型: 0x%x\n", socketType)
		return
	}
}

// 读取协议头
func (scrcpyClient *ScrcpyClient) readHeader(conn net.Conn) (int, error) {
	buf := make([]byte, 4)
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	conn.Read(buf)
	conn.SetReadDeadline(time.Time{})
	if string(buf) == "h264" || string(buf) == "h265" || string(buf) == "av1" {
		paramData := make([]byte, 8)
		io.ReadFull(conn, paramData)
		scrcpyClient.videoWidth = int(binary.BigEndian.Uint32(paramData[0:4]))
		scrcpyClient.videoHeight = int(binary.BigEndian.Uint32(paramData[4:8]))
		fmt.Printf("视频width:%d\n", binary.BigEndian.Uint32(paramData[0:4]))
		fmt.Printf("视频Height:%d\n", binary.BigEndian.Uint32(paramData[4:8])) // 打印视频参数，实际使用时需要解析并处理这些参数，这里仅打印示例
		scrcpyClient.castx.UpdateConfig(scrcpyClient.videoWidth, scrcpyClient.videoHeight, scrcpyClient.videoWidth, scrcpyClient.videoHeight, 0)
		return 1, nil
	} else if string(buf) == "opus" || string(buf) == "aac" || string(buf) == "raw" {
		return 2, nil
	}
	scrcpyClient.controlConn = conn
	return 3, nil
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

// 处理视频数据（保存为H264文件）
func (scrcpyClient *ScrcpyClient) handleVideo(conn net.Conn) error {
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
			scrcpyClient.castx.WebrtcServer.SendVideo(append(startCode, sps...), int64(h.PTS))
			scrcpyClient.castx.WebrtcServer.SendVideo(append(startCode, pps...), int64(h.PTS))
			pspInfo, _ := ParseSPS(sps)

			if pspInfo.Width != scrcpyClient.castx.Config.ScreenWidth {
				scrcpyClient.castx.UpdateConfig(pspInfo.Width, pspInfo.Height, pspInfo.Width, pspInfo.Height, 0)
			}
			continue
		}
		if h.IsKeyFrame {
			scrcpyClient.castx.WebrtcServer.SendVideo(append(startCode, sps...), int64(h.PTS))
			scrcpyClient.castx.WebrtcServer.SendVideo(append(startCode, pps...), int64(h.PTS))
			// 打印关键帧信息，实际使用时可以根据需要进行处理，这里仅打印示例
		}
		scrcpyClient.castx.WebrtcServer.SendVideo(data[:h.DataLength], int64(h.PTS))
	}
}

// 处理音频数据（示例仅打印信息）
func (scrcpyClient *ScrcpyClient) handleAudio(conn net.Conn) error {
	data := make([]byte, 65535)
	var pts int64 = 0

	for {
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
			opusHead := parseOpusHead(data[:n])
			scrcpyClient.audioSampleRate = int(opusHead.SampleRate)
			scrcpyClient.castx.WebrtcServer.SendAudio(buf.Bytes(), int64(h.PTS))
		} else {
			pts = int64(h.PTS)
			//pts = scrcpyClient.fixAudioPts(int64(h.PTS))
			scrcpyClient.castx.WebrtcServer.SendAudio(data[:n], pts)
		}
	}
	return nil
}

func (scrcpyClient *ScrcpyClient) fixAudioPts(_pts int64) int64 {
	if scrcpyClient.audioLastPts == 0 {
		scrcpyClient.audioLastPts = _pts
	} else {
		scrcpyClient.audioLastPts = scrcpyClient.audioLastPts + (1000000 / int64(scrcpyClient.audioSampleRate))
	}
	return scrcpyClient.audioLastPts
}

// 处理控制数据（示例解析基本控制指令）
func handleControl(conn net.Conn) error {
	data := make([]byte, 1) // 创建1字节长度的切片
	for {

		n, err := conn.Read(data)
		if err != nil {
			fmt.Printf("handleControl err:%+v\n", err)
			return err
		}
		// 示例解析：第一个字节为事件类型
		if n < 1 {
			return errors.New("无效控制数据")
		}

		switch int(data[0]) {
		case TYPE_CLIPBOARD: //剪贴板变化
			var lenData = make([]byte, 4)
			io.ReadFull(conn, lenData)
			len := binary.BigEndian.Uint32(lenData)
			//剪贴板数据
			var clipboardData = make([]byte, len)
			io.ReadFull(conn, clipboardData)
		case TYPE_ACK_CLIPBOARD: //剪贴板变化确认:
			var lenData = make([]byte, 8)
			io.ReadFull(conn, lenData)
		case TYPE_UHID_OUTPUT:

			var idData = make([]byte, 2)
			io.ReadFull(conn, idData)

			var lenData = make([]byte, 2)
			io.ReadFull(conn, lenData)
			len := binary.BigEndian.Uint16(lenData)
			var clipboardData = make([]byte, len)
			io.ReadFull(conn, clipboardData)
		default:
			fmt.Printf("未知device类型: 0x%x\n", data[0])
		}
	}
}

type OpusHead struct {
	Magic      [8]byte
	Version    byte
	Channels   byte
	PreSkip    uint16
	SampleRate uint32
	OutputGain int16 // 注意：有符号
	Mapping    byte
}

func createOpusHeader() []byte {
	// [65, 79, 80, 85, 83, 72, 68, 82, 19, 0, 0, 0, 0, 0, 0, 0, 79, 112, 117, 115, 72, 101, 97, 100, 1, 2, 56, 1, -128, -69, 0, 0, 0, 0, 0, 65, 79, 80, 85, 83, 68, 76, 89, 8, 0, 0, 0, 0, 0, 0, 0, -96, 46, 99, 0, 0, 0, 0, 0, 65, 79, 80, 85, 83, 80, 82, 76, 8, 0, 0, 0, 0, 0, 0, 0, 0, -76, -60, 4, 0, 0, 0, 0]

	buf := new(bytes.Buffer)
	// 1. AOPUSHD 块
	buf.WriteString("AOPUSHD")                         // Magic
	binary.Write(buf, binary.LittleEndian, uint64(19)) // Length
	opusHead := OpusHead{
		Magic:      [8]byte{'O', 'p', 'u', 's', 'H', 'e', 'a', 'd'},
		Version:    1,
		Channels:   2,
		PreSkip:    312,   // 0x0138
		SampleRate: 48000, // 0x0000BB80
	}
	binary.Write(buf, binary.LittleEndian, opusHead.Magic)
	binary.Write(buf, binary.LittleEndian, opusHead.Version)
	binary.Write(buf, binary.LittleEndian, opusHead.Channels)
	binary.Write(buf, binary.LittleEndian, opusHead.PreSkip)
	binary.Write(buf, binary.LittleEndian, opusHead.SampleRate)
	binary.Write(buf, binary.LittleEndian, []byte{0, 0, 0}) // OutputGain+Mapping

	// 2. AOPUSDLY 块
	buf.WriteString("AOPUSDLY")                             // Magic
	binary.Write(buf, binary.LittleEndian, uint64(8))       // Length
	binary.Write(buf, binary.LittleEndian, uint64(6500000)) // Data

	// 3. AOPUSPRL 块
	buf.WriteString("AOPUSPRL")                              // Magic
	binary.Write(buf, binary.LittleEndian, uint64(8))        // Length
	binary.Write(buf, binary.LittleEndian, uint64(80000000)) // Data

	return buf.Bytes()
}

func parseOpusHead(data []byte) *OpusHead {
	var head OpusHead
	r := bytes.NewReader(data)

	binary.Read(r, binary.LittleEndian, &head.Magic)
	binary.Read(r, binary.LittleEndian, &head.Version)
	binary.Read(r, binary.LittleEndian, &head.Channels)
	binary.Read(r, binary.LittleEndian, &head.PreSkip)
	binary.Read(r, binary.LittleEndian, &head.SampleRate)
	binary.Read(r, binary.LittleEndian, &head.OutputGain)
	binary.Read(r, binary.LittleEndian, &head.Mapping)
	fmt.Printf("[OpusHead]\n")
	fmt.Printf("  Version:    %d\n", head.Version)
	fmt.Printf("  Channels:   %d\n", head.Channels)
	fmt.Printf("  PreSkip:    %d samples\n", head.PreSkip)
	fmt.Printf("  SampleRate: %d Hz\n", head.SampleRate)
	fmt.Printf("  OutputGain: %d dB\n", head.OutputGain)
	fmt.Printf("  Mapping:    %d\n", head.Mapping)
	return &head
}
