package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	// 定义命令行参数
	port := 8089
	dir := "./static/cardboard-vr" // 当前目录，可以修改为您的项目路径，如："C:/my-project"

	// 获取绝对路径
	absDir, err := filepath.Abs(dir)
	if err != nil {
		log.Fatalf("获取绝对路径失败: %v", err)
	}

	// 检查目录是否存在
	if _, err := os.Stat(absDir); os.IsNotExist(err) {
		log.Fatalf("目录不存在: %s", absDir)
	}

	// 设置文件服务器
	fs := http.FileServer(http.Dir(absDir))
	http.Handle("/", fs)

	// 启动服务器
	addr := fmt.Sprintf(":%d", port)
	log.Printf("服务器运行在 http://localhost%s", addr)
	log.Printf("服务目录: %s", absDir)
	log.Printf("按 Ctrl+C 停止服务器")

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}
