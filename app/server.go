package main

import (
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"unicode/utf8"
)

type HTTPRequest struct {
	Headers map[string]string
	Url     string
}

func main() {
	fmt.Println("Logs from your program will appear here!")

	l, err := net.Listen("tcp", "0.0.0.0:4221")
	if err != nil {
		fmt.Println("Failed to bind to port 4221")
		os.Exit(1)
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go listenReq(l)
	}

	wg.Wait()
}

func listenReq(l net.Listener) {
	conn, err := l.Accept()
	if err != nil {
		fmt.Println("Error accepting connection: ", err.Error())
		os.Exit(1)
	}

	rawReq := make([]byte, 4096)
	conn.Read(rawReq)

	parts := strings.Split(string(rawReq), "\r\n")
	requestLineParts := strings.Split(parts[0], " ")
	headers := make(map[string]string)
	for i := 1; i < len(parts); i++ {
		headerParts := strings.Split(parts[i], ": ")
		if len(headerParts) >= 2 {
			headers[headerParts[0]] = strings.Join(headerParts[1:], "")
		}
	}

	request := HTTPRequest{
		Url:     requestLineParts[1],
		Headers: headers,
	}

	if request.Url == "/" {
		conn.Write([]byte("HTTP/1.1 200 OK\r\n\r\n"))
		conn.Close()
	} else if strings.HasPrefix(request.Url, "/echo") {
		uriParts := strings.Split(request.Url, "/")
		if len(uriParts) > 3 {
			conn.Write([]byte("HTTP/1.1 404 Not Found\r\n\r\n"))
			conn.Close()
		}
		content := uriParts[2]
		contentLength := utf8.RuneCountInString((content))
		conn.Write([]byte(fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: %d\r\n\r\n%s", contentLength, content)))
		conn.Close()
	} else if strings.HasPrefix(request.Url, "/user-agent") {
		content := request.Headers["User-Agent"]
		contentLength := utf8.RuneCountInString((content))

		conn.Write([]byte(fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: %d\r\n\r\n%s", contentLength, content)))
		conn.Close()
	} else {
		conn.Write([]byte("HTTP/1.1 404 Not Found\r\n\r\n"))
		conn.Close()
	}

	go listenReq(l)
}
