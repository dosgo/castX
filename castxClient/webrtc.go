package castxClient

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sync"
	"time"

	"github.com/dosgo/castX/comm"
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
				ioBuf := NewBufferedPipe(1024 * 1024 * 5)
				player := NewPlayer(ioBuf)

				const sampleRate = 48000
				const channels = 2
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
							fmt.Printf("rtpPacket.Payload:%+v\r\n", rtpPacket.Payload)
							fmt.Printf("opus head:%+v\r\n", opHead)
							continue
						}

						outLen, err := decoder.Decode(rtpPacket.Payload, 0, len(rtpPacket.Payload), pcmData, 0, 960*2, false)
						outLen = outLen * channels
						//	fmt.Printf("len(rtpPacket.Payload):%d\r\n", len(rtpPacket.Payload))
						if err != nil {
							fmt.Printf("errr1111:%+v\r\n", err)
						}
						ddd := ManualWriteInt16(pcmData[:outLen])
						ioBuf.Write(ddd)
					}
				}()
				// 3. 开始播放
				time.Sleep(time.Second * 2)
				player.Play()
			}()
		}

	})
	return nil
}

func processMultiband(input []int16, sampleRate int) []int16 {
	output := make([]int16, len(input))

	// 简易高通滤波器参数（提取中高频）
	rc := 1.0 / (2.0 * math.Pi * 800.0) // 截止频率~800Hz
	dt := 1.0 / float64(sampleRate)
	alpha := dt / (rc + dt)

	prev := int(0)
	for i, s := range input {
		// 这是一个高通滤波器，outputHigh只包含中高频
		outputHigh := int16(alpha * (float64(prev) + float64(s) - float64(prev)))
		prev = int(s)

		// 只对中高频成分进行衰减
		attenuatedHigh := int(float64(outputHigh) * 0.5) // 衰减50%

		// 重新组合：低频原样输出 + 衰减后的中高频
		output[i] = int16(int(s) - int(outputHigh) + attenuatedHigh)
	}
	return output
}

func applyGain(pcmData []int16, gain float64) []int16 {
	// gain: 增益系数。小于1.0就是衰减。
	// 例如 0.5 就是衰减一半（-6dB），0.7 大约是-3dB
	result := make([]int16, len(pcmData))
	for i, sample := range pcmData {
		adjusted := float64(sample) * gain
		// 确保转换后不溢出
		if adjusted > 32767.0 {
			adjusted = 32767.0
		} else if adjusted < -32768.0 {
			adjusted = -32768.0
		}
		result[i] = int16(adjusted)
	}
	return result
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
