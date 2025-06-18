package main

import (
	"fmt"
	"net/http"
	_ "net/http/pprof" // 自动注册HTTP处理器

	"github.com/dosgo/castX/scrcpy"
)

func main() {
	go func() {
		http.ListenAndServe("localhost:6060", nil) // 默认端口6060
	}()
	scrcpyClient := scrcpy.NewScrcpyClient(8083, "test11", "", "1234561")
	scrcpyClient.StartClient()
	fmt.Scanln()
}
