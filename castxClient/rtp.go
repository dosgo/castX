package castxClient

import (
	"fmt"
	"log"
	"net"
	"sync"

	"github.com/pion/sdp/v3"
	"github.com/pion/webrtc/v3"
)

// WebRTCToRTPRelay 将 WebRTC 流转发为本地 RTP 并生成 SDP
type WebRTCToRTPRelay struct {
	audioConn *net.UDPConn
	videoConn *net.UDPConn

	audioPort int
	videoPort int

	audioParams *mediaParams
	videoParams *mediaParams

	peerConnection *webrtc.PeerConnection
	started        bool
	mu             sync.Mutex
}

type mediaParams struct {
	payloadType uint8
	codecName   string
	clockRate   uint32
	channels    uint16
	fmtp        string
	ssrc        uint32
	mediaType   string
}

// Start 启动 WebRTC 连接并准备接收媒体流
func (r *WebRTCToRTPRelay) Start(config webrtc.Configuration) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.started {
		return fmt.Errorf("relay already started")
	}

	// 创建本地 UDP 端口用于音频和视频转发
	var err error
	r.audioPort, r.audioConn, err = createLocalUDPListener()
	if err != nil {
		return fmt.Errorf("audio port create error: %w", err)
	}

	r.videoPort, r.videoConn, err = createLocalUDPListener()
	if err != nil {
		r.audioConn.Close()
		return fmt.Errorf("video port create error: %w", err)
	}

	// 创建 WebRTC PeerConnection
	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		r.audioConn.Close()
		r.videoConn.Close()
		return fmt.Errorf("peer connection create error: %w", err)
	}

	r.peerConnection = peerConnection

	// 设置媒体流接收处理器
	peerConnection.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		log.Printf("Track received: ID=%s, Kind=%s, SSRC=%d\n", track.ID(), track.Kind(), track.SSRC())

		// 首次接收时初始化媒体参数
		params := &mediaParams{
			payloadType: uint8(track.PayloadType()),
			codecName:   track.Codec().MimeType,
			clockRate:   track.Codec().ClockRate,
			//	ssrc:        track.SSRC(),
			mediaType: string(track.Kind()),
		}

		if track.Kind() == webrtc.RTPCodecTypeAudio {
			// 音频特定参数
			params.channels = track.Codec().Channels
			params.fmtp = track.Codec().SDPFmtpLine
			r.audioParams = params
		} else if track.Kind() == webrtc.RTPCodecTypeVideo {
			// 视频特定参数（特别是 H.264 需要 fmtp）
			params.fmtp = track.Codec().SDPFmtpLine
			r.videoParams = params
		}

		// 启动转发协程
		go r.forwardRTPPackets(track)
	})

	peerConnection.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Printf("Connection state changed: %s\n", state.String())
	})

	r.started = true
	return nil
}

// 创建本地 UDP 监听器
func createLocalUDPListener() (int, *net.UDPConn, error) {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		return 0, nil, err
	}

	// 获取实际绑定的端口
	addr := conn.LocalAddr().(*net.UDPAddr)
	return addr.Port, conn, nil
}

// 转发 RTP 数据包到本地 UDP 端口
func (r *WebRTCToRTPRelay) forwardRTPPackets(track *webrtc.TrackRemote) {
	// 根据媒体类型确定目标连接
	var conn *net.UDPConn
	if track.Kind() == webrtc.RTPCodecTypeAudio {
		conn = r.audioConn
	} else if track.Kind() == webrtc.RTPCodecTypeVideo {
		conn = r.videoConn
	} else {
		return
	}

	buf := make([]byte, 1500) // MTU 大小
	for {
		n, _, err := track.Read(buf)
		if err != nil {
			log.Printf("Track read error: %v\n", err)
			break
		}

		// 直接转发 RTP 包 (不解析)
		_, err = conn.Write(buf[:n])
		if err != nil {
			log.Printf("UDP write error: %v\n", err)
		}
	}
}

