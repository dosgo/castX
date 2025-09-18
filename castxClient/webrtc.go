package castxClient

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"

	"github.com/dosgo/castX/comm"
	opusComm "github.com/dosgo/libopus/comm"
	"github.com/dosgo/libopus/opus"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media/h264writer"
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

			go func() {
				h264writer := h264writer.NewWith(client.stream)
				for {
					rtpPacket, _, err := track.ReadRTP()
					if err != nil {
						break
					}
					h264writer.WriteRTP(rtpPacket)
				}
			}()
		}
		if track.Codec().MimeType == "audio/opus" {
			go func() {
				ior, iow := io.Pipe()
				//var buf bytes.Buffer
				player := NewPlayer(ior)
				const sampleRate = 48000
				decoder, _ := opus.NewOpusDecoder(sampleRate, 2)

				pcmData := make([]int16, 1024*2)
				go func() {
					//one := true
					i := 0

					var resampler = opusComm.NewSpeexResampler(2, 48000, 44100, 10)

					for {
						rtpPacket, _, err := track.ReadRTP()
						if err != nil {
							break
						}
						i++
						//跳过第一个包  AOPUSHD
						if IsOpusHead(rtpPacket.Payload) {
							// 2. 将长度前缀转换为uint32
							length := binary.LittleEndian.Uint64(rtpPacket.Payload[8:16])
							//one = false
							opHead := comm.ParseOpusHead(rtpPacket.Payload[16 : 16+length])
							fmt.Printf("rtpPacket.Payload:%+v\r\n", rtpPacket.Payload)
							fmt.Printf("opus head:%+v\r\n", opHead)
							continue
						}

						//	AppendFile("testnew.opus", rtpPacket.Payload, 0644, true)

						outLen, err := decoder.Decode(rtpPacket.Payload, 0, len(rtpPacket.Payload), pcmData, 0, 960*2, false)

						//	fmt.Printf("len(rtpPacket.Payload):%d\r\n", len(rtpPacket.Payload))
						if err != nil {
							fmt.Printf("errr1111:%+v\r\n", err)
						}
						//	fmt.Printf("outLen:%d\r\n", outLen)
						//fmt.Printf("pcmData[:outLen]:%+v\r\n", pcmData[:outLen])

						var inputLen = outLen
						data_packet := make([]int16, 960*2)
						var outLen1 = len(data_packet)

						resampler.ProcessShort(0, pcmData[:outLen], 0, &inputLen, data_packet, 0, &outLen1)

						iow.Write(ManualWriteInt16(data_packet[:outLen1]))
					}
				}()
				// 3. 开始播放
				player.Play()
			}()
		}

	})
	return nil
}

func IsOpusHead(data []byte) bool {

	//AOPUSHDR
	var OpusHeadMagic = []byte{'A', 'O', 'P', 'U', 'S', 'H', 'D'}
	//AOPUSHDR
	// 确保数据长度至少为 8 字节
	if len(data) < len(OpusHeadMagic) {
		return false
	}

	// 比较前 8 个字节是否匹配魔数
	return bytes.Equal(data[:len(OpusHeadMagic)], OpusHeadMagic)
}
func Int16SliceToByteSlice(input []int16) []byte {
	// 计算所需的字节长度
	byteLen := len(input) * 2 // 每个int16占2个字节
	buf := make([]byte, byteLen)

	// 使用binary包进行转换
	for i, s := range input {
		binary.LittleEndian.PutUint16(buf[i*2:], uint16(s))
	}
	return buf
}

func ManualWriteInt16(data []int16) []byte {
	buf := make([]byte, len(data)*2)
	for i, v := range data {
		// 将int16视为uint16来处理，因为它们的位表示相同
		binary.LittleEndian.PutUint16(buf[i*2:], uint16(v))
	}
	return buf
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
	client.WsClient.SendOffer(string(offerJSON))
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
