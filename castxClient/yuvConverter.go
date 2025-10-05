package castxClient

// YUVConverter 使用查表法优化的YUV420P到RGBA转换器
type YUVConverter struct {
	rgbaBuffer []byte

	// 预计算查表
	yTable  [256]int32
	rTable  [256]int32 // R分量查表 [V]
	gUTable [256]int32 // G分量U系数
	gVTable [256]int32 // G分量V系数
	bTable  [256]int32 // B分量查表 [U]
}

// NewYUVConverter 创建新的转换器实例
func NewYUVConverter() *YUVConverter {
	conv := &YUVConverter{}

	conv.initTables()
	return conv
}

// 修正查表初始化
func (conv *YUVConverter) initTables() {
	// Y分量查表 (16-235 -> 0-255)
	for y := 0; y < 256; y++ {
		if y < 16 {
			conv.yTable[y] = 0
		} else if y > 235 {
			conv.yTable[y] = 255
		} else {
			conv.yTable[y] = int32((y - 16) * 255 / 219)
		}
	}

	// UV分量查表 (正确的系数)
	for uv := 0; uv < 256; uv++ {
		// UV值转换为有符号(-128到127)
		u := float64(uv) - 128.0
		v := float64(uv) - 128.0

		// 使用浮点计算确保精度，然后转换为整数
		conv.rTable[uv] = int32(1.402 * v)  // R = 1.402 * V
		conv.gUTable[uv] = int32(0.344 * u) // G的U分量: -0.344 * U
		conv.gVTable[uv] = int32(0.714 * v) // G的V分量: -0.714 * V
		conv.bTable[uv] = int32(1.772 * u)  // B = 1.772 * U
	}
}

// ConvertYUV420PToRGBA 将YUV420P转换为RGBA
func (conv *YUVConverter) ConvertYUV420PToRGBA(yuvData []byte, width, height int) []byte {
	if yuvData == nil || width <= 0 || height <= 0 {
		return nil
	}

	expectedSize := width * height * 3 / 2
	if len(yuvData) < expectedSize {
		return nil
	}

	ySize := width * height
	uvSize := (width / 2) * (height / 2)

	yData := yuvData[:ySize]
	uData := yuvData[ySize : ySize+uvSize]
	vData := yuvData[ySize+uvSize : ySize+uvSize*2]

	// 确保缓冲区大小
	requiredSize := width * height * 4
	if cap(conv.rgbaBuffer) < requiredSize {
		conv.rgbaBuffer = make([]byte, requiredSize)
	}
	rgbaData := conv.rgbaBuffer[:requiredSize]

	// 逐像素转换
	for y := 0; y < height; y++ {
		yRow := y * width
		uvRow := (y / 2) * (width / 2)

		for x := 0; x < width; x++ {
			yIdx := yRow + x
			uvIdx := uvRow + (x / 2)

			if uvIdx >= uvSize {
				continue
			}

			Y := conv.yTable[yData[yIdx]]
			U := uData[uvIdx]
			V := vData[uvIdx]

			// 使用查表计算RGB分量
			R := Y + conv.rTable[V]
			G := Y - conv.gUTable[U] - conv.gVTable[V]
			B := Y + conv.bTable[U]

			// 钳制到0-255
			R = clampInt32(R, 0, 255)
			G = clampInt32(G, 0, 255)
			B = clampInt32(B, 0, 255)

			rgbaIdx := yIdx * 4
			// 尝试BGRA顺序（大多数图形API使用）
			rgbaData[rgbaIdx] = uint8(R)   // R
			rgbaData[rgbaIdx+1] = uint8(G) // G
			rgbaData[rgbaIdx+2] = uint8(B) // B
			rgbaData[rgbaIdx+3] = 255      // A
		}
	}

	return rgbaData
}

// clampInt32 钳制整数值到指定范围
func clampInt32(v, min, max int32) int32 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
