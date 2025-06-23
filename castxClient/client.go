package castxClient

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/dosgo/castX/comm"
	"github.com/gorilla/websocket"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3"
	"github.com/wlynxg/anet"
)

type CastXClient struct {
	wsConn         *websocket.Conn
	isAuth         bool
	securityKey    string
	peerConnection *webrtc.PeerConnection
}

func (client *CastXClient) Start(wsUrl string, password string, maxSize int) error {
	var err error
	client.wsConn, _, err = websocket.DefaultDialer.Dial(wsUrl, nil)
	if err != nil {
		return err
	}
	// 消息接收协程
	go client.WsRecv(password, maxSize)
	return nil
}

func (client *CastXClient) login(password string, maxSize int) {
	timestamp := time.Now().UnixMilli()
	var srcData = client.securityKey + "|" + strconv.FormatInt(int64(timestamp), 10) + "|" + password
	sum := sha256.Sum256([]byte(srcData))
	token := hex.EncodeToString(sum[:])
	args := map[string]interface{}{
		"maxSize":   maxSize,
		"token":     token,
		"timestamp": timestamp,
	}
	argsStr, _ := json.Marshal(args)
	fmt.Print("asksStr:")
	//登录
	client.wsConn.WriteJSON(comm.WSMessage{
		Type: comm.MsgTypeLoginAuth,
		Data: string(argsStr),
	})
}
func (client *CastXClient) WsRecv(password string, maxSize int) {
	var msg comm.WSMessage
	defer func() {
		fmt.Printf("ws closed\r\n")
	}()
	for {
		err := client.wsConn.ReadJSON(&msg)
		if err != nil {
			log.Println("read error:", err)
			return
		}
		log.Printf("received: %s", msg)
		switch msg.Type {
		case comm.MsgTypeLoginAuthResp:
			data := msg.Data.(map[string]interface{})
			if data["auth"].(bool) {
				client.isAuth = true
				client.StartWebRtcReceive()
			}
		case comm.MsgTypeInitConfig:
			data := msg.Data.(map[string]interface{})
			client.securityKey = data["securityKey"].(string)
			client.login(password, maxSize)
		case comm.MsgTypeOfferResp:
			data := msg.Data.(map[string]interface{})
			answerStr, _ := json.Marshal(data["sdp"])
			var answer webrtc.SessionDescription
			json.NewDecoder(bytes.NewBuffer([]byte(answerStr))).Decode(&answer)
			// 设置远程描述
			if err = client.peerConnection.SetRemoteDescription(answer); err != nil {
				fmt.Printf("StartWebRtcReceive err:%+v\n", err)
			}
		}

	}
}

func (client *CastXClient) StartWebRtcReceive() error {
	if runtime.GOOS == "android" {
		anet.SetAndroidVersion(14)
	}
	depacketizer := NewH264Depacketizer()
	// WebRTC配置
	config := webrtc.Configuration{}
	// 创建PeerConnection
	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		fmt.Printf("StartWebRtcReceive err:%+v\n", err)
		return err
	}
	if _, err = peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo); err != nil {
		fmt.Printf("StartWebRtcReceive err:%+v\n", err)
		return err
	}
	if _, err = peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio); err != nil {
		fmt.Printf("StartWebRtcReceive err:%+v\n", err)
		return err
	}
	// 设置视频轨道处理
	peerConnection.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		fmt.Printf("接收到 %s 轨道\n", track.Kind())
		// 创建内存缓冲区
		fmt.Printf("开始接收轨道: %s\n", track.Codec().MimeType)
		if track.Codec().MimeType == "video/H264" {
			go func() {
				for {
					rtpPacket, _, err := track.ReadRTP()
					if err != nil {
						break
					}
					//comm.ProcessNalUnit(rtpPacket.Payload)
					depacketizer.ProcessRTP(rtpPacket)
				}
			}()
		}
	})
	gatherCompletePromise := webrtc.GatheringCompletePromise(peerConnection)
	// 创建Offer
	offer, err := peerConnection.CreateOffer(nil)
	if err != nil {
		fmt.Printf("StartWebRtcReceive err:%+v\n", err)
		return err
	}
	fmt.Printf("StartWebRtcReceive4\r\n")
	// 设置本地描述
	if err = peerConnection.SetLocalDescription(offer); err != nil {
		return err
	}
	client.peerConnection = peerConnection
	<-gatherCompletePromise
	// 发送Offer到信令服务
	client.getOffer(*peerConnection.LocalDescription())
	return nil
}

