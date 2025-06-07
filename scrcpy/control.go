package scrcpy

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"time"

	"github.com/dosgo/castX/comm"
)

var KEYCODE_BACK = 4
var KEYCODE_HOME = 3
var KEYCODE_MENU = 82

func mtRand(min int, max int) int {
	return rand.Intn(max-min+1) + min
}

func SendKeyCode(controlConn net.Conn, action byte, keycode uint32, repeat uint32, metaState uint32) {
	if controlConn != nil {
		if action != ACTION_DOWN && action != ACTION_UP {
			return
		}
		controlConn.Write([]byte{TYPE_INJECT_KEYCODE})
		controlConn.Write([]byte{action})

		binary.Write(controlConn, binary.BigEndian, keycode)
		binary.Write(controlConn, binary.BigEndian, repeat)
		binary.Write(controlConn, binary.BigEndian, metaState)
	}

}
func SendKTouchEvent(controlConn net.Conn, action byte, pointerId uint64, x uint32, y uint32, screenWidth uint16, screenHeight uint16, pressure uint16) {
	if controlConn != nil {
		buf := new(bytes.Buffer)

		buf.Write([]byte{TYPE_INJECT_TOUCH_EVENT})
		buf.Write([]byte{action})

		binary.Write(buf, binary.BigEndian, pointerId)

		binary.Write(buf, binary.BigEndian, x)
		binary.Write(buf, binary.BigEndian, y)

		binary.Write(buf, binary.BigEndian, screenWidth)
		binary.Write(buf, binary.BigEndian, screenHeight)

		binary.Write(buf, binary.BigEndian, pressure)

		//actionButton
		binary.Write(buf, binary.BigEndian, BUTTON_PRIMARY)
		//buttons
		binary.Write(buf, binary.BigEndian, BUTTON_PRIMARY)

		controlConn.Write(buf.Bytes())
	} else {
		fmt.Printf("SendKTouchEvent controlConn is nil\r\n")
	}
}

func SendScrollEvent(controlConn net.Conn, x uint32, y uint32, screenWidth uint16, screenHeight uint16, hScroll uint16, vScroll uint16) {
	if controlConn != nil {
		controlConn.Write([]byte{TYPE_INJECT_SCROLL_EVENT})

		binary.Write(controlConn, binary.BigEndian, x)
		binary.Write(controlConn, binary.BigEndian, y)
		binary.Write(controlConn, binary.BigEndian, screenWidth)
		binary.Write(controlConn, binary.BigEndian, screenHeight)

		binary.Write(controlConn, binary.BigEndian, hScroll)
		binary.Write(controlConn, binary.BigEndian, vScroll)

		//buttons
		binary.Write(controlConn, binary.BigEndian, BUTTON_PRIMARY)
	}
}

func SendDisplayPower(controlConn net.Conn, on byte) {
	if controlConn != nil {
		controlConn.Write([]byte{TYPE_SET_DISPLAY_POWER})
		controlConn.Write([]byte{on})
		fmt.Printf("SendKDisplayPower on:%d\r\n", on)
	}
}

func controlCall(controlConn net.Conn, config *comm.Config, controlData map[string]interface{}) {

	if controlData["type"] == "left" {
		if f, ok := controlData["x"].(float64); ok {
			x := uint32(f)
			y := uint32(controlData["y"].(float64))
			var pointerId uint64 = 0
			SendKTouchEvent(controlConn, ACTION_DOWN, pointerId, x, y, uint16(config.ScreenWidth), uint16(config.ScreenHeight), uint16(mtRand(100, 200)))
			time.Sleep(time.Millisecond * time.Duration(mtRand(50, 90))) // 等待100毫秒
			SendKTouchEvent(controlConn, ACTION_UP, pointerId, x, y, uint16(config.ScreenWidth), uint16(config.ScreenHeight), uint16(mtRand(100, 200)))
		}
	}
	if controlData["type"] == "swipe" {
		if code, ok := controlData["code"].(string); ok {
			fmt.Printf("code:%s\r\n", code)
			//SendScrollEvent(1, 100, 100)
		}
	}
	if controlData["type"] == "panstart" {
		if f, ok := controlData["x"].(float64); ok {
			x := uint32(f)
			y := uint32(controlData["y"].(float64))
			var pointerId uint64 = 0
			SendKTouchEvent(controlConn, ACTION_DOWN, pointerId, x, y, uint16(config.ScreenWidth), uint16(config.ScreenHeight), uint16(mtRand(100, 200)))
			fmt.Printf("panstart:%d,%d\r\n", x, y) // 打印 x 和 y 的值，用于调试，你可以根据需要修改打印 forma
		}
	}
	if controlData["type"] == "pan" {
		if f, ok := controlData["x"].(float64); ok {
			x := uint32(f)
			y := uint32(controlData["y"].(float64))
			var pointerId uint64 = 0
			SendKTouchEvent(controlConn, ACTION_MOVE, pointerId, x, y, uint16(config.ScreenWidth), uint16(config.ScreenHeight), uint16(mtRand(100, 200)))
			fmt.Printf("pan:%d,%d\r\n", x, y)
		}
	}
	if controlData["type"] == "panend" {
		if f, ok := controlData["x"].(float64); ok {
			x := uint32(f)
			y := uint32(controlData["y"].(float64))
			var pointerId uint64 = 0
			SendKTouchEvent(controlConn, ACTION_UP, pointerId, x, y, uint16(config.ScreenWidth), uint16(config.ScreenHeight), uint16(mtRand(100, 200)))
			fmt.Printf("panend:%d,%d\r\n", x, y)
		}
	}
	if controlData["type"] == "keyboard" {
		if _code, ok := controlData["code"].(float64); ok {
			SendKeyCode(controlConn, ACTION_DOWN, uint32(_code), 0, 0)
		}
		if _code, ok := controlData["code"].(string); ok {
			if _code == "home" {
				SendKeyCode(controlConn, ACTION_DOWN, uint32(KEYCODE_HOME), 0, 0)
				time.Sleep(time.Millisecond * 20) // 等待100毫秒，确保事件被处理
				SendKeyCode(controlConn, ACTION_UP, uint32(KEYCODE_HOME), 0, 0)

			}
			if _code == "back" {
				SendKeyCode(controlConn, ACTION_DOWN, uint32(KEYCODE_BACK), 0, 0)
				time.Sleep(time.Millisecond * 20)
				SendKeyCode(controlConn, ACTION_UP, uint32(KEYCODE_HOME), 0, 0)

			}
		}

	}
	if controlData["type"] == "displayPower" {
		if _on, ok := controlData["action"].(float64); ok {
			on := byte(_on)
			SendDisplayPower(controlConn, on)
		}
	}
}
