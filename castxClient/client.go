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

func (client *CastXClient) Start(wsUrl string, password string, maxSize int, useRtsp bool) int {
	var err error
	client.wsConn, _, err = websocket.DefaultDialer.Dial(wsUrl, nil)
	if err != nil {
		return 0
	}
	client.audioConn = make(map[string]net.Conn)
	client.videoConn = make(map[string]net.Conn)
	//init websrc
	if useRtsp {
		client.initWebRtc()
		client.startRtsp()
	}
	// 消息接收协程
	go client.WsRecv(password, maxSize)

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
		switch msg.Type {
		case comm.MsgTypeInitConfig:
			data := msg.Data.(map[string]interface{})
			client.securityKey = data["securityKey"].(string)
			client.login(password, maxSize)
		case comm.MsgTypeLoginAuthResp:
			data := msg.Data.(map[string]interface{})
			if data["auth"].(bool) {
				client.isAuth = true
				client.CreateOffer()
			}
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
