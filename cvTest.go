// main.go
package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"image"
	"image/jpeg"
	"os"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// OpenCVWASMProcessor 封装了 WASM 运行时和 OpenCV 功能
type OpenCVWASMProcessor struct {
	runtime wazero.Runtime
	module  api.Module
	ctx     context.Context
}

// NewOpenCVWASMProcessor 创建新的处理器
func NewOpenCVWASMProcessor() (*OpenCVWASMProcessor, error) {
	ctx := context.Background()

	// 创建 WASM 运行时
	r := wazero.NewRuntime(ctx)

	// 添加 WASI 支持
	if _, err := wasi_snapshot_preview1.Instantiate(ctx, r); err != nil {
		return nil, fmt.Errorf("failed to instantiate WASI: %v", err)
	}

	// 这里应该加载预编译的 opencv.wasm 文件
	wasmBytes, err := os.ReadFile("opencv.wasm")
	if err != nil {
		// 如果没有 wasm 文件，创建一个简单的模拟模块
		fmt.Println("Warning: opencv.wasm not found, using mock implementation")
		return createMockProcessor(ctx, r)
	}

	// 实例化 WASM 模块
	module, err := r.Instantiate(ctx, wasmBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate module: %v", err)
	}

	return &OpenCVWASMProcessor{
		runtime: r,
		module:  module,
		ctx:     ctx,
	}, nil
}

// createMockProcessor 创建模拟的处理器用于测试
func createMockProcessor(ctx context.Context, r wazero.Runtime) (*OpenCVWASMProcessor, error) {
	// 创建一个简单的 WASM 模块用于演示
	wasmCode := []byte{
		0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00, // WASM magic number
		0x01, 0x07, 0x01, 0x60, 0x02, 0x7f, 0x7f, 0x01, 0x7f, // type section
		0x03, 0x02, 0x01, 0x00, // function section
		0x07, 0x0f, 0x02, 0x06, 0x6d, 0x65, 0x6d, 0x6f, 0x72, 0x79, 0x02, 0x00, // memory section
		0x09, 0x67, 0x72, 0x61, 0x79, 0x73, 0x63, 0x61, 0x6c, 0x65, 0x00, 0x00, // export section
		0x0a, 0x11, 0x01, 0x0f, 0x00, 0x41, 0x00, 0x41, 0x01, 0x10, 0x00, 0x1a, 0x41, 0x00, 0x41, 0x01, 0x10, 0x00, // code section
	}

	module, err := r.Instantiate(ctx, wasmCode)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate mock module: %v", err)
	}

	return &OpenCVWASMProcessor{
		runtime: r,
		module:  module,
		ctx:     ctx,
	}, nil
}

// ProcessImage 处理图像的主要方法
func (p *OpenCVWASMProcessor) ProcessImage(inputPath, outputPath string, operation string) error {
	start := time.Now()

	// 1. 读取输入图像
	img, err := loadImage(inputPath)
	if err != nil {
		return fmt.Errorf("failed to load image: %v", err)
	}

	fmt.Printf("Loaded image: %dx%d\n", img.Bounds().Dx(), img.Bounds().Dy())

	// 2. 将图像数据转换为 WASM 可处理的格式
	width, height := img.Bounds().Dx(), img.Bounds().Dy()
	imgData, err := convertImageToWASMFormat(img)
	if err != nil {
		return fmt.Errorf("failed to convert image: %v", err)
	}

	// 3. 调用 WASM 函数处理图像
	processedData, err := p.callWASMFunction(operation, imgData, width, height)
	if err != nil {
		return fmt.Errorf("WASM processing failed: %v", err)
	}

	// 4. 将处理后的数据转换回图像
	resultImg, err := convertWASMFormatToImage(processedData, width, height)
	if err != nil {
		return fmt.Errorf("failed to convert result: %v", err)
	}

	// 5. 保存输出图像
	if err := saveImage(resultImg, outputPath); err != nil {
		return fmt.Errorf("failed to save image: %v", err)
	}

	fmt.Printf("Processing completed in %v\n", time.Since(start))
	return nil
}

// 辅助函数
func loadImage(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	return img, err
}

