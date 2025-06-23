package castxClient

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"sync"

	"github.com/pion/rtp"
)

type H264Depacketizer struct {
	file           *os.File
	sps            []byte
	pps            []byte
	fragmentBuffer []byte
	lastTimestamp  uint32
	mu             sync.Mutex
}

func NewH264Depacketizer() *H264Depacketizer {
	h264Decode := &H264Depacketizer{}
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
	//startCode := []byte{0x00, 0x00, 0x00, 0x01}
	// 提取参数集
	switch naluType {
	case 7: // SPS
		d.sps = append([]byte{}, nalu...)

		fmt.Printf("Got SPS: %s\n", hex.EncodeToString(nalu))
	case 8: // PPS

		d.pps = append([]byte{}, nalu...)

		fmt.Printf("Got PPS: %s\n", hex.EncodeToString(nalu))

	}

	// 实时解码示例（需实现解码器接口）
	if naluType == 1 || naluType == 5 {
		fmt.Printf("h264 data\r\n")
	}
}
