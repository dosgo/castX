package castX

// android build   gomobile bind -androidapi 21 -target=android -ldflags "-checklinkname=0 -s -w"

import (
	"encoding/json"
	"runtime"

	"github.com/dosgo/castX/castxServer"
	"github.com/dosgo/castX/wsClient"
	"github.com/wlynxg/anet"

	"github.com/dosgo/castX/scrcpy"
	_ "golang.org/x/mobile/bind"
)

var castx *castxServer.Castx
var _wsClient *wsClient.WsClient
var scrcpyClient *scrcpy.ScrcpyClient

func Start(webPort int, width int, height int, mimeType string, password string, receiverPort int) {
	if runtime.GOOS == "android" {
		anet.SetAndroidVersion(14)
	}
	castx, _ = castxServer.Start(webPort, width, height, mimeType, false, password, receiverPort)
	castx.WsServer.SetControlFun(func(data map[string]interface{}) {
		jsonStr, err := json.Marshal(data)
		if err == nil {
			javaObj.JavaCall.ControlCall(string(jsonStr))
		}
	})
	castx.WebrtcServer.SetWebRtcConnectionStateChange(func(count int, state int) {
		javaObj.JavaCall.WebRtcConnectionStateChange(count)
	})
	castx.WsServer.SetLoadInitFunc(func(data string) {
		var dataInfo map[string]interface{}
		err := json.Unmarshal([]byte(data), &dataInfo)
		if err != nil {
			return
		}
		if _, ok := dataInfo["maxSize"]; ok {
			if _, ok1 := dataInfo["maxSize"].(float64); ok1 {
				javaObj.JavaCall.SetMaxSize(int(dataInfo["maxSize"].(float64)))
			}
		}
	})
}

func SendVideo(nal []byte, timestamp int64) {
	if castx != nil {
		castx.WebrtcServer.SendVideo(nal, timestamp)
	}
}
func SendAudio(nal []byte, timestamp int64) {
	if castx != nil {
		castx.WebrtcServer.SendAudio(nal, timestamp)
	}
}

func Shutdown() {
	if castx != nil {
		if castx.HttpServer != nil {
			castx.HttpServer.Shutdown()
		}
		if castx.WsServer != nil {
			castx.WsServer.Shutdown()
		}
		if castx.ScrcpyReceiver != nil {
			castx.CloseScrcpyReceiver()
		}
	}
}

type JavaCallbackInterface interface {
	ControlCall(param string)
	WebRtcConnectionStateChange(count int)
	SetMaxSize(maxsize int)
	LoginCall(data string)
	OfferRespCall(data string)
}

var c JavaCallbackInterface

type JavaClass struct {
	JavaCall JavaCallbackInterface
}

var javaObj *JavaClass

func RegJavaClass(c JavaCallbackInterface) {
	javaObj = &JavaClass{c}
}

func StartWsClient(url string, password string, maxsize int) int {
	if runtime.GOOS == "android" {
		anet.SetAndroidVersion(14)
	}
	_wsClient = &wsClient.WsClient{}
	_wsClient.SetLoginFun(func(dataInfo map[string]interface{}) {
		jsonStr, err := json.Marshal(dataInfo)
		if err == nil {
			javaObj.JavaCall.LoginCall(string(jsonStr))
		}
	})
	_wsClient.SetOfferRespFun(func(dataInfo map[string]interface{}) {
		jsonStr, err := json.Marshal(dataInfo)
		if err == nil {
			javaObj.JavaCall.OfferRespCall(string(jsonStr))
		}
	})
	return _wsClient.Conect(url, password, maxsize)
}

func WsClientSendOffer(offerJSON string) {
	if _wsClient != nil {
		_wsClient.SendOffer(offerJSON)
	}
}

func ShutdownWsClient() {
	if _wsClient != nil {
		_wsClient.Shutdown()
		_wsClient = nil
	}
}

func SetSize(videoWidth int, videoHeight int, orientation int) {
	castx.UpdateConfig(videoWidth, videoHeight, orientation)
}

func StartScrcpyClient(webPort int, peerName string, savaPath string, password string) {
	if runtime.GOOS == "android" {
		anet.SetAndroidVersion(14)
	}
	scrcpyClient = scrcpy.NewScrcpyClient(webPort, peerName, savaPath, password)
	scrcpyClient.StartClient()
}
func ShutdownScrcpyClient() {
	if scrcpyClient != nil {
		scrcpyClient.Shutdown()
		scrcpyClient = nil
	}
}
