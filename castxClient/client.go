package castxClient

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"runtime"
	"strconv"
	"time"

	"github.com/dosgo/castX/comm"
	"github.com/gorilla/websocket"
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
		if track.Codec().MimeType == "audio/opus" {
			go func() {

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
