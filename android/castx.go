package castX

// android build   gomobile bind -androidapi=21 -target=android -ldflags "-checklinkname=0 -s -w"

import (
	"encoding/json"
	"runtime"

	"github.com/dosgo/castX/castxServer"
	"github.com/wlynxg/anet"

	"github.com/dosgo/castX/scrcpy"
	_ "golang.org/x/mobile/bind"
)

var castx *castxServer.Castx
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
}

var c JavaCallbackInterface

type JavaClass struct {
	JavaCall JavaCallbackInterface
}

var javaObj *JavaClass

func RegJavaClass(c JavaCallbackInterface) {
	javaObj = &JavaClass{c}
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
