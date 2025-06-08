package scrcpy

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/dosgo/castX/static"
	"github.com/dosgo/libadb"
	"github.com/gorilla/websocket"
)

func (scrcpyClient *ScrcpyClient) InitAdb(peerName string, savPath string, reversePort int) {
	//init
	var adbClient = libadb.AdbClient{CertFile: fmt.Sprintf("%sadbkey.pub", savPath), KeyFile: fmt.Sprintf("%sadbkey.key", savPath), PeerName: peerName}

	scrcpyClient.castx.WsServer.SetAdbConnect(func(data string) {
		var dataInfo map[string]interface{}
		err := json.Unmarshal([]byte(data), &dataInfo)
		if err != nil {
			return
		}
		if _, ok := dataInfo["selectedType"]; ok {
			if selectedType, ok1 := dataInfo["selectedType"].(string); ok1 {

				if selectedType == "wifi" {
					var address = dataInfo["address"].(string)
					var adbType = dataInfo["adbType"].(string)

					if adbType == "connect" {
						var connectPort = dataInfo["connectPort"].(float64)

						//已经连接
						if scrcpyClient.castx.Config.AdbConnect {
							return
						}
						connected := adbClient.Connect(fmt.Sprintf("%s:%d", address, int(connectPort)))
						if connected == nil {
							scrcpyClient.adbConnectOk(&adbClient, savPath, reversePort)
						} else {
							fmt.Printf("adb connect failed connected:%+v\r\n", connected)
						}
					}
					if adbType == "pair" {
						var authPort = dataInfo["authPort"].(float64)
						var authCode = 0
						if _authCode, ok3 := dataInfo["authCode"].(float64); ok3 {
							authCode = int(_authCode)
						}
						if _authCode, ok4 := dataInfo["authCode"].(string); ok4 {
							authCode, err = strconv.Atoi(_authCode)
						}
						adbClient.Pair(fmt.Sprintf("%d", authCode), fmt.Sprintf("%s:%d", address, int(authPort)))
					}
				}
			}
		}
	})

	scrcpyClient.castx.WsServer.SetUsbConnectFun(func(usbConn *websocket.Conn) {

		netConn := NewWebsocketConnAdapter(usbConn)
		connected := adbClient.UsbConnect(netConn)
		fmt.Printf("UsbConnect err:%+v\r\n", connected)
		if connected == nil {
			scrcpyClient.adbConnectOk(&adbClient, savPath, reversePort)
		}

	})
}

func (scrcpyClient *ScrcpyClient) adbConnectOk(adbClient *libadb.AdbClient, savPath string, reversePort int) {
	maxSize := ""
	if scrcpyClient.castx.Config.MaxSize > 0 {
		maxSize = fmt.Sprintf("max_size=%d", int(scrcpyClient.castx.Config.MaxSize))
	}
	scrcpyClient.castx.ScrcpyReceiver.Counter = 0 //重置接收计数器很重要
	go func() {
		defer func() {
			scrcpyClient.castx.Config.AdbConnect = false
			scrcpyClient.castx.WsServer.BroadcastInfo()
		}()
		localFile := fmt.Sprintf("%sscrcpy-server-v3.1", savPath)
		writeIfMD5Mismatch(localFile)
		pushErr := adbClient.Push(localFile, "/data/local/tmp/scrcpy-server", 0644)
		fmt.Printf("pushErr:%+v\r\n", pushErr)

		scid := GenerateSCID()
		reverseErr := adbClient.Reverse(fmt.Sprintf("localabstract:scrcpy_%s", scid), fmt.Sprintf("tcp:%d", reversePort))
		fmt.Printf("ReverseErr:%+v\r\n", reverseErr)

		//repeat-previous-frame-after=0
		// audio-output-buffer=100 --audio-buffer=100
		//'profile=4200,b-frames=0,preset=ultrafast'
		//repeat-previous-frame-after=5
		cmd := fmt.Sprintf("CLASSPATH=/data/local/tmp/scrcpy-server app_process / com.genymobile.scrcpy.Server 3.1 scid=%s  log_level=debug cleanup=true video_bit_rate=4000000  video_codec_options=profile=65536 %s", scid, maxSize)
		adbClient.ShellCmd(cmd, true)

	}()
	scrcpyClient.castx.Config.AdbConnect = true
	scrcpyClient.castx.WsServer.BroadcastInfo()
}
func writeIfMD5Mismatch(localPath string) error {
	embedData, err := static.StaticFiles.ReadFile(filepath.Base(localPath))
	if err != nil {
		return fmt.Errorf("读取嵌入文件失败: %w", err)
	}
	embedHash := md5.Sum(embedData)
	if _, err := os.Stat(localPath); os.IsNotExist(err) {
		os.WriteFile(localPath, embedData, 0644)
		return nil
	}
	localData, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("读取本地文件失败: %w", err)
	}
	localHash := md5.Sum(localData)
	if localHash != embedHash {
		os.WriteFile(localPath, embedData, 0644)
	}
	return nil
}

func GenerateSCID() string {
	seed := time.Now().UnixNano() + rand.Int63()
	r := rand.New(rand.NewSource(seed))
	// 生成31位随机整数
	return strconv.FormatInt(int64(r.Uint32()&0x7FFFFFFF), 16)
}

// 定义适配器结构体
type WebsocketConnAdapter struct {
	conn    *websocket.Conn
	rbuf    []byte // 读缓冲区
	writeMu sync.Mutex
	readMu  sync.Mutex
}

func NewWebsocketConnAdapter(conn *websocket.Conn) *WebsocketConnAdapter {
	return &WebsocketConnAdapter{
		conn: conn,
	}
}

// 实现Read方法
func (a *WebsocketConnAdapter) Read(b []byte) (int, error) {
	a.readMu.Lock()
	defer a.readMu.Unlock()
	// 如果缓冲区有数据，先从中读取
	if len(a.rbuf) > 0 {
		n := copy(b, a.rbuf)
		a.rbuf = a.rbuf[n:]
		return n, nil
	}

	// 读取下一个WebSocket消息
	msgType, data, err := a.conn.ReadMessage()
	if err != nil {
		return 0, err
	}

	// 只处理二进制和文本消息
	if msgType != websocket.BinaryMessage && msgType != websocket.TextMessage {
		// 跳过非数据帧，继续读取下一个
		return a.Read(b)
	}

	// 将消息放入缓冲区
	a.rbuf = data
	n := copy(b, a.rbuf)
	a.rbuf = a.rbuf[n:]
	return n, nil
}

// 实现Write方法
func (a *WebsocketConnAdapter) Write(b []byte) (int, error) {
	a.writeMu.Lock()
	defer a.writeMu.Unlock()
	// 将数据写入为二进制消息
	err := a.conn.WriteMessage(websocket.BinaryMessage, b)
	if err != nil {
		return 0, err
	}
	return len(b), nil
}

// 实现Close方法
func (a *WebsocketConnAdapter) Close() error {
	// 发送一个关闭消息并关闭连接
	return a.conn.Close()
}
