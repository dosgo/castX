package castX

// android build   gomobile bind -androidapi 21 -target=android -ldflags "-checklinkname=0"

import (
	"encoding/json"

	"github.com/dosgo/castX/castxServer"
	"github.com/dosgo/castX/comm"
	"github.com/dosgo/castX/scrcpy"
	_ "golang.org/x/mobile/bind"
)

var castx *castxServer.Castx
var webrtcReceive *comm.WebrtcReceive
var scrcpyClient *scrcpy.ScrcpyClient

func Start(webPort int, width int, height int, mimeType string, password string, receiverPort int) {
	castx, _ = castxServer.Start(webPort, width, height, mimeType, false, password, receiverPort)
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
	CallString(param string)
	CallBytes(cmd int, param []byte, timestamp int64)
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
	castx.WsServer.SetControlFun(func(data map[string]interface{}) {
		jsonStr, err := json.Marshal(data)
		if err == nil {
			javaObj.JavaCall.CallString(string(jsonStr))
		}
	})
	if webrtcReceive != nil {
		webrtcReceive.SetReceiveCall(func(cmd int, data []byte, timestamp int64) {
			javaObj.JavaCall.CallBytes(cmd, data, timestamp)
		})
	}
	castx.WebrtcServer.SetWebRtcConnectionStateChange(func(count int) {
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

func StartWebRtcReceive(url string) {
	webrtcReceive = &comm.WebrtcReceive{}
	webrtcReceive.StartWebRtcReceive(url, false)
}

func SetSize(width int, height int, videoWidth int, videoHeight int, orientation int) {
	castx.UpdateConfig(width, height, videoWidth, videoHeight, orientation)
}

func StartScrcpyClient(webPort int, peerName string, savaPath string, password string) {
	scrcpyClient = scrcpy.NewScrcpyClient(webPort, peerName, savaPath, password)
	scrcpyClient.StartClient()
}
func ShutdownScrcpyClient() {
	if scrcpyClient != nil {
		scrcpyClient.Shutdown()
		scrcpyClient = nil
	}
}
