package main

import (
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"

	"github.com/dosgo/castX/scrcpy"
)

func main() {
	StartHTTPServer()
	scrcpyClient := scrcpy.NewScrcpyClient(8083, "test11", "", "1234561")
	scrcpyClient.StartClient()
	fmt.Scanln()
}

func StartHTTPServer() {
	// 在独立goroutine中启动pprof
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	// 原有HTTP服务器启动代码...
}
