package castxClient

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strconv"
	"time"

	"github.com/dosgo/castX/comm"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
)

type CastXClient struct {
	wsConn         *websocket.Conn
	isAuth         bool
	securityKey    string
	listener       net.Listener
	run            bool
	LoginCall      func(map[string]interface{}) //登录回调
	OfferRespCall  func(map[string]interface{}) //offer回调
	rtsp           *serverHandler
	peerConnection *webrtc.PeerConnection
}

func (client *CastXClient) Start(wsUrl string, password string, maxSize int, useRtsp bool) int {
	var err error
	client.wsConn, _, err = websocket.DefaultDialer.Dial(wsUrl, nil)
	if err != nil {
		return 0
	}
	//init websrc
	if useRtsp {
		client.initWebRtc()
		client.rtsp = &serverHandler{}
		go client.rtsp.Start(client.peerConnection)
		client.SetLoginFun(func(data map[string]interface{}) {
			if data["auth"].(bool) {
				client.isAuth = true
				client.CreateOffer()
			}
		})
		client.SetOfferRespFun(func(data map[string]interface{}) {
			client.SetRemoteDescription(data)
		})
	}
	// 消息接收协程
	go client.WsRecv(password, maxSize)
	return 0
}

// 信令交互
func (client *CastXClient) SendOffer(offerJSON string) {
	client.wsConn.WriteJSON(comm.WSMessage{
		Type: comm.MsgTypeOffer,
		Data: offerJSON,
	})
}
func (client *CastXClient) Shutdown() {
	client.run = false
	if client.listener != nil {
		client.listener.Close()
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
	fmt.Printf("argsStr:%s\r\n", string(argsStr))
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
			if client.LoginCall != nil {
				client.LoginCall(data)
			}
		case comm.MsgTypeOfferResp:
			data := msg.Data.(map[string]interface{})
			if client.OfferRespCall != nil {
				client.OfferRespCall(data)
			}
		}
	}
}
func (client *CastXClient) SetLoginFun(_loginCall func(map[string]interface{})) {
	client.LoginCall = _loginCall
}

func (client *CastXClient) SetOfferRespFun(_offerRespCall func(map[string]interface{})) {
	client.OfferRespCall = _offerRespCall
}
