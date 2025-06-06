package main

import (
	"fmt"

	"github.com/dosgo/castX/scrcpy"
)

func main() {
	scrcpyClient := scrcpy.NewScrcpyClient(8083, "test11", "", "1234561")
	scrcpyClient.StartClient()
	fmt.Scanln()
}
