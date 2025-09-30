package castxClient

import (
	"errors"
	"fmt"
	"time"
	"unsafe"

	"github.com/dwdcth/ffmpeg-go/v7/ffcommon"
	"github.com/dwdcth/ffmpeg-go/v7/libavcodec"
	"github.com/dwdcth/ffmpeg-go/v7/libavutil"
)

// H264Decoder H.264解码器封装
type H264Decoder struct {
	codecCtx       *libavcodec.AVCodecContext
	yuvFrame       *libavutil.AVFrame
	width          int32
	height         int32
	outputDataChan chan []byte
	pkt            *libavcodec.AVPacket
}

// NewH264Decoder 创建新的H.264解码器
func NewH264Decoder() (*H264Decoder, error) {

	//os.Setenv("Path", os.Getenv("Path")+";./lib")
	ffcommon.SetAvutilPath("avutil-59.dll")
	ffcommon.SetAvcodecPath("avcodec-61.dll")
	ffcommon.SetAvdevicePath("avdevice-61.dll")
	ffcommon.SetAvfilterPath("avfilter-10.dll")
	ffcommon.SetAvformatPath("avformat-61.dll")
	ffcommon.SetAvpostprocPath("postproc-58.dll")
	ffcommon.SetAvswresamplePath("swresample-5.dll")
	ffcommon.SetAvswscalePath("swscale-8.dll")

	// 查找H.264解码器
	codec := libavcodec.AvcodecFindDecoder(libavcodec.AV_CODEC_ID_H264)
	if codec == nil {
		return nil, fmt.Errorf("找不到H.264解码器")
	}

	// 分配解码器上下文
	codecCtx := codec.AvcodecAllocContext3()
	if codecCtx == nil {
		return nil, fmt.Errorf("无法分配解码器上下文")
	}

	// 打开解码器
	if ret := codecCtx.AvcodecOpen2(codec, nil); ret < 0 {
		// 注意：这里使用示例代码中的方式释放资源
		codecCtx.AvcodecClose()
		return nil, fmt.Errorf("无法打开解码器: %d", ret)
	}

	// 分配帧
	yuvFrame := libavutil.AvFrameAlloc()
	if yuvFrame == nil {
		codecCtx.AvcodecClose()
		return nil, fmt.Errorf("无法分配帧")
	}

	// 分配packet
	pkt := libavcodec.AvPacketAlloc()
	if pkt == nil {
		codecCtx.AvcodecClose()
		libavutil.AvFrameFree(&yuvFrame)
		return nil, fmt.Errorf("无法分配AVPacket")
	}

	return &H264Decoder{
		codecCtx:       codecCtx,
		yuvFrame:       yuvFrame,
		pkt:            pkt,
		outputDataChan: make(chan []byte, 5),
	}, nil
}

// DecodeFrame 解码单个H.264帧
func (d *H264Decoder) DecodeFrame(h264Data []byte) ([]byte, error) {
	if len(h264Data) == 0 {
		return nil, fmt.Errorf("输入数据为空")
	}

	// 创建新packet并填充数据
	d.pkt.AvNewPacket(int32(len(h264Data)))
	copy((*[1 << 30]byte)(unsafe.Pointer(d.pkt.Data))[:len(h264Data):len(h264Data)], h264Data)
	d.pkt.Size = uint32(len(h264Data))

	// 发送数据包到解码器
	if ret := d.codecCtx.AvcodecSendPacket(d.pkt); ret < 0 {
		return nil, fmt.Errorf("发送数据包失败: %d", ret)
	}

	// 接收解码后的帧
	ret := d.codecCtx.AvcodecReceiveFrame(d.yuvFrame)
	if ret == -int32(libavutil.EAGAIN) || ret == int32(libavutil.AVERROR_EOF) {
		return nil, nil // 没有帧可接收
	} else if ret < 0 {
		return nil, fmt.Errorf("接收帧失败: %d", ret)
	}

	// 更新分辨率信息
	d.width = d.yuvFrame.Width
	d.height = d.yuvFrame.Height

	// 提取YUV数据
	yuvData := d.extractYUVData()

	// 重置packet
	d.pkt.AvPacketUnref()

	return yuvData, nil
}
func (ff *H264Decoder) RecvOutput() ([]byte, error) {
	select {
	case data := <-ff.outputDataChan:
		return data, nil
	case <-time.After(5 * time.Millisecond):
		return nil, errors.New("接收输出超时")
	}
}
func (m *H264Decoder) Write(buffer []byte) (n int, err error) {
	buf, err := m.DecodeFrame(buffer)
	if err == nil {
		select {
		case m.outputDataChan <- buf: // 尝试发送数据
			// 发送成功
		default:
			// 通道满，丢弃数据
			// 这里可以添加日志记录或其他处理逻辑
		}
	}
	return len(buffer), nil
}

