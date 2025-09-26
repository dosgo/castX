package main

import (
	"os"

	"github.com/pion/webrtc/v4/pkg/media/oggreader"
)

func main() {
	f, _ := os.Open("test.ogg")
	oggObj, _, _ := oggreader.NewWith(f)

	for {
		pageData, _, err := oggObj.ParseNextPage()
		if err != nil {
			break
		}

	}
}
