package main

import (
	"fmt"
	"net"
	"os"
	"strings"
	"unicode/utf8"
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

	headers := make(map[string]string)

	fmt.Println(parts)

	for i := 1; i < len(parts); i++ {
		headerParts := strings.Split(parts[i], ": ")
		if len(headerParts) >= 2 {
			headers[headerParts[0]] = strings.Join(headerParts[1:], "")
		}
	}

	if requestLineParts[1] == "/" {
		conn.Write([]byte("HTTP/1.1 200 OK\r\n\r\n"))
		conn.Close()
	} else if strings.HasPrefix(requestLineParts[1], "/echo") {
		uriParts := strings.Split(requestLineParts[1], "/")
		if len(uriParts) > 3 {
			conn.Write([]byte("HTTP/1.1 404 Not Found\r\n\r\n"))
			conn.Close()
		}
		content := uriParts[2]
		contentLength := utf8.RuneCountInString((content))
		conn.Write([]byte(fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: %d\r\n\r\n%s", contentLength, content)))
		conn.Close()
	} else if strings.HasPrefix(requestLineParts[1], "/user-agent") {
		content := headers["User-Agent"]
		contentLength := utf8.RuneCountInString((content))

		conn.Write([]byte(fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: %d\r\n\r\n%s", contentLength, content)))
		conn.Close()
	} else {
		conn.Write([]byte("HTTP/1.1 404 Not Found\r\n\r\n"))
		conn.Close()
	}

}
