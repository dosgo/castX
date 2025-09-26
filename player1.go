package main

import (
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/widget"
	vlc "github.com/adrg/libvlc-go/v3"
)

func main() {
	myApp := app.New()
	myWindow := myApp.NewWindow("视频播放器")

	// 创建播放器容器
	videoContainer := widget.NewCard("", "", nil)

	// 播放按钮
	playBtn := widget.NewButton("播放", func() {
		go func() {
			if err := vlc.Init(); err != nil {
				return
			}
			defer vlc.Release()

			player, _ := vlc.NewPlayer()
			defer player.Release()

			// 根据操作系统设置播放窗口
			switch runtime.GOOS {
			case "windows":
				player.SetHWND(videoContainer.Handle())
			case "darwin":
				player.SetNSObject(videoContainer.Handle())
			case "linux":
				player.SetXWindow(uint(videoContainer.Handle()))
			}

			_ = player.LoadMedia("video.mp4")
			player.Play()
		}()
	})

	// 布局
	myWindow.SetContent(fyne.NewContainerWithLayout(
		fyne.NewBorderLayout(nil, playBtn, nil, nil),
		videoContainer,
		playBtn,
	))

	myWindow.Resize(fyne.NewSize(800, 600))
	myWindow.ShowAndRun()
}
