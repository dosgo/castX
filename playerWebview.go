package main

import (
	"github.com/abemedia/go-webview"
	_ "github.com/abemedia/go-webview/embedded" // embed native library
)

func main() {
	w := webview.New(true)
	w.SetTitle("webview")
	w.SetSize(1920, 864, webview.HintNone)
	//w.SetHtml("Hello World!")
	w.Navigate("http://172.30.16.83:8081")
	w.Run()
	w.Destroy()
}