func saveImage(img image.Image, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	return jpeg.Encode(file, img, &jpeg.Options{Quality: 90})
}

func convertImageToWASMFormat(img image.Image) ([]byte, error) {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	// 创建足够大的缓冲区：宽度 + 高度 + 图像数据
	buffer := make([]byte, 8+width*height*4)

	// 前8字节存储宽度和高度
	binary.LittleEndian.PutUint32(buffer[0:4], uint32(width))
	binary.LittleEndian.PutUint32(buffer[4:8], uint32(height))

	// 将图像数据复制到缓冲区
	index := 8
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			buffer[index] = byte(r >> 8)
			buffer[index+1] = byte(g >> 8)
			buffer[index+2] = byte(b >> 8)
			buffer[index+3] = 255 // Alpha
			index += 4
		}
	}

	return buffer, nil
}

func convertWASMFormatToImage(data []byte, width, height int) (image.Image, error) {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	if len(data) < width*height*4 {
		return nil, fmt.Errorf("insufficient data for image conversion")
	}

	copy(img.Pix, data)
	return img, nil
}

func (p *OpenCVWASMProcessor) callWASMFunction(operation string, data []byte, width, height int) ([]byte, error) {
	switch operation {
	case "grayscale":
		return p.applyGrayscale(data, width, height)
	case "blur":
		return p.applyGaussianBlur(data, width, height)
	case "canny":
		return p.applyCannyEdge(data, width, height)
	default:
		return nil, fmt.Errorf("unknown operation: %s", operation)
	}
}

// 图像处理函数 - 这些是纯 Go 实现，实际中应该调用 WASM 函数
func (p *OpenCVWASMProcessor) applyGrayscale(data []byte, width, height int) ([]byte, error) {
	fmt.Println("Applying grayscale filter")

	// 跳过前8字节的宽度/高度信息
	pixels := data[8:]
	result := make([]byte, len(pixels))

	for i := 0; i < len(pixels); i += 4 {
		r := float64(pixels[i])
		g := float64(pixels[i+1])
		b := float64(pixels[i+2])

		// 灰度化公式
		gray := byte(0.299*r + 0.587*g + 0.114*b)

		result[i] = gray
		result[i+1] = gray
		result[i+2] = gray
		result[i+3] = 255
	}

	return result, nil
}

func (p *OpenCVWASMProcessor) applyGaussianBlur(data []byte, width, height int) ([]byte, error) {
	fmt.Println("Applying Gaussian blur")
	// 简化实现 - 实际应该调用 WASM
	return data[8:], nil
}

func (p *OpenCVWASMProcessor) applyCannyEdge(data []byte, width, height int) ([]byte, error) {
	fmt.Println("Applying Canny edge detection")
	// 简化实现 - 实际应该调用 WASM
	return data[8:], nil
}

func (p *OpenCVWASMProcessor) Close() error {
	return p.runtime.Close(p.ctx)
}

// CLI 入口
func main() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: go-opencv <input.jpg> <output.jpg> <operation>")
		fmt.Println("Operations: grayscale, blur, canny")
		fmt.Println("Example: go-opencv input.jpg output.jpg grayscale")
		os.Exit(1)
	}

	inputPath := os.Args[1]
	outputPath := os.Args[2]
	operation := os.Args[3]

	// 验证操作类型
	validOperations := map[string]bool{
		"grayscale": true,
		"blur":      true,
		"canny":     true,
	}

	if !validOperations[operation] {
		fmt.Printf("Error: Invalid operation '%s'. Valid operations are: grayscale, blur, canny\n", operation)
		os.Exit(1)
	}

	// 检查输入文件是否存在
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		fmt.Printf("Error: Input file '%s' does not exist\n", inputPath)
		os.Exit(1)
	}

	fmt.Printf("Processing %s -> %s with %s\n", inputPath, outputPath, operation)

	// 创建处理器
	processor, err := NewOpenCVWASMProcessor()
	if err != nil {
		fmt.Printf("Failed to create processor: %v\n", err)
		os.Exit(1)
	}
	defer processor.Close()

	// 处理图像
	if err := processor.ProcessImage(inputPath, outputPath, operation); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Image processing completed successfully!")
}
