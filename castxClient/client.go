package castxClient

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/dosgo/castX/comm"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
)

type CastXClient struct {
	wsConn         *websocket.Conn
	isAuth         bool
	securityKey    string
	listener       net.Listener
	run            bool
	videomu        sync.RWMutex
	videoConn      map[string]net.Conn
	audioConn      map[string]net.Conn
	audiomu        sync.RWMutex
	peerConnection *webrtc.PeerConnection
}

func (client *CastXClient) Start(wsUrl string, password string, maxSize int) int {
	var err error
	client.wsConn, _, err = websocket.DefaultDialer.Dial(wsUrl, nil)
	if err != nil {
		return 0
	}
	//init websrc
	client.initWebRtc()
	// 消息接收协程
	go client.WsRecv(password, maxSize)
	go client.startSendListen()
	// 获取实际分配的端口
	if client.listener != nil {
		addr := client.listener.Addr().(*net.TCPAddr)
		return addr.Port
	}
	return 0
}

func (client *CastXClient) Shutdown() {
	client.run = false
	if client.listener != nil {
		client.listener.Close()
	}
	for key, _ := range client.videoConn {
		delete(client.videoConn, key)
	}
	for key, _ := range client.audioConn {
		delete(client.videoConn, key)
	}
	client.peerConnection.Close()
	client.isAuth = false
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
	defer client.wsConn.Close()
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
				client.CreateOffer()
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

func (client *CastXClient) initWebRtc() error {
	config := webrtc.Configuration{}
	depacketizer := NewH264Depacketizer(client)
	var err error
	// 创建PeerConnection
	client.peerConnection, err = webrtc.NewPeerConnection(config)
	if err != nil {
		fmt.Printf("StartWebRtcReceive err:%+v\n", err)
		return err
	}
	if _, err = client.peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo); err != nil {
		fmt.Printf("StartWebRtcReceive err:%+v\n", err)
		return err
	}
	if _, err = client.peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio); err != nil {
		fmt.Printf("StartWebRtcReceive err:%+v\n", err)
		return err
	}
	// 设置视频轨道处理
	client.peerConnection.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
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
	return nil
}
func (client *CastXClient) CreateOffer() error {
	gatherCompletePromise := webrtc.GatheringCompletePromise(client.peerConnection)
	// 创建Offer
	offer, err := client.peerConnection.CreateOffer(nil)
	if err != nil {
		fmt.Printf("StartWebRtcReceive err:%+v\n", err)
		return err
	}
	fmt.Printf("StartWebRtcReceive4\r\n")
	// 设置本地描述
	if err = client.peerConnection.SetLocalDescription(offer); err != nil {
		return err
	}
	<-gatherCompletePromise
	// 发送Offer到信令服务
	client.getOffer(*client.peerConnection.LocalDescription())
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

func (client *CastXClient) startSendListen() {
	// 启动 TCP 服务器
	var err error
	client.listener, err = net.Listen("tcp", fmt.Sprintf("127.0.0.1:0"))
	if err != nil {
		panic(fmt.Sprintf("监听失败: %v", err))
	}
	client.run = true
	// 主接收循环

	for client.run {
		conn, err := client.listener.Accept()
		if err != nil {
			fmt.Printf("接受连接失败: %v\n", err)
			break
		}
		buf := make([]byte, 5)
		conn.Read(buf)
		if string(buf) == "video" {
			client.videoConn[conn.RemoteAddr().String()] = conn
		} else if string(buf) == "audio" {
			client.audioConn[conn.RemoteAddr().String()] = conn
		} else {
			conn.Close()
		}
	}
}

func (client *CastXClient) sendVidee(data []byte, pts uint64, isKeyFrame bool) {
	client.videomu.RLock()
	defer client.videomu.RUnlock()
	var err error
	for key, conn := range client.videoConn {
		err = writeFrameHeader(conn, data, pts, isKeyFrame)
		if err != nil {
			delete(client.videoConn, key)
		}
	}
}
