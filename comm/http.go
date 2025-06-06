package comm

import (
	"fmt"
	"net"
	"net/http"

	"github.com/dosgo/castX/static"
)

type HttpServer struct {
	server *http.Server
}

func StartWeb(port int, wsServer *WsServer) (*HttpServer, error) {
	httpServer := &HttpServer{}
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", wsServer.handleWebSocket)
	mux.HandleFunc("/usbWs", wsServer.handleWebSocket)
	mux.Handle("/", http.FileServer(http.FS(static.StaticFiles)))
	httpServer.server = &http.Server{Addr: fmt.Sprintf(":%d", port), Handler: mux}
	fmt.Printf("StartWeb port:%d\r\n", port)
	go httpServer.server.ListenAndServe()
	return httpServer, nil
}

func (httpServer *HttpServer) Shutdown() {
	if httpServer.server != nil {
		httpServer.server.Shutdown(nil)
		httpServer.server.Close()
		httpServer.server = nil
	}
}

func isPrivateIPv4(ipAddr string) bool {
	ipStr, _, err := net.SplitHostPort(ipAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(ipStr)
	if ip = ip.To4(); ip == nil {
		return false // 不是IPv4
	}
	return ip.IsPrivate() || // 192.168.0.0/16
		ip.IsLoopback() // 127.0.0.0/8
}
