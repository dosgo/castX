package castxClient

import (
	"fmt"
	"net"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
	"golang.org/x/exp/rand"
)

type CastXClient struct {
	wsClient       *WsClient
	peerConnection *webrtc.PeerConnection
	rtpSendConn    *net.UDPConn
	videoPort      int
	audioPort      int
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
	//port, _ := GetFreeUDPPort()
	port := rand.Intn(65533-1000+1) + 1000
	if client.audioPort == 0 {
		client.audioPort = port
	}
	if client.videoPort == 0 {
		client.videoPort = port + 2
	}
	return 0
}

func (client *CastXClient) sendRtp(rtpPacket *rtp.Packet) error {

	var err error
	if client.rtpSendConn == nil {
		targetAddr, _ := net.ResolveUDPAddr("udp", fmt.Sprintf("127.0.0.1:%d", client.videoPort))
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