// GenerateSDP 生成用于播放本地 RTP 流的 SDP 描述
func (r *WebRTCToRTPRelay) GenerateSDP() (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.started {
		return "", fmt.Errorf("relay not started")
	}

	// 创建 SDP 会话描述
	session := sdp.SessionDescription{
		Version: 0,
		Origin: sdp.Origin{
			Username:       "-",
			SessionID:      0,
			SessionVersion: 0,
			NetworkType:    "IN",
			AddressType:    "IP4",
			UnicastAddress: "127.0.0.1",
		},
		SessionName: "WebRTC-RTP Relay",
		ConnectionInformation: &sdp.ConnectionInformation{
			NetworkType: "IN",
			AddressType: "IP4",
			Address:     &sdp.Address{Address: "127.0.0.1"},
		},
		TimeDescriptions: []sdp.TimeDescription{
			{
				Timing: sdp.Timing{
					StartTime: 0,
					StopTime:  0,
				},
			},
		},
	}

	// 添加音频媒体描述
	if r.audioParams != nil {
		audioMedia := sdp.MediaDescription{
			MediaName: sdp.MediaName{
				Media:   "audio",
				Port:    sdp.RangedPort{Value: r.audioPort},
				Protos:  []string{"RTP", "AVP"},
				Formats: []string{string(r.audioParams.mediaType)},
			},
			ConnectionInformation: &sdp.ConnectionInformation{
				NetworkType: "IN",
				AddressType: "IP4",
				Address:     &sdp.Address{Address: "127.0.0.1"},
			},
			Attributes: []sdp.Attribute{
				{Key: "rtpmap", Value: fmt.Sprintf("%d %s/%d/%d",
					r.audioParams.payloadType,
					r.audioParams.codecName,
					r.audioParams.clockRate,
					r.audioParams.channels)},
			},
		}

		if r.audioParams.fmtp != "" {
			audioMedia.Attributes = append(audioMedia.Attributes, sdp.Attribute{
				Key: "fmtp", Value: fmt.Sprintf("%d %s",
					r.audioParams.payloadType,
					r.audioParams.fmtp),
			})
		}

		session.MediaDescriptions = append(session.MediaDescriptions, &audioMedia)
	}

	// 添加视频媒体描述
	if r.videoParams != nil {
		videoMedia := sdp.MediaDescription{
			MediaName: sdp.MediaName{
				Media:   "video",
				Port:    sdp.RangedPort{Value: r.videoPort},
				Protos:  []string{"RTP", "AVP"},
				Formats: []string{string(r.videoParams.mediaType)},
			},
			ConnectionInformation: &sdp.ConnectionInformation{
				NetworkType: "IN",
				AddressType: "IP4",
				Address:     &sdp.Address{Address: "127.0.0.1"},
			},
			Attributes: []sdp.Attribute{
				{Key: "rtpmap", Value: fmt.Sprintf("%d %s/%d",
					r.videoParams.payloadType,
					r.videoParams.codecName,
					r.videoParams.clockRate)},
			},
		}

		if r.videoParams.fmtp != "" {
			videoMedia.Attributes = append(videoMedia.Attributes, sdp.Attribute{
				Key: "fmtp", Value: fmt.Sprintf("%d %s",
					r.videoParams.payloadType,
					r.videoParams.fmtp),
			})
		}

		session.MediaDescriptions = append(session.MediaDescriptions, &videoMedia)
	}

	// 生成 SDP 字符串
	var sdpBuf []byte
	var err error
	sdpBuf, err = session.Marshal()
	if err != nil {
		return "", fmt.Errorf("SDP marshal error: %w", err)
	}

	return string(sdpBuf), nil
}

// Close 关闭所有资源
func (r *WebRTCToRTPRelay) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var errs []error

	if r.audioConn != nil {
		if err := r.audioConn.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if r.videoConn != nil {
		if err := r.videoConn.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if r.peerConnection != nil {
		if err := r.peerConnection.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	r.started = false
	return combineErrors(errs)
}

// 辅助函数：合并多个错误
func combineErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}

	var combined string
	for _, err := range errs {
		combined += err.Error() + "; "
	}
	return fmt.Errorf("errors during close: %s", combined)
}

// CreateOffer 创建 WebRTC Offer
func (r *WebRTCToRTPRelay) CreateOffer() (string, error) {
	if r.peerConnection == nil {
		return "", fmt.Errorf("peer connection not initialized")
	}

	offer, err := r.peerConnection.CreateOffer(nil)
	if err != nil {
		return "", err
	}

	err = r.peerConnection.SetLocalDescription(offer)
	if err != nil {
		return "", err
	}

	return offer.SDP, nil
}

// SetAnswer 设置远程 Answer
func (r *WebRTCToRTPRelay) SetAnswer(answerSDP string) error {
	if r.peerConnection == nil {
		return fmt.Errorf("peer connection not initialized")
	}

	answer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer,
		SDP:  answerSDP,
	}

	return r.peerConnection.SetRemoteDescription(answer)
}
