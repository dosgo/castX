package comm

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type WsServer struct {
	loadInitCall      func(data string)            //页面加载完成回调
	adbConnectCall    func(data string)            //adb连接回调
	controlCall       func(map[string]interface{}) //控制消息回调
	usbConnectCall    func(*websocket.Conn)        //usb连接回调
	connectionManager *ConnectionManager
	webrtcServer      *WebrtcServer
	config            *Config
	auth              sync.Map
	tokens            *ttlMap
	loginNum          *ttlMap
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type WSMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

const (
	MsgTypeOffer          = "offer"
	MsgTypeControl        = "control"
	MsgTypeOfferResp      = "offerResponse"
	MsgTypeControlResp    = "controlResponse"
	MsgTypeInfoNotify     = "infoNotify"
	MsgTypeLoginAuth      = "loginAuth"
	MsgTypeLoginAuthResp  = "loginAuthResp"
	MsgTypeConnectAdb     = "connectAdb"
	MsgTypeConnectAdbResp = "connectAdbResp"
	MsgTypeInitConfig     = "initConfig"
)

func NewWs(config *Config, webrtcServer *WebrtcServer) *WsServer {
	wsServer := &WsServer{}
	wsServer.config = config
	wsServer.webrtcServer = webrtcServer
	wsServer.connectionManager = &ConnectionManager{
		connections: make(map[*WsSafeConn]bool),
	}
	wsServer.tokens = NewTTLMap(20)
	wsServer.loginNum = NewTTLMap(3600)
	return wsServer
}

func (wsServer *WsServer) SetLoadInitFunc(_loadInit func(string)) {
	wsServer.loadInitCall = _loadInit
}

func (wsServer *WsServer) SetAdbConnect(_adbConnect func(string)) {
	wsServer.adbConnectCall = _adbConnect
}
func (wsServer *WsServer) SetControlFun(_controlCallFun func(map[string]interface{})) {
	wsServer.controlCall = _controlCallFun
}
func (wsServer *WsServer) SetUsbConnectFun(usbConnectCall func(*websocket.Conn)) {
	wsServer.usbConnectCall = usbConnectCall
}

func (wsServer *WsServer) BroadcastInfo() {
	wsServer.connectionManager.Broadcast(WSMessage{
		Type: MsgTypeInfoNotify,
		Data: map[string]interface{}{
			"orientation": wsServer.config.Orientation,
			"videoHeight": wsServer.config.VideoHeight,
			"videoWidth":  wsServer.config.VideoWidth,
			"useAdb":      wsServer.config.UseAdb,
			"adbConnect":  wsServer.config.AdbConnect,
		},
	})
}

/*发送初始化数据*/
func (wsServer *WsServer) SendInitConfig(c *WsSafeConn) {
	msg := WSMessage{
		Type: MsgTypeInitConfig,
		Data: map[string]interface{}{
			"GOOS":        runtime.GOOS,
			"securityKey": wsServer.config.SecurityKey,
		},
	}
	c.WriteJSON(msg)
}
func (wsServer *WsServer) Shutdown() {
	wsServer.tokens.Close()
	wsServer.loginNum.Close()
}
func (wsServer *WsServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	if isPrivateIPv4(r.RemoteAddr) == false {
		http.Error(w, "Access denied. Only IPv4 LAN allowed.", http.StatusForbidden)
		return
	}
	_conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	//如果是usb连接上的websocket
	if r.URL.Path == "/usbWs" {
		if wsServer.usbConnectCall != nil {
			wsServer.usbConnectCall(_conn)
		} else {
			fmt.Printf("usbConnectCall nil\r\n")
			_conn.Close()
		}
		return
	}
	remoteIP, _, _ := net.SplitHostPort(_conn.RemoteAddr().String())
	conn := NewWsSafeConn(_conn)
	wsServer.auth.Store(conn, false)
	wsServer.connectionManager.Add(conn)
	defer func() {
		conn.Close()
		wsServer.auth.Delete(conn)
		wsServer.connectionManager.Remove(conn)
	}()
	wsServer.SendInitConfig(conn)
	var msg WSMessage
	for {
		err := conn.ReadJSON(&msg)
		if err != nil {
			break
		}
		//如果没有登录并且数据不是登录数据跳过
		if msg.Type != MsgTypeLoginAuth {
			flag, ok := wsServer.auth.Load(conn)
			if !ok || !flag.(bool) {
				continue
			}
		}

		switch msg.Type {
		case MsgTypeLoginAuth:
			wsServer.handleLogin(conn, msg.Data, remoteIP)
		//获取webrtc连接
		case MsgTypeOffer:
			go wsServer.handleOffer(conn, msg.Data)
			//控制命令
		case MsgTypeControl:
			wsServer.handleControl(conn, msg.Data)
			//连接到adb
		case MsgTypeConnectAdb:
			if wsServer.adbConnectCall != nil {
				wsServer.adbConnectCall(msg.Data.(string)) // 处理初始化消息，例如设置屏幕尺寸或其他设置
			}
		}
	}
}

// HTTP Handler that accepts an Offer and returns an Answer
// adds outboundVideoTrack to PeerConnection
func (wsServer *WsServer) handleOffer(conn *WsSafeConn, data interface{}) {
	dataStr, ok := data.(string)
	if !ok {
		return
	}
	//fmt.Printf("handleOffer data:%+v\r\n", data)
	webRtcSession, err := wsServer.webrtcServer.getSdp(strings.NewReader(dataStr))
	//response, err := json.Marshal(webRtcSession)
	if err != nil {
		fmt.Printf("handleOffer err:%v\r\n", err)
		return
	}
	conn.WriteJSON(WSMessage{
		Type: MsgTypeOfferResp,
		Data: map[string]interface{}{
			"GOOS": runtime.GOOS,
			"sdp":  webRtcSession,
		},
	})
}

// 处理控制命令的WebSocket实现
func (wsServer *WsServer) handleControl(conn *WsSafeConn, data interface{}) {
	var controlData map[string]interface{}
	dataStr, ok := data.(string)
	if !ok {
		return
	}

	err := json.Unmarshal([]byte(dataStr), &controlData)
	if err != nil {
		return
	}
	fmt.Println(data)
	if wsServer.controlCall != nil {
		wsServer.controlCall(controlData)
	}

	conn.WriteJSON(WSMessage{
		Type: MsgTypeControlResp,
		Data: map[string]interface{}{
			"code": 0,
		},
	})
}

func (wsServer *WsServer) handleLogin(conn *WsSafeConn, data interface{}, ip string) {
	if wsServer.loginNum.Get(ip) > 20 {
		conn.WriteJSON(WSMessage{
			Type: MsgTypeLoginAuthResp,
			Data: map[string]interface{}{
				"auth": false,
			},
		})
		conn.Close()
		return
	}
	//解析参数
	dataStr, ok := data.(string)
	if !ok {
		return
	}
	if wsServer.loadInitCall != nil {
		wsServer.loadInitCall(dataStr) // 处理初始化消息，例如设置屏幕尺寸或其他设置
	}
	var reqData map[string]interface{}
	err := json.Unmarshal([]byte(dataStr), &reqData)
	if err != nil {
		return
	}

	if _, ok := reqData["maxSize"]; ok {
		if _, ok1 := reqData["maxSize"].(float64); ok1 {
			wsServer.config.MaxSize = int(reqData["maxSize"].(float64))
		}
	}

	reqToken, ok := reqData["token"].(string)
	if wsServer.tokens.IsExists(reqToken) {
		//已经使用直接关闭
		return
	}
	wsServer.tokens.Store(reqToken, 1)
	timestamp, ok := reqData["timestamp"].(float64)

	var srcData = wsServer.config.SecurityKey + "|" + strconv.FormatInt(int64(timestamp), 10) + "|" + wsServer.config.Password
	sum := sha256.Sum256([]byte(srcData))
	token := hex.EncodeToString(sum[:])
	var auth = false
	//2秒内有效
	if token == reqToken && math.Abs(timestamp-float64(time.Now().UnixMilli())) < 10*1000 {
		wsServer.auth.Store(conn, true)
		auth = true
	} else {
		wsServer.loginNum.Incr(ip, 1)
	}

	conn.WriteJSON(WSMessage{
		Type: MsgTypeLoginAuthResp,
		Data: map[string]interface{}{
			"auth": auth,
		},
	})
	if auth {
		//广播配置信息
		wsServer.BroadcastInfo()
	}
	return
}
