package comm

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
	"github.com/wlynxg/anet"
)

type WebrtcServer struct {
	lastVideoTimestamp          int64
	lastAudioTimestamp          int64
	webRtcConnectionStateChange func(int)
	outboundVideoTrack          *webrtc.TrackLocalStaticSample
	outboundAudioTrack          *webrtc.TrackLocalStaticSample
	peerConnectionCount         int64
}

func (webrtcServer *WebrtcServer) SetWebRtcConnectionStateChange(_webRtcConnectionStateChange func(int)) {
	webrtcServer.webRtcConnectionStateChange = _webRtcConnectionStateChange
}

func (webrtcServer *WebrtcServer) SendVideo(nal []byte, timestamp int64) {
	var duration time.Duration = 0
	if webrtcServer.lastVideoTimestamp == 0 {
		duration = time.Second / 40
	} else {
		duration = time.Duration(timestamp-webrtcServer.lastVideoTimestamp) * time.Microsecond
	}
	webrtcServer.lastVideoTimestamp = timestamp
	webrtcServer.SendWebrtc(nal, timestamp, duration, false)
}
func (webrtcServer *WebrtcServer) SendAudio(nal []byte, timestamp int64) {
	var duration time.Duration = 0
	if webrtcServer.lastAudioTimestamp == 0 {
		duration = time.Second / 40
	} else {
		duration = time.Duration(timestamp-webrtcServer.lastAudioTimestamp) * time.Microsecond
	}
	webrtcServer.lastAudioTimestamp = timestamp
	webrtcServer.SendWebrtc(nal, timestamp, duration, true)
}

func (webrtcServer *WebrtcServer) SendWebrtc(data []byte, timestamp int64, duration time.Duration, audio bool) error {
	var err error
	if audio {
		err = webrtcServer.outboundAudioTrack.WriteSample(media.Sample{
			Data:      data,
			Duration:  duration,
			Timestamp: time.UnixMicro(timestamp),
		})
	} else {
		err = webrtcServer.outboundVideoTrack.WriteSample(media.Sample{
			Data:      data,
			Duration:  duration,
			Timestamp: time.UnixMicro(timestamp),
		})
	}
	return err
}

// HTTP Handler that accepts an Offer and returns an Answer
// adds outboundVideoTrack to PeerConnection
func (webrtcServer *WebrtcServer) getSdp(r io.Reader) (*webrtc.SessionDescription, error) {
	if runtime.GOOS == "android" {
		anet.SetAndroidVersion(14)
	}
	peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		return nil, err
	}
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		if connectionState == webrtc.ICEConnectionStateDisconnected {
			atomic.AddInt64(&webrtcServer.peerConnectionCount, -1)
			if webrtcServer.webRtcConnectionStateChange != nil {
				webrtcServer.webRtcConnectionStateChange(int(webrtcServer.peerConnectionCount))
			}
		} else if connectionState == webrtc.ICEConnectionStateConnected {
			atomic.AddInt64(&webrtcServer.peerConnectionCount, 1)
			if webrtcServer.webRtcConnectionStateChange != nil {
				webrtcServer.webRtcConnectionStateChange(int(webrtcServer.peerConnectionCount))
			}
		}
	})
	//添加视频
	if _, err = peerConnection.AddTrack(webrtcServer.outboundVideoTrack); err != nil {
		return nil, err
	}
	//添加音频
	if _, err = peerConnection.AddTrack(webrtcServer.outboundAudioTrack); err != nil {
		return nil, err
	}

	var offer webrtc.SessionDescription
	if err = json.NewDecoder(r).Decode(&offer); err != nil {
		return nil, err
	}
	if err = peerConnection.SetRemoteDescription(offer); err != nil {
		return nil, err
	}
	gatherCompletePromise := webrtc.GatheringCompletePromise(peerConnection)

	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		return nil, err
	} else if err = peerConnection.SetLocalDescription(answer); err != nil {
		return nil, err
	}
	<-gatherCompletePromise
	return peerConnection.LocalDescription(), nil
}

