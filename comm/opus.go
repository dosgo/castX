package comm

import (
	"bytes"
	"encoding/binary"
)

type OpusHead struct {
	Magic      [8]byte
	Version    byte
	Channels   byte
	PreSkip    uint16
	SampleRate uint32
	OutputGain int16 // 注意：有符号
	Mapping    byte
}

func ParseOpusHead(data []byte) *OpusHead {
	var head OpusHead
	r := bytes.NewReader(data)

	binary.Read(r, binary.LittleEndian, &head.Magic)
	binary.Read(r, binary.LittleEndian, &head.Version)
	binary.Read(r, binary.LittleEndian, &head.Channels)
	binary.Read(r, binary.LittleEndian, &head.PreSkip)
	binary.Read(r, binary.LittleEndian, &head.SampleRate)
	binary.Read(r, binary.LittleEndian, &head.OutputGain)
	binary.Read(r, binary.LittleEndian, &head.Mapping)
	return &head
}
