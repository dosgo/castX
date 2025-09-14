package castxClient

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

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
				//ior, iow := io.Pipe()
				sss := NewStringReader()
				player := NewPlayer(sss)
				const sampleRate = 48000
				decoder, _ := opus.NewOpusDecoder(sampleRate, 2)

				pcmData := make([]int16, 1024*2)
				go func() {
					for {
						rtpPacket, _, err := track.ReadRTP()
						if err != nil {
							break
						}
						AppendFile("testnew.opus", rtpPacket.Payload, 0644)
						fmt.Printf("rtpPacket.Payload:%+v\r\n", rtpPacket.Payload)
						outLen, err := decoder.Decode(rtpPacket.Payload, 0, len(rtpPacket.Payload), pcmData, 0, sampleRate, false)
						if len(rtpPacket.Payload) > 3 {
							os.WriteFile("test.opus", rtpPacket.Payload, os.ModePerm)
						}
						//	fmt.Printf("len(rtpPacket.Payload):%d\r\n", len(rtpPacket.Payload))
						if err != nil {
							fmt.Printf("errr1111:%+v\r\n", err)
						}
						//fmt.Printf("outLen:%d\r\n", outLen)

						sss.Write(ManualWriteInt16(pcmData[:outLen]))
						// 处理解析后的帧

					}
				}()
				// 3. 开始播放
				player.Play()
			}()
		}

	})
	return nil
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
	var xx bytes.Buffer
	for _, v := range data {
		// 将 int16 分解为两个字节
		b1 := byte(v)
		b2 := byte(v >> 8)

		// 写入两个字节
		xx.Write([]byte{b1, b2})
	}
	return xx.Bytes()
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

type StringReader struct {
	mu     sync.Mutex
	buf    bytes.Buffer
	cuTime time.Time
}

// 构造函数
func NewStringReader() *StringReader {
	obj := &StringReader{cuTime: time.Now()}
	return obj
}

// 2. 实现 io.Reader 接口
func (r *StringReader) Read(p []byte) (n int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var readLen = 1920 * 2
	var buf = make([]byte, readLen)
	io.ReadFull(&r.buf, buf)
	copy(p, buf)
	AppendFile("test.pcm", buf, 0644)
	elapsed := time.Since(r.cuTime)
	fmt.Printf("elapsed:%+v \r\n", elapsed)
	if elapsed < 20 {
		time.Sleep(time.Millisecond * (20 - elapsed))
	}
	r.cuTime = time.Now()
	return readLen, nil
}
func AppendFile(filename string, data []byte, perm os.FileMode) error {
	//ffplay -f s16le -ar 48000 -ch_layout stereo test.pcm
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(data)
	return err
}
func (r *StringReader) Write(p []byte) (n int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf.Write(p)
	return len(p), nil
}
