package comm

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"sync"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3"
	"github.com/wlynxg/anet"
)

type WebrtcReceive struct {
	receiveCall func(int, []byte, int64)
}

func (webrtcReceive *WebrtcReceive) SetReceiveCall(compare func(int, []byte, int64)) {
	webrtcReceive.receiveCall = compare
}

func (webrtcReceive *WebrtcReceive) StartWebRtcReceive(url string, writeFile bool) error {
	if runtime.GOOS == "android" {
		anet.SetAndroidVersion(14)
	}
	depacketizer := NewH264Depacketizer(webrtcReceive, writeFile)
	// WebRTC配置
	config := webrtc.Configuration{}
	// 创建PeerConnection
	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		fmt.Printf("StartWebRtcReceive err:%+v\n", err)
		return err
	}
	if _, err = peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo); err != nil {
		fmt.Printf("StartWebRtcReceive err:%+v\n", err)
		return err
	}
	if _, err = peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio); err != nil {
		fmt.Printf("StartWebRtcReceive err:%+v\n", err)
		return err
	}
	// 设置视频轨道处理
	peerConnection.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
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
	})
	gatherCompletePromise := webrtc.GatheringCompletePromise(peerConnection)
	// 创建Offer
	offer, err := peerConnection.CreateOffer(nil)
	if err != nil {
		fmt.Printf("StartWebRtcReceive err:%+v\n", err)
		return err
	}
	fmt.Printf("StartWebRtcReceive4\r\n")
	// 设置本地描述
	if err = peerConnection.SetLocalDescription(offer); err != nil {
		return err
	}
	<-gatherCompletePromise

	// 发送Offer到信令服务
	answer := getOffer(*peerConnection.LocalDescription(), url)

	// 设置远程描述
	if err = peerConnection.SetRemoteDescription(answer); err != nil {
		fmt.Printf("StartWebRtcReceive err:%+v\n", err)
		return err
	}
	return nil
}

// 信令交互
func getOffer(offer webrtc.SessionDescription, url string) webrtc.SessionDescription {
	offerJSON, _ := json.Marshal(offer)
	resp, _ := http.Post(url, "application/json", bytes.NewReader(offerJSON))
	var data = make(map[string]interface{})
	json.NewDecoder(resp.Body).Decode(&data)
	answerStr, _ := json.Marshal(data["sdp"])
	fmt.Printf("answerStr:%s\r\n", answerStr)
	var answer webrtc.SessionDescription
	json.NewDecoder(bytes.NewBuffer([]byte(answerStr))).Decode(&answer)
	return answer
}

type H264Depacketizer struct {
	file           *os.File
	sps            []byte
	pps            []byte
	fragmentBuffer []byte
	lastTimestamp  uint32
	mu             sync.Mutex
	writeFile      bool
	webrtcReceive  *WebrtcReceive
}

func NewH264Depacketizer(webrtcReceive *WebrtcReceive, _writeFile bool) *H264Depacketizer {
	h264Decode := &H264Depacketizer{
		writeFile: _writeFile,
	}
	if _writeFile {
		f, _ := os.Create("output.264")
		h264Decode.file = f
	}
	h264Decode.webrtcReceive = webrtcReceive
	return h264Decode
}

func (d *H264Depacketizer) ProcessRTP(pkt *rtp.Packet) {
	d.mu.Lock()
	defer d.mu.Unlock()

	payload := pkt.Payload
	if len(payload) < 1 {
		return
	}

	// 处理分片单元
	naluType := payload[0] & 0x1F
	switch {
	case naluType >= 1 && naluType <= 23:
		d.writeNALU(payload, int64(pkt.Timestamp))
	case naluType == 28: // FU-A分片
		d.processFUA(payload, pkt.Timestamp)
	case naluType == 24: // STAP-A聚合包
		d.processSTAPA(payload, pkt.Timestamp)
	}
}

func (d *H264Depacketizer) processFUA(payload []byte, timestamp uint32) {
	if len(payload) < 2 {
		return
	}

	fuHeader := payload[1]
	start := (fuHeader & 0x80) != 0
	end := (fuHeader & 0x40) != 0

	nalType := fuHeader & 0x1F
	naluHeader := (payload[0] & 0xE0) | nalType

	if start {
		d.fragmentBuffer = []byte{naluHeader}
		d.fragmentBuffer = append(d.fragmentBuffer, payload[2:]...)
		d.lastTimestamp = timestamp
	} else if timestamp == d.lastTimestamp {
		d.fragmentBuffer = append(d.fragmentBuffer, payload[2:]...)
	}

	if end {
		if d.fragmentBuffer != nil {
			d.writeNALU(d.fragmentBuffer, int64(timestamp))
			d.fragmentBuffer = nil
		}
	}
}

func (d *H264Depacketizer) processSTAPA(payload []byte, timestamp uint32) {
	offset := 1

	for offset < len(payload) {
		if offset+2 > len(payload) {
			break
		}

		size := int(binary.BigEndian.Uint16(payload[offset:]))
		offset += 2

		if offset+size > len(payload) {
			break
		}

		d.writeNALU(payload[offset:offset+size], int64(timestamp))
		offset += size
	}
}

func (d *H264Depacketizer) writeNALU(nalu []byte, timestamp int64) {
	naluType := nalu[0] & 0x1F
	startCode := []byte{0x00, 0x00, 0x00, 0x01}
	// 提取参数集
	switch naluType {
	case 7: // SPS
		d.sps = append([]byte{}, nalu...)
		if d.webrtcReceive.receiveCall != nil {
			d.webrtcReceive.receiveCall(2, nalu, timestamp)
		}

		if d.writeFile {
			d.file.Write(startCode)
			d.file.Write(d.sps)
		}
		fmt.Printf("Got SPS: %s\n", hex.EncodeToString(nalu))
	case 8: // PPS
		if d.webrtcReceive.receiveCall != nil {
			d.webrtcReceive.receiveCall(3, nalu, timestamp)
		}
		d.pps = append([]byte{}, nalu...)
		if d.writeFile {
			d.file.Write(startCode)
			d.file.Write(d.pps)
		}
		fmt.Printf("Got PPS: %s\n", hex.EncodeToString(nalu))

	}

	// 实时解码示例（需实现解码器接口）
	if naluType == 1 || naluType == 5 {
		if d.webrtcReceive.receiveCall != nil {
			d.webrtcReceive.receiveCall(1, nalu, timestamp)
		}
		if d.writeFile {
			d.file.Write(startCode)
			d.file.Write(nalu)
		}
	}
}
