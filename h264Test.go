package main

import (
	"bytes"
	"fmt"
	"image"

	"github.com/y9o/go-openh264"
)

func main() {
	// 创建解码器
	decoder, err := NewH264Decoder("./openh264-2.4.1-win64.dll")
	if err != nil {
		panic(err)
	}
	defer decoder.Close()

	// 创建帧通道
	frameChan := make(chan *image.YCbCr, 10)

	// 启动goroutine处理帧
	go func() {
		for frame := range frameChan {
			fmt.Printf("收到帧: %dx%d\n", frame.Rect.Dx(), frame.Rect.Dy())
			// 实时处理帧数据
		}
	}()

	// 解码数据
	h264Data := []byte{} // 您的H.264数据
	if err := decoder.DecodeToChannel(h264Data, frameChan); err != nil {
		panic(err)
	}

	// 刷新剩余帧
	decoder.FlushRemainingFrames(frameChan)
	close(frameChan)
}

// H264Decoder H.264解码器封装
type H264Decoder struct {
	decoder *openh264.ISVCDecoder
}

// NewH264Decoder 创建新的H.264解码器
func NewH264Decoder(dllPath string) (*H264Decoder, error) {
	var err error
	if err := openh264.Open(dllPath); err != nil {
		return nil, err
	}

	var decoder *openh264.ISVCDecoder
	if ret := openh264.WelsCreateDecoder(&decoder); ret != 0 {
		openh264.Close()
		return nil, err
	}

	param := openh264.SDecodingParam{}
	param.EEcActiveIdc = openh264.ERROR_CON_SLICE_MV_COPY_CROSS_IDR_FREEZE_RES_CHANGE
	if r := decoder.Initialize(&param); r != 0 {
		openh264.WelsDestroyDecoder(decoder)
		openh264.Close()
		return nil, err
	}

	return &H264Decoder{decoder: decoder}, nil
}

// Close 关闭解码器并释放资源
func (d *H264Decoder) Close() {
	if d.decoder != nil {
		d.decoder.Uninitialize()
		openh264.WelsDestroyDecoder(d.decoder)
		openh264.Close()
	}
}

// DecodeToChannel 解码H.264数据并通过通道输出YCbCr帧
func (d *H264Decoder) DecodeToChannel(h264Data []byte, frameChan chan<- *image.YCbCr) error {
	data := h264Data

	for len(data) > 4 {
		pos := bytes.Index(data[4:], []byte{0, 0, 0, 1})
		nalLength := len(data)
		if pos != -1 {
			nalLength = pos + 4
		}

		var bufferInfo openh264.SBufferInfo
		var yuvData [3][]byte

		if r := d.decoder.DecodeFrameNoDelay(data[:nalLength], nalLength, &yuvData, &bufferInfo); r != 0 {
			continue
		}

		if yuvData[0] != nil {
			frame := &image.YCbCr{
				Y:       yuvData[0],
				Cb:      yuvData[1],
				Cr:      yuvData[2],
				YStride: int(bufferInfo.UsrData_sSystemBuffer().IStride[0]),
				CStride: int(bufferInfo.UsrData_sSystemBuffer().IStride[1]),
				Rect: image.Rect(0, 0,
					int(bufferInfo.UsrData_sSystemBuffer().IWidth),
					int(bufferInfo.UsrData_sSystemBuffer().IHeight)),
				SubsampleRatio: image.YCbCrSubsampleRatio420,
			}
			frameChan <- frame
		}

		if pos == -1 {
			break
		}
		data = data[pos+4:]
	}

	return nil
}

// FlushRemainingFrames 刷新解码器中的剩余帧到通道
func (d *H264Decoder) FlushRemainingFrames(frameChan chan<- *image.YCbCr) {
	var remainingFrames int
	d.decoder.GetOption(openh264.DECODER_OPTION_NUM_OF_FRAMES_REMAINING_IN_BUFFER, &remainingFrames)

	for i := 0; i < remainingFrames; i++ {
		var bufferInfo openh264.SBufferInfo
		var yuvData [3][]byte

		if r := d.decoder.FlushFrame(&yuvData, &bufferInfo); r == 0 && yuvData[0] != nil {
			frame := &image.YCbCr{
				Y:       yuvData[0],
				Cb:      yuvData[1],
				Cr:      yuvData[2],
				YStride: int(bufferInfo.UsrData_sSystemBuffer().IStride[0]),
				CStride: int(bufferInfo.UsrData_sSystemBuffer().IStride[1]),
				Rect: image.Rect(0, 0,
					int(bufferInfo.UsrData_sSystemBuffer().IWidth),
					int(bufferInfo.UsrData_sSystemBuffer().IHeight)),
				SubsampleRatio: image.YCbCrSubsampleRatio420,
			}
			frameChan <- frame
		}
	}
}
