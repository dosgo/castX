package castxClient

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/dosgo/castX/comm"
	"github.com/dosgo/libopus/opus"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media/h264writer"
)

func (client *CastXClient) initWebRtc() error {
	config := webrtc.Configuration{}

	var err error
	// 创建PeerConnection
	client.peerConnection, err = webrtc.NewPeerConnection(config)
	if err != nil {
		log.Printf("StartWebRtcReceive err:%+v\n", err)
		return err
	}
	if _, err = client.peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo); err != nil {
		log.Printf("StartWebRtcReceive err:%+v\n", err)
		return err
	}
	if _, err = client.peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio); err != nil {
		log.Printf("StartWebRtcReceive err:%+v\n", err)
		return err
	}
	// 设置视频轨道处理

	client.peerConnection.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		log.Printf("接收到 %s 轨道\n", track.Kind())
		// 创建内存缓冲区
		log.Printf("开始接收轨道: %s\n", track.Codec().MimeType)

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
				ioBuf := NewBufferedPipe(1024 * 1024)
				player := NewPlayer(ioBuf)

				var sampleRate = 48000
				var channels = 2
				decoder, _ := opus.NewOpusDecoder(sampleRate, channels)
				pcmData := make([]int16, 1024*2)
				go func() {
					i := 0
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
							log.Printf("rtpPacket.Payload:%+v\r\n", rtpPacket.Payload)
							log.Printf("opus head:%+v\r\n", opHead)
							continue
						}

						outLen, err := decoder.Decode(rtpPacket.Payload, 0, len(rtpPacket.Payload), pcmData, 0, 960*2, false)
						outLen = outLen * channels
						if err != nil {
							log.Printf("err:%+v\r\n", err)
						}
						ioBuf.Write(ManualWriteInt16(pcmData[:outLen]))
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
		log.Printf("StartWebRtcReceive err:%+v\n", err)
	}
}

func AppendFile(filename string, data []byte, perm os.FileMode, isLen bool) error {
	//ffplay -f s16le -ar 48000 -ch_layout stereo test.pcm
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	defer file.Close()
	if isLen {
		lengthBuf := make([]byte, 4)
		binary.LittleEndian.PutUint32(lengthBuf, uint32(len(data)))
		file.Write(lengthBuf)
	}
	_, err = file.Write(data)
	return err
}

// BufferedPipe 带缓冲的非阻塞管道
type BufferedPipe struct {
	buf bytes.Buffer
	mu  sync.Mutex // 互斥锁
}

// NewBufferedPipe 创建新的带缓冲管道
func NewBufferedPipe(bufferSize int) *BufferedPipe {
	p := &BufferedPipe{}
	return p
}

// Write 写入数据到管道（非阻塞）
func (p *BufferedPipe) Write(data []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.buf.Write(data)
}

// Read 从管道读取数据（非阻塞）
func (p *BufferedPipe) Read(data []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	readLen := min(1920, len(data))
	_len, _ := p.buf.Read(data[:readLen])
	return _len, nil
}

// Close 关闭管道
func (p *BufferedPipe) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return nil
}

// 辅助函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
