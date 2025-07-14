package castxClient

import (
	"fmt"
	"net"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
)

type CastXClient struct {
	wsClient       *WsClient
	peerConnection *webrtc.PeerConnection
	rtpSendConn    *net.UDPConn
	sdp            string
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
	client.wsClient.Conect(wsUrl, password, maxSize)

	return 0
}

func (client *CastXClient) sendRtp(rtpPacket *rtp.Packet) error {

	var err error
	if client.rtpSendConn == nil {
		targetAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:4002")
		client.rtpSendConn, err = net.DialUDP("udp", nil, targetAddr)
		if err != nil {
			fmt.Println("连接失败:", err)
			return err
		}
	}
	rtpPacket.Header.Extension = false

	//rtpPacket.PayloadType = 96
	buf, err := rtpPacket.Marshal()

	_, err = client.rtpSendConn.Write(buf)
	//fmt.Printf("buf:%+v\r\n", buf)
	if err != nil {
		client.rtpSendConn = nil
	}

	return nil
}
