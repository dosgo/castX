package main

import (
	"fmt"
	"os"
	"time"

	"github.com/dosgo/castX/castxClient"
)

func main() {
	castxClient := &castxClient.CastXClient{}
	castxClient.Start("ws://127.0.0.1:8081/ws", "123456", 1920)
	time.Sleep(time.Second * 5)
	fmt.Printf("sdp:%s\r\n", castxClient.GetRtpSdp())
	os.WriteFile("test6.sdp", []byte(castxClient.GetRtpSdp()), 0644)
	// ffplay -i test.sdp -protocol_whitelist file,udp,rtp
	fmt.Scanln()
}
