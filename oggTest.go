package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/dosgo/castX/castxClient"
	"github.com/dosgo/libopus/opus"
	"github.com/pion/webrtc/v4/pkg/media/oggreader"
)

func main() {
	opusToPcm()
}

func oggRead() {
	f, _ := os.Open("test.ogg")
	oggObj, _, _ := oggreader.NewWith(f)

	for {
		pageData, _, err := oggObj.ParseNextPage()
		if err != nil {
			break
		}
		fmt.Printf("len:%d pageData:%+v\r\n", len(pageData), pageData)

	}
}
func opusToPcm() {

	decoder, _ := opus.NewOpusDecoder(48000, 2)
	fileIn, err := os.Open("testnew.opus")
	if err != nil {
		panic(err)
	}
	i := 0
	for {

		lengthBuf := make([]byte, 4)
		if _, err := io.ReadFull(fileIn, lengthBuf); err != nil {
			return
		}

		// 2. 将长度前缀转换为uint32
		length := binary.LittleEndian.Uint32(lengthBuf)

		inBuf := make([]byte, length)

		_, err := io.ReadFull(fileIn, inBuf)
		if err != nil {
			break
		}

		var pcm = make([]int16, 960*9)

		_len, err := decoder.Decode(inBuf, 0, len(inBuf), pcm, 0, len(pcm), false)
		fmt.Printf("length:%d _len:%d\r\n", length, _len)
		fmt.Printf("pcm:%+v\r\n", pcm[:_len*2])
		if err == nil {
			ddd := castxClient.ManualWriteInt16(pcm[:_len])
			castxClient.AppendFile("test111.pcm", ddd, 0664, false)
		} else {
			fmt.Printf("err:%+v\r\n", err)
		}
		i++

	}

}
