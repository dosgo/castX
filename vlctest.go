package main

import (
	"fmt"

	vlc "github.com/sunaipa5/libvlcPurego"
	"golang.org/x/sys/windows"
)

func main() {
	if err := vlc.Init(); err != nil {
		panic(err)
		return
	}

	player, err := vlc.NewPlayer()
	if err != nil {
		fmt.Printf("Failed to create player: %v", err)
		return
	}
	defer player.Release()

	if err := player.NewSource("screen://"); err != nil {
		fmt.Printf("Failed to set source: %v\r\n", err)
		return
	}

	player.Play()

	eventManager, err := vlc.NewEventManager(player.Player)
	if err != nil {
		panic(err)
	}
	fmt.Println("Event Manager Created")

	vout := make(chan struct{})
	eventid, err := eventManager.EventListenerOld(vlc.MediaPlayerVout, vout)
	if err != nil {
		panic(err)
	}
	fmt.Println(eventid)

	go func() {
		<-vout
		fmt.Println("Vout Event Recivet")
	}()

	//windows: VLC (Direct3D11 output)
	//Linux:  VLC media Test_player
	closeChan := player.WindowCloseEvent("VLC media player")
	<-closeChan
	fmt.Println("Player window closed")
	player.Release()
	fmt.Println("Player released")
}
func createNamedPipe(name string) (windows.Handle, error) {
	// 转换字符串为 UTF16
	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return 0, err
	}

	// 创建命名管道
	pipe, err := windows.CreateNamedPipe(
		namePtr,
		windows.PIPE_ACCESS_OUTBOUND, // 输出管道
		windows.PIPE_TYPE_BYTE|windows.PIPE_WAIT,
		1,     // 只有一个实例
		65536, // 输出缓冲区大小
		65536, // 输入缓冲区大小
		0,     // 默认超时时间
		nil,   // 默认安全属性
	)
	return pipe, err
}
