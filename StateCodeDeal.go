package main

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"strconv"
)

func Hijack(w http.ResponseWriter, ContentLength int64) (conn net.Conn, bufrw *bufio.ReadWriter, err error) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}
	conn, bufrw, err = hj.Hijack()
	if err != nil {
		fmt.Printf("Conn, bufrw Wrong? \n")
		return
	}
	bufrw.WriteString("HTTP/1.1 200 OK\r\n")
	bufrw.WriteString("Content-Type: application/octet-stream\r\n")
	bufrw.WriteString("Transfer-Encoding: chunked\r\n")
	bufrw.WriteString("Accept-Ranges: bytes\r\n")
	bufrw.WriteString("Connection: close\r\n") // 建议添加
	bufrw.WriteString("Cache-Control: no-cache\r\n")
	bufrw.WriteString("X-Proxy-Chunked: true\r\n")
	bufrw.WriteString("Content-Length: " + strconv.FormatInt(ContentLength, 10) + "\r\n") // 这个如果后面有问题就会去掉
	bufrw.WriteString("\r\n")

	// 刷新，Header 立即发送
	if err = bufrw.Flush(); err != nil {
		fmt.Printf("Error flushing headers after hijack: %v", err)
		conn.Close()
		return nil, nil, err
	}
	return conn, bufrw, nil
}
