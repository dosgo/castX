package main

import (
	"github.com/abemedia/go-webview"
	_ "github.com/abemedia/go-webview/embedded" // embed native library
	"github.com/kbinani/screenshot"
)

func main() {
	w := webview.New(true)
	w.SetTitle("webview")
	bounds := screenshot.GetDisplayBounds(0)

	w.SetSize(bounds.Dx(), bounds.Dy(), webview.HintNone)
	//w.SetHtml("Hello World!")
	w.Navigate("http://192.168.221.147:8081")
	w.Run()
	w.Destroy()
}
