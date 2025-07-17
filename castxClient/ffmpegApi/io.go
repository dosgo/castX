package ffmpegapi

import (
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

type FfmpegIo struct {
	input          net.Listener
	output         net.Listener
	inputConn      net.Conn
	inputSps       bool
	outputDataChan chan []byte
	frameSize      int
	running        bool
}

func NewFfmpegIo() *FfmpegIo {
	ffmpegIo := &FfmpegIo{running: true, outputDataChan: make(chan []byte, 1)}
	ffmpegIo.input, _ = net.Listen("tcp", "127.0.0.1:0")
	ffmpegIo.output, _ = net.Listen("tcp", "127.0.0.1:0")
	go ffmpegIo.acceptIo(ffmpegIo.input, true)
	go ffmpegIo.acceptIo(ffmpegIo.output, false)
	return ffmpegIo
}
func (m *FfmpegIo) GetPort(isIn bool) int {
	if isIn {
		return m.input.Addr().(*net.TCPAddr).Port
	} else {
		return m.output.Addr().(*net.TCPAddr).Port
	}
}

// 监听指定端口
func (m *FfmpegIo) acceptIo(listener net.Listener, isIn bool) {
	for {
		// 接受新连接
		conn, err := listener.Accept()
		if err != nil {
			fmt.Printf("接受连接失败: %v\n", err)
		}
		if listener == m.input {
			m.inputConn = conn
			m.inputSps = false
		}
		// 处理连接
		go m.handler(conn, isIn)
	}
}
func (m *FfmpegIo) handler(conn net.Conn, isIn bool) {
	var buffer []byte
	defer conn.Close()
	if isIn {
		buffer = make([]byte, 1024*1024)
	} else {
		buffer = make([]byte, 1024*1024*10)
	}
	for m.running {
		if isIn {
			_, err := conn.Read(buffer)
			if err != nil {
				break
			}
		} else {
			n, err := io.ReadFull(conn, buffer[:m.frameSize])
			if err != nil {
				break
			}
			// 为了避免数据竞争，我们复制数据并发送到通道
			data := make([]byte, n)
			copy(data, buffer[:n])
			select {
			case m.outputDataChan <- data:
			case <-time.After(time.Millisecond * 500):
				fmt.Println("outputDataChan timeout")
			}
			m.outputDataChan <- data
		}
	}
}

/*兼容io.Writer*/
func (m *FfmpegIo) Write(buffer []byte) (n int, err error) {
	nalType := buffer[4] & 0x1F // 取低5位
	//有sps才开始写入不然解码器会报错
	if nalType == 7 {
		m.inputSps = true
	}
	return m.SendInput(buffer)
}

func (m *FfmpegIo) SendInput(buffer []byte) (n int, err error) {
	if m.inputConn != nil && m.inputSps {
		return m.inputConn.Write(buffer)
	}
	return len(buffer), nil
}

func (ff *FfmpegIo) RecvOutput() ([]byte, error) {
	select {
	case data := <-ff.outputDataChan:
		return data, nil
	case <-time.After(5 * time.Millisecond):
		return nil, errors.New("接收输出超时")
	}
}
func (ff *FfmpegIo) SetFrameSize(frameSize int) {
	ff.frameSize = frameSize
}
func (m *FfmpegIo) Stop() {
	m.input.Close()
	m.output.Close()
	m.running = false
}
