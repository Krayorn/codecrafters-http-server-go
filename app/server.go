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
)

type HTTPRequest struct {
	Headers map[string]string
	Url     string
	Method  string
	Body    []byte
}

type HTTPResponse struct {
	Headers map[string]string
	Code    int
	Body    []byte
}

const (
	StatusOK      = 200
	StatusCreated = 201

	StatusNotFound = 404
)

func StatusText(code int) string {
	switch code {
	case StatusOK:
		return "OK"
	case StatusCreated:
		return "Created"
	case StatusNotFound:
		return "Not Found"
	}

	return ""
}

func (response HTTPResponse) Write(request HTTPRequest) []byte {
	str := fmt.Sprintf("HTTP/1.1 %d %s\r\n", response.Code, StatusText(response.Code))

	if encodingsStr, ok := request.Headers["Accept-Encoding"]; ok {
		encodings := strings.Split(encodingsStr, ", ")
		for _, encoding := range encodings {
			if encoding == "gzip" {
				var encodedContent bytes.Buffer
				gz := gzip.NewWriter(&encodedContent)
				if _, err := gz.Write(response.Body); err != nil {
					log.Fatal(err)
				}
				gz.Close()

				response.Headers["Content-Encoding"] = encoding
				response.Body = encodedContent.Bytes()
				break
			}
		}
	}

	for header, value := range response.Headers {
		str += fmt.Sprintf("%s: %s\r\n", header, value)
	}

	if len(response.Body) > 0 {
		str += fmt.Sprintf("Content-Length: %d\r\n", len(response.Body))
	}

	str += "\r\n"

	if len(response.Body) > 0 {
		str += string(response.Body)
	}

	return []byte(str)
}

var tempDirectory string

func main() {
	fmt.Println("Logs from your program will appear here!")
	// take inspiration from http.ReadRequest // readLine()
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

	response := HTTPResponse{
		Code: StatusNotFound,
	}
	if request.Url == "/" {
		response = HTTPResponse{
			Code: StatusOK,
		}
	} else if strings.HasPrefix(request.Url, "/echo") {
		uriParts := strings.Split(request.Url, "/")
		if len(uriParts) <= 3 {
			content := uriParts[2]

			response = HTTPResponse{
				Code:    StatusOK,
				Headers: map[string]string{"Content-Type": "text/plain"},
				Body:    []byte(content),
			}
		}
	} else if strings.HasPrefix(request.Url, "/user-agent") {
		content := request.Headers["User-Agent"]
		response = HTTPResponse{
			Code:    StatusOK,
			Headers: map[string]string{"Content-Type": "text/plain"},
			Body:    []byte(content),
		}
	} else if strings.HasPrefix(request.Url, "/files") {
		uriParts := strings.Split(request.Url, "/")
		if len(uriParts) <= 3 {
			path := uriParts[2]

			if request.Method == "GET" {
				if _, err := os.Stat(fmt.Sprintf("/%s/%s", tempDirectory, path)); errors.Is(err, os.ErrNotExist) {
					response = HTTPResponse{
						Code: StatusNotFound,
					}
				} else {
					content, _ := os.ReadFile(fmt.Sprintf("/%s/%s", tempDirectory, path))
					response = HTTPResponse{
						Code:    StatusOK,
						Headers: map[string]string{"Content-Type": "application/octet-stream"},
						Body:    []byte(content),
					}
				}
			} else if request.Method == "POST" {
				os.WriteFile(fmt.Sprintf("/%s/%s", tempDirectory, path), request.Body, 0666)
				response = HTTPResponse{
					Code: StatusCreated,
				}
			}
		}
	}

	conn.Write(response.Write(request))
}
