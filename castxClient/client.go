package castxClient

import (
	"fmt"
	"io"

	"github.com/pion/webrtc/v4"
)

type CastXClient struct {
	wsClient       *WsClient
	peerConnection *webrtc.PeerConnection
	Stream         io.WriteCloser
	Width          int
	Height         int
}

func (client *CastXClient) Start(wsUrl string, password string, maxSize int) int {
	client.wsClient = &WsClient{}
	client.initWebRtc()
	client.wsClient.SetLoginFun(func(data map[string]interface{}) {
		fmt.Printf("login  data:%+v\r\n", data)
		if data["auth"].(bool) {
			fmt.Printf("auth ok\r\n")
			client.CreateOffer()
		}
	})
	client.wsClient.SetOfferRespFun(func(data map[string]interface{}) {
		client.SetRemoteDescription(data)
	})
	client.wsClient.SetInfoNotifyFun(func(data map[string]interface{}) {
		fmt.Printf("info  data:%+v\r\n", data)

		if _height, ok := data["videoHeight"].(float64); ok {
			client.Height = int(_height)
		}
		if _width, ok := data["videoWidth"].(float64); ok {
			client.Width = int(_width)
		}
	})
	client.wsClient.Conect(wsUrl, password, maxSize)
	return 0
}
