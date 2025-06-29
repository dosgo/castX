package main

import (
	"fmt"

	"github.com/dosgo/castX/castxClient"
)

func main() {
	webRtcReceive := &castxClient.CastXClient{}
	webRtcReceive.Start("ws://127.0.0.1:8081/ws", "123456", 1920, true)
	// 保持运行
	fmt.Scanln()
}