// extractYUVData 从AVFrame提取YUV数据
func (d *H264Decoder) extractYUVData() []byte {
	if d.yuvFrame == nil || d.width <= 0 || d.height <= 0 {
		return nil
	}

	w := d.width
	h := d.height

	bytes := []byte{}

	// Y分量
	ptr := uintptr(unsafe.Pointer(d.yuvFrame.Data[0]))
	for j := int32(0); j < h; j++ {
		line := unsafe.Slice((*byte)(unsafe.Pointer(ptr)), int(w))
		bytes = append(bytes, line...)
		ptr += uintptr(d.yuvFrame.Linesize[0])
	}

	// U分量
	ptru := uintptr(unsafe.Pointer(d.yuvFrame.Data[1]))
	for j := int32(0); j < h/2; j++ {
		line := unsafe.Slice((*byte)(unsafe.Pointer(ptru)), int(w/2))
		bytes = append(bytes, line...)
		ptru += uintptr(d.yuvFrame.Linesize[1])
	}

	// V分量
	ptrv := uintptr(unsafe.Pointer(d.yuvFrame.Data[2]))
	for j := int32(0); j < h/2; j++ {
		line := unsafe.Slice((*byte)(unsafe.Pointer(ptrv)), int(w/2))
		bytes = append(bytes, line...)
		ptrv += uintptr(d.yuvFrame.Linesize[2])
	}

	return bytes
}

// YUV420PToRGBA 将YUV420P字节切片转换为RGBA字节切片
func (d *H264Decoder) YUV420PToRGBA(yuvData []byte) []byte {
	if yuvData == nil || d.width <= 0 || d.height <= 0 {
		return nil
	}

	w := int(d.width)
	h := int(d.height)

	// 检查数据长度
	expectedYUVSize := w * h * 3 / 2 // YUV420P格式
	if len(yuvData) < expectedYUVSize {
		return nil
	}

	// 提取YUV平面
	ySize := w * h
	uSize := (w / 2) * (h / 2)

	yData := yuvData[:ySize]
	uData := yuvData[ySize : ySize+uSize]
	vData := yuvData[ySize+uSize : ySize+uSize*2]

	// 创建RGBA数据缓冲区
	rgbaData := make([]byte, w*h*4)

	// 使用整数运算提高性能
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			// 计算索引
			yIdx := y*w + x
			uvY := y / 2
			uvX := x / 2
			uvIdx := uvY*(w/2) + uvX

			// 确保UV索引不越界
			if uvIdx >= len(uData) || uvIdx >= len(vData) {
				continue
			}

			// 获取YUV值（使用整数运算提高性能）
			Y := int(yData[yIdx])
			U := int(uData[uvIdx])
			V := int(vData[uvIdx])

			// 调整YUV范围（从有限范围到全范围）
			Y = (Y - 16) * 256 / 219 // Y: 16-235 -> 0-255
			U = U - 128              // U: 16-240 -> -112 to 112
			V = V - 128              // V: 16-240 -> -112 to 112

			// YUV到RGB转换 (ITU-R BT.601) - 整数版本
			R := (Y + 359*V/256)            // 1.402 * 256 ≈ 359
			G := (Y - 88*U/256 - 183*V/256) // 0.344 * 256≈88, 0.714 * 256≈183
			B := (Y + 454*U/256)            // 1.772 * 256≈454

			// 钳制值到0-255范围
			R = clampInt(R, 0, 255)
			G = clampInt(G, 0, 255)
			B = clampInt(B, 0, 255)

			// 设置RGBA像素
			rgbaIdx := yIdx * 4
			rgbaData[rgbaIdx] = uint8(R)   // R
			rgbaData[rgbaIdx+1] = uint8(G) // G
			rgbaData[rgbaIdx+2] = uint8(B) // B
			rgbaData[rgbaIdx+3] = 255      // A
		}
	}

	return rgbaData
}

func clampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// Close 关闭解码器并释放资源
func (d *H264Decoder) Close() {
	if d.yuvFrame != nil {
		libavutil.AvFrameFree(&d.yuvFrame)
	}
	if d.codecCtx != nil {
		d.codecCtx.AvcodecClose()
		// 注意：根据示例代码，这里没有调用额外的释放函数
	}
	if d.pkt != nil {
		libavcodec.AvPacketFree(&d.pkt)
	}
}

// GetResolution 获取当前分辨率
func (d *H264Decoder) GetResolution() (int32, int32) {
	return d.width, d.height
}
