package main

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"
)

type HTTPRequest struct {
	Headers map[string]string
	Url     string
	Method  string
	Body    []byte
}

var tempDirectory string

func main() {
	fmt.Println("Logs from your program will appear here!")

	if len(os.Args) > 2 {
		tempDirectory = os.Args[2]
	}

	l, err := net.Listen("tcp", "0.0.0.0:4221")
	if err != nil {
		fmt.Println("Failed to bind to port 4221")
		os.Exit(1)
	}

	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting connection: ", err.Error())
			os.Exit(1)
		}

		go listenReq(conn)
	}

}

func listenReq(conn net.Conn) {
	rawReq := make([]byte, 4096)
	conn.Read(rawReq)

	defer conn.Close()

	parts := strings.Split(string(rawReq), "\r\n\r\n")
	metaParts := strings.Split(parts[0], "\r\n")
	requestLineParts := strings.Split(metaParts[0], " ")

	headers := make(map[string]string)
	for i := 1; i < len(metaParts); i++ {
		headerParts := strings.Split(metaParts[i], ": ")
		if len(headerParts) >= 2 {
			headers[headerParts[0]] = strings.Join(headerParts[1:], "")
		}
	}

	contentLength, _ := strconv.Atoi(headers["Content-Length"])

	request := HTTPRequest{
		Url:     requestLineParts[1],
		Headers: headers,
		Method:  requestLineParts[0],
		Body:    []byte(parts[1][:contentLength]),
	}

	if request.Url == "/" {
		conn.Write([]byte("HTTP/1.1 200 OK\r\n\r\n"))
	} else if strings.HasPrefix(request.Url, "/echo") {
		uriParts := strings.Split(request.Url, "/")
		if len(uriParts) > 3 {
			conn.Write([]byte("HTTP/1.1 404 Not Found\r\n\r\n"))
			return
		}
		content := uriParts[2]

		if encodingsStr, ok := request.Headers["Accept-Encoding"]; ok {
			encodings := strings.Split(encodingsStr, ", ")
			for _, encoding := range encodings {
				if encoding == "gzip" {
					var b bytes.Buffer
					gz := gzip.NewWriter(&b)
					if _, err := gz.Write([]byte(content)); err != nil {
						log.Fatal(err)
					}

					contentLength := len(b.Bytes())
					conn.Write([]byte(fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: %d\r\nContent-Encoding: %s\r\n\r\n%s", contentLength, encoding, b.Bytes())))
					return
				}
			}

		}

		contentLength := utf8.RuneCountInString(content)
		conn.Write([]byte(fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: %d\r\n\r\n%s", contentLength, content)))

	} else if strings.HasPrefix(request.Url, "/user-agent") {
		content := request.Headers["User-Agent"]
		contentLength := utf8.RuneCountInString(content)
		conn.Write([]byte(fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: %d\r\n\r\n%s", contentLength, content)))
	} else if strings.HasPrefix(request.Url, "/files") {
		uriParts := strings.Split(request.Url, "/")
		if len(uriParts) > 3 {
			conn.Write([]byte("HTTP/1.1 404 Not Found\r\n\r\n"))
			return
		}

		path := uriParts[2]

		if request.Method == "GET" {
			if _, err := os.Stat(fmt.Sprintf("/%s/%s", tempDirectory, path)); errors.Is(err, os.ErrNotExist) {
				conn.Write([]byte("HTTP/1.1 404 Not Found\r\n\r\n"))
				return
			}

			content, _ := os.ReadFile(fmt.Sprintf("/%s/%s", tempDirectory, path))
			contentLength := utf8.RuneCountInString(string(content))
			conn.Write([]byte(fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: application/octet-stream\r\nContent-Length: %d\r\n\r\n%s", contentLength, content)))
		} else if request.Method == "POST" {
			os.WriteFile(fmt.Sprintf("/%s/%s", tempDirectory, path), request.Body, 0666)
			conn.Write([]byte("HTTP/1.1 201 Created\r\n\r\n"))
		}
	} else {
		conn.Write([]byte("HTTP/1.1 404 Not Found\r\n\r\n"))
	}
}
