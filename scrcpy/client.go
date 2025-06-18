package scrcpy

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/dosgo/castX/castxServer"
)

type ScrcpyClient struct {
	controlConn net.Conn
	castx       *castxServer.Castx
}

func NewScrcpyClient(webPort int, peerName string, savaPath string, password string) *ScrcpyClient {
	scrcpyClient := &ScrcpyClient{}
	reversePort := 6000
	scrcpyClient.castx, _ = castxServer.Start(webPort, 0, 0, "", true, password, reversePort)
	scrcpyClient.InitAdb(peerName, savaPath, reversePort)
	return scrcpyClient
}
func (scrcpyClient *ScrcpyClient) getControlConn() net.Conn {
	return scrcpyClient.controlConn
}

func (scrcpyClient *ScrcpyClient) StartClient() {
	scrcpyClient.castx.WsServer.SetControlFun(func(controlData map[string]interface{}) {
		controlConn := scrcpyClient.getControlConn()
		if controlConn != nil {
			controlCall(controlConn, scrcpyClient.castx.Config, controlData)
		}
	})
	scrcpyClient.castx.SetControlConnectCall(func(c net.Conn) {
		scrcpyClient.controlConn = c
		handleControl(c)
	})

}

func (scrcpyClient *ScrcpyClient) Shutdown() {
	if scrcpyClient.castx != nil {
		scrcpyClient.castx.HttpServer.Shutdown()
	}
	if scrcpyClient.castx.WsServer != nil {
		scrcpyClient.castx.WsServer.Shutdown()
	}
	scrcpyClient.castx.CloseScrcpyReceiver()
}

// 处理控制数据（示例解析基本控制指令）
func handleControl(conn net.Conn) error {
	data := make([]byte, 1) // 创建1字节长度的切片
	for {

		n, err := conn.Read(data)
		if err != nil {
			fmt.Printf("handleControl err:%+v\n", err)
			return err
		}
		// 示例解析：第一个字节为事件类型
		if n < 1 {
			return errors.New("无效控制数据")
		}

		switch int(data[0]) {
		case TYPE_CLIPBOARD: //剪贴板变化
			var lenData = make([]byte, 4)
			io.ReadFull(conn, lenData)
			len := binary.BigEndian.Uint32(lenData)
			//剪贴板数据
			var clipboardData = make([]byte, len)
			io.ReadFull(conn, clipboardData)
		case TYPE_ACK_CLIPBOARD: //剪贴板变化确认:
			var lenData = make([]byte, 8)
			io.ReadFull(conn, lenData)
		case TYPE_UHID_OUTPUT:
			var idData = make([]byte, 2)
			io.ReadFull(conn, idData)
			var lenData = make([]byte, 2)
			io.ReadFull(conn, lenData)
			len := binary.BigEndian.Uint16(lenData)
			var clipboardData = make([]byte, len)
			io.ReadFull(conn, clipboardData)
		default:
			fmt.Printf("未知device类型: 0x%x\n", data[0])
		}
	}
}