// 信令交互
func (client *CastXClient) getOffer(offer webrtc.SessionDescription) {
	offerJSON, _ := json.Marshal(offer)
	client.wsConn.WriteJSON(comm.WSMessage{
		Type: comm.MsgTypeOffer,
		Data: string(offerJSON),
	})
}

type H264Depacketizer struct {
	file           *os.File
	sps            []byte
	pps            []byte
	fragmentBuffer []byte
	lastTimestamp  uint32
	mu             sync.Mutex
}

func NewH264Depacketizer() *H264Depacketizer {
	h264Decode := &H264Depacketizer{}
	return h264Decode
}

func (d *H264Depacketizer) ProcessRTP(pkt *rtp.Packet) {
	d.mu.Lock()
	defer d.mu.Unlock()

	payload := pkt.Payload
	if len(payload) < 1 {
		return
	}

	// 处理分片单元
	naluType := payload[0] & 0x1F
	switch {
	case naluType >= 1 && naluType <= 23:
		d.writeNALU(payload, int64(pkt.Timestamp))
	case naluType == 28: // FU-A分片
		d.processFUA(payload, pkt.Timestamp)
	case naluType == 24: // STAP-A聚合包
		d.processSTAPA(payload, pkt.Timestamp)
	}
}

func (d *H264Depacketizer) processFUA(payload []byte, timestamp uint32) {
	if len(payload) < 2 {
		return
	}

	fuHeader := payload[1]
	start := (fuHeader & 0x80) != 0
	end := (fuHeader & 0x40) != 0

	nalType := fuHeader & 0x1F
	naluHeader := (payload[0] & 0xE0) | nalType

	if start {
		d.fragmentBuffer = []byte{naluHeader}
		d.fragmentBuffer = append(d.fragmentBuffer, payload[2:]...)
		d.lastTimestamp = timestamp
	} else if timestamp == d.lastTimestamp {
		d.fragmentBuffer = append(d.fragmentBuffer, payload[2:]...)
	}

	if end {
		if d.fragmentBuffer != nil {
			d.writeNALU(d.fragmentBuffer, int64(timestamp))
			d.fragmentBuffer = nil
		}
	}
}

func (d *H264Depacketizer) processSTAPA(payload []byte, timestamp uint32) {
	offset := 1

	for offset < len(payload) {
		if offset+2 > len(payload) {
			break
		}

		size := int(binary.BigEndian.Uint16(payload[offset:]))
		offset += 2

		if offset+size > len(payload) {
			break
		}

		d.writeNALU(payload[offset:offset+size], int64(timestamp))
		offset += size
	}
}

func (d *H264Depacketizer) writeNALU(nalu []byte, timestamp int64) {
	naluType := nalu[0] & 0x1F
	//startCode := []byte{0x00, 0x00, 0x00, 0x01}
	// 提取参数集
	switch naluType {
	case 7: // SPS
		d.sps = append([]byte{}, nalu...)

		fmt.Printf("Got SPS: %s\n", hex.EncodeToString(nalu))
	case 8: // PPS

		d.pps = append([]byte{}, nalu...)

		fmt.Printf("Got PPS: %s\n", hex.EncodeToString(nalu))

	}

	// 实时解码示例（需实现解码器接口）
	if naluType == 1 || naluType == 5 {
		fmt.Printf("h264 data\r\n")
	}
}
