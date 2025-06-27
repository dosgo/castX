package castxClient

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/pion/webrtc/v3"
)

func (client *CastXClient) initWebRtc() error {
	config := webrtc.Configuration{}
	depacketizer := NewH264Depacketizer(client)
	var err error
	// 创建PeerConnection
	client.peerConnection, err = webrtc.NewPeerConnection(config)
	if err != nil {
		fmt.Printf("StartWebRtcReceive err:%+v\n", err)
		return err
	}
	if _, err = client.peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo); err != nil {
		fmt.Printf("StartWebRtcReceive err:%+v\n", err)
		return err
	}
	if _, err = client.peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio); err != nil {
		fmt.Printf("StartWebRtcReceive err:%+v\n", err)
		return err
	}
	// 设置视频轨道处理
	client.peerConnection.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		fmt.Printf("接收到 %s 轨道\n", track.Kind())
		// 创建内存缓冲区
		fmt.Printf("开始接收轨道: %s\n", track.Codec().MimeType)
		if track.Codec().MimeType == "video/H264" {
			go func() {
				for {
					rtpPacket, _, err := track.ReadRTP()
					if err != nil {
						break
					}
					//comm.ProcessNalUnit(rtpPacket.Payload)
					depacketizer.ProcessRTP(rtpPacket)
				}
			}()
		}
		if track.Codec().MimeType == "audio/opus" {
			fmt.Printf("eeee\r\n")
		}
	})
	return nil
}
func (client *CastXClient) CreateOffer() error {
	gatherCompletePromise := webrtc.GatheringCompletePromise(client.peerConnection)
	// 创建Offer
	offer, err := client.peerConnection.CreateOffer(nil)
	if err != nil {
		fmt.Printf("StartWebRtcReceive err:%+v\n", err)
		return err
	}

	// 设置本地描述
	if err = client.peerConnection.SetLocalDescription(offer); err != nil {
		return err
	}
	<-gatherCompletePromise

	offerJSON, _ := json.Marshal(*client.peerConnection.LocalDescription())
	// 发送Offer到信令服务
	client.SendOffer(string(offerJSON))
	return nil
}

func (client *CastXClient) SetRemoteDescription(data map[string]interface{}) {
	answerStr, _ := json.Marshal(data["sdp"])
	var answer webrtc.SessionDescription
	json.NewDecoder(bytes.NewBuffer([]byte(answerStr))).Decode(&answer)
	// 设置远程描述
	if err := client.peerConnection.SetRemoteDescription(answer); err != nil {
		fmt.Printf("StartWebRtcReceive err:%+v\n", err)
	}
}
