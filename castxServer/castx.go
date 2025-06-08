package castxServer

import (
	"math/rand"
	"time"

	"github.com/dosgo/castX/comm"
	"github.com/pion/webrtc/v3"
)

type Castx struct {
	//	framerate    int
	WebrtcServer   *comm.WebrtcServer
	WsServer       *comm.WsServer
	HttpServer     *comm.HttpServer
	Config         *comm.Config
	ScrcpyReceiver *ScrcpyReceiver
}

func Start(webPort int, width int, height int, _mimeType string, useAdb bool, password string, receiverPort int) (*Castx, error) {
	var castx = &Castx{}
	var err error
	castx.Config = &comm.Config{MimeType: webrtc.MimeTypeH264}
	castx.Config.ScreenWidth = width
	castx.Config.ScreenHeight = height
	castx.Config.UseAdb = useAdb
	castx.Config.SecurityKey = randStr(12)
	castx.Config.Password = password
	if len(_mimeType) > 0 {
		castx.Config.MimeType = _mimeType
	}
	castx.WebrtcServer, err = comm.NewWebRtc(castx.Config.MimeType)
	if err != nil {
		return nil, err
	}
	castx.WsServer = comm.NewWs(castx.Config, castx.WebrtcServer)
	castx.HttpServer, err = comm.StartWeb(webPort, castx.WsServer)
	if receiverPort > 0 {
		castx.ScrcpyReceiver = &ScrcpyReceiver{}
		go castx.startReceiver(receiverPort)
	}
	return castx, nil
}
func (castx *Castx) UpdateConfig(width int, height int, _videoWidth int, _videoHeight int, _orientation int) {
	castx.Config.ScreenWidth = width
	castx.Config.ScreenHeight = height
	castx.Config.VideoWidth = _videoWidth
	castx.Config.VideoHeight = _videoHeight
	castx.Config.Orientation = _orientation
	castx.WsServer.BroadcastInfo()
}

func randStr(n int) string {
	charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, n)
	for i := range b {
		b[i] = charset[rng.Intn(len(charset))]
	}
	return string(b)
}
