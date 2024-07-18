package main

import (
	"fmt"
	"net"
	"os"
	"strings"
)

func main() {
	fmt.Println("Logs from your program will appear here!")

	l, err := net.Listen("tcp", "0.0.0.0:4221")
	if err != nil {
		fmt.Println("Failed to bind to port 4221")
		os.Exit(1)
	}
	conn, err := l.Accept()
	if err != nil {
		fmt.Println("Error accepting connection: ", err.Error())
		os.Exit(1)
	}

	req := make([]byte, 4096)
	conn.Read(req)

	parts := strings.Split(string(req), "\r\n")

	requestLineParts := strings.Split(parts[0], " ")
	if requestLineParts[1] != "/" {
		conn.Write([]byte("HTTP/1.1 404 Not Found\r\n\r\n"))
		conn.Close()
	}

	conn.Write([]byte("HTTP/1.1 200 OK\r\n\r\n"))
	conn.Close()
}
