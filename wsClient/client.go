package wsClient

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/dosgo/castX/comm"
	"github.com/gorilla/websocket"
)

type WsClient struct {
	wsConn        *websocket.Conn
	isAuth        bool
	securityKey   string
	run           bool
	LoginCall     func(map[string]interface{}) //登录回调
	OfferRespCall func(map[string]interface{}) //offer回调
}

func (client *WsClient) Conect(wsUrl string, password string, maxSize int) int {
	var err error
	client.wsConn, _, err = websocket.DefaultDialer.Dial(wsUrl, nil)
	if err != nil {
		return 0
	}
	// 消息接收协程
	go client.WsRecv(password, maxSize)
	return 0
}

// 信令交互
func (client *WsClient) SendOffer(offerJSON string) {
	client.wsConn.WriteJSON(comm.WSMessage{
		Type: comm.MsgTypeOffer,
		Data: offerJSON,
	})
}
func (client *WsClient) Shutdown() {
	client.run = false
	client.isAuth = false
}
func (client *WsClient) login(password string, maxSize int) {
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
func (client *WsClient) WsRecv(password string, maxSize int) {
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
func (client *WsClient) SetLoginFun(_loginCall func(map[string]interface{})) {
	client.LoginCall = _loginCall
}

func (client *WsClient) SetOfferRespFun(_offerRespCall func(map[string]interface{})) {
	client.OfferRespCall = _offerRespCall
}
