package castxClient

import (
	"fmt"
	"io"

	"github.com/pion/webrtc/v4"
)

type CastXClient struct {
	WsClient       *WsClient
	peerConnection *webrtc.PeerConnection
	stream         io.WriteCloser
	Width          int
	Height         int
}

func NewCastXClient() *CastXClient {
	client := &CastXClient{}
	client.WsClient = &WsClient{}
	return client
}
func (client *CastXClient) SetStream(stream io.WriteCloser) {
	client.stream = stream
}
func (client *CastXClient) Start(wsUrl string, password string, maxSize int) int {
	client.initWebRtc()
	client.WsClient.SetLoginFun(func(data map[string]interface{}) {
		fmt.Printf("login  data:%+v\r\n", data)
		if data["auth"].(bool) {
			fmt.Printf("auth ok\r\n")
			client.CreateOffer()
		}
	})
	client.WsClient.SetOfferRespFun(func(data map[string]interface{}) {
		client.SetRemoteDescription(data)
	})
	client.WsClient.Conect(wsUrl, password, maxSize)
	return 0
}
