package main

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/dosgo/castX/castxClient"
)

func main() {
	castxClient := &castxClient.CastXClient{}
	castxClient.Start("ws://127.0.0.1:8081/ws", "123456", 1920)
	time.Sleep(time.Second * 5)
	sdpFile := "castXClientSdp.sdp"
	os.WriteFile(sdpFile, []byte(castxClient.GetRtpSdp()), 0644)
	cmd := exec.Command("ffplay",
		"-i", sdpFile,
		"-protocol_whitelist", "file,udp,rtp")
	// 执行命令并等待完成
	err := cmd.Run()
	if err != nil {
		fmt.Printf("ffplay 执行失败: %v\n", err)
		return
	}
	fmt.Scanln()
}