func NewWebRtc(mimeType string) (*WebrtcServer, error) {
	var err error
	webrtcServer := &WebrtcServer{}
	//视频轨道
	webrtcServer.outboundVideoTrack, err = webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{
		MimeType: mimeType,
	}, "screens", "screens")
	if err != nil {
		return nil, err
	}
	//音频轨道
	webrtcServer.outboundAudioTrack, err = webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{
		MimeType:  "audio/opus",
		ClockRate: 48000, // Opus标准采样率
		Channels:  2,     // 立体声
	}, "audio", "screens")
	if err != nil {
		return nil, err
	}
	return webrtcServer, nil
}

var nalTypeMap = map[byte]string{
	0:  "UNSPECIFIED (未定义)",      // 未定义或保留类型
	1:  "SLICE_NON_IDR (非关键帧切片)", // 普通编码片（P/B帧，非IDR）
	2:  "DPA (数据分区A)",            // 数据分区A（含关键宏块头信息）
	3:  "DPB (数据分区B)",            // 数据分区B（含帧内编码宏块数据）
	4:  "DPC (数据分区C)",            // 数据分区C（含帧间编码宏块数据）
	5:  "SLICE_IDR (关键帧切片)",      // IDR帧切片（解码器重置点）
	6:  "SEI (补充增强信息)",           // 携带时间戳、版权信息等元数据
	7:  "SPS (序列参数集)",            // 视频分辨率、帧率等全局解码参数
	8:  "PPS (图像参数集)",            // 量化表、熵编码模式等帧级参数
	9:  "AUD (访问单元分隔符)",          // 视频帧边界标记（用于流分割）
	10: "END_OF_SEQ (序列结束符)",     // 视频序列结束标志
	11: "END_OF_STREAM (流结束符)",   // 视频流结束标志
	12: "FILLER (填充数据)",          // 网络对齐或占位数据（无意义字节）
	13: "SPS_EXT (SPS扩展)",        // SPS扩展信息（用于高级编码配置）
	14: "PREFIX_NAL (前缀单元)",      // MVC/SVC扩展前缀信息
	15: "SUB_SPS (子序列参数集)",       // 分层编码（如SVC）的子序列参数
	16: "DPS (解码参数集)",            // H.264扩展（如MVC）的解码参数
	17: "AUX_SLICE (辅助编码片)",      // 辅助编码切片（冗余流、深度图等）
	18: "EXTENSION_SLICE (扩展切片)", // 扩展编码切片（保留用途）
	19: "AUX_CODED_PIC (辅助编码画面)", // 辅助编码画面（如3D视频深度信息）
	// 20-23: 保留类型
	24: "STAP_A (单一时间聚合包A)",  // RTP封装：聚合多个单时间戳NALU
	25: "STAP_B (单一时间聚合包B)",  // RTP封装：带时间戳偏移的聚合包
	26: "MTAP16 (多时间聚合包16位)", // RTP封装：多时间戳聚合包（16位偏移）
	27: "MTAP24 (多时间聚合包24位)", // RTP封装：多时间戳聚合包（24位偏移）
	28: "FU_A (分片单元A)",       // RTP分片传输：分片单元开始/中间部分
	29: "FU_B (分片单元B)",       // RTP分片传输：分片单元结束部分
	// 30-31: 保留类型
}

func ProcessNalUnit(data []byte) {
	if len(data) == 0 {
		return
	}

	nalHeader := data[0]
	nalType := nalHeader & 0x1F // 取低5位

	name, ok := nalTypeMap[nalType]
	if !ok {
		name = "UNKNOWN"
	}

	fmt.Printf("  Type    : %d (%s)\n", nalType, name)
	fmt.Printf("  Size    : %d bytes\n", len(data))
	fmt.Printf("  Header  : %s\n", hex.EncodeToString(data[:1]))
	if nalType == 7 || nalType == 8 {
		fmt.Printf("  Payload : %s...\n", hex.EncodeToString(data[1:5]))
	}
	fmt.Println("--------------------------")
}
