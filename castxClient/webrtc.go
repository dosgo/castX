package castxClient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pion/webrtc/v4"
)

func (client *CastXClient) initWebRtc() error {
	config := webrtc.Configuration{}
	//	depacketizer := NewH264Depacketizer(client)
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
			client.addSDPFromTrack(track)
			go func() {
				for {
					rtpPacket, _, err := track.ReadRTP()
					if err != nil {
						break
					}
					client.sendRtp(rtpPacket)
				}
			}()
		}
		if track.Codec().MimeType == "audio/opus" {
			client.addSDPFromTrack(track)
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
	client.wsClient.SendOffer(string(offerJSON))
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

func (client *CastXClient) addSDPFromTrack(track *webrtc.TrackRemote) {
	// 构建基本 SDP
	if client.sdp == "" {
		client.sdp = "v=0\n"
		client.sdp += "o=- 0 0 IN IP4 127.0.0.1\n"
		client.sdp += "s=TrackRemote Generated SDP\n"
		client.sdp += "c=IN IP4 127.0.0.1\n"
		client.sdp += "t=0 0\n"
	}

	// 媒体类型
	if track != nil {
		mediaType := "video"
		if track.Kind() == webrtc.RTPCodecTypeAudio {
			mediaType = "audio"
		}
		codec := track.Codec()
		var port int
		if mediaType == "video" {
			port = client.videoPort
		} else {
			port = client.audioPort
		}
		// 媒体描述
		client.sdp += fmt.Sprintf("m=%s %d RTP/AVP %d\n", mediaType, port, codec.PayloadType)
		mimeTypeInfo := strings.Split(codec.MimeType, "/")
		client.sdp += fmt.Sprintf("a=rtpmap:%d %s/%d\n", codec.PayloadType, strings.ToUpper(mimeTypeInfo[1]), codec.ClockRate)
	}
}
func (client *CastXClient) GetRtpSdp() string {
	return strings.Trim(client.sdp, "\n")

}
