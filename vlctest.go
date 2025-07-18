package main

import (
	"fmt"

	vlc "github.com/sunaipa5/libvlcPurego"
)

func main() {
	if err := vlc.Init(); err != nil {
		fmt.Printf("err:%+vr\n", err)
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
		for {

			<-vout
			fmt.Println("Vout Event Recivet")
		}
		//<-vout
		//fmt.Println("Vout Event Recivet")
	}()

	//windows: VLC (Direct3D11 output)
	//Linux:  VLC media Test_player
	//	closeChan := player.WindowCloseEvent("VLC media player")
	//	<-closeChan
	fmt.Println("Player window closed")

	fmt.Println("Player released")
	fmt.Scanln()
}
