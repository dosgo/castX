package main

import (
	"fmt"

	"github.com/dosgo/castX/comm"
)

func main() {
	webRtcReceive := &comm.WebrtcReceive{}
	webRtcReceive.SetReceiveCall(func(cmd int, data []byte, timestamp int64) {
		fmt.Printf("test")
	})
	webRtcReceive.StartWebRtcReceive("http://127.0.0.1:8081/sendOffer", true)
	fmt.Printf("eee\r\n")
	// 保持运行
	select {}
}
