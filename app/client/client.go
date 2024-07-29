package client

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
)

type Server struct {
	Routes []Route
}
type HTTPRequest struct {
	Headers map[string]string
	Url     URL
	Method  string
	Body    []byte
}

type URL struct {
	Original   string
	Parameters map[string]string
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

type HTTPResponse struct {
	Headers map[string]string
	Code    int
	Body    []byte
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

type Route struct {
	Callback func(HTTPRequest) HTTPResponse
	Method   string
	Path     string
}

func (server *Server) AddRoute(path string, callback func(HTTPRequest) HTTPResponse, method string) {
	server.Routes = append(server.Routes, Route{
		Callback: callback,
		Method:   method,
		Path:     path,
	})
}

func ListenReq(conn net.Conn, routes []Route) {
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
		Url: URL{
			Original: requestLineParts[1],
		},
		Headers: headers,
		Method:  requestLineParts[0],
		Body:    []byte(parts[1][:contentLength]),
	}
	uriParts := strings.Split(requestLineParts[1], "/")

ROUTELOOP:
	for _, route := range routes {
		if requestLineParts[0] != route.Method {
			continue
		}

		routeParts := strings.Split(route.Path, "/")

		parameters := make(map[string]string)
		if len(routeParts) != len(uriParts) {
			continue
		}

		for i := 0; i < len(routeParts); i++ {
			if strings.HasPrefix(routeParts[i], "{") && strings.HasSuffix(routeParts[i], "}") {
				parameters[routeParts[i][1:len(routeParts[i])-1]] = uriParts[i]
				continue
			}

			if routeParts[i] == uriParts[i] {
				continue
			}

			continue ROUTELOOP
		}

		request.Url.Parameters = parameters
		conn.Write(route.Callback(request).Write(request))
		return
	}

	response := HTTPResponse{
		Code:    StatusNotFound,
		Headers: map[string]string{},
	}

	conn.Write(response.Write(request))
}

func NewServer() Server {
	return Server{Routes: make([]Route, 0)}
}

func (server Server) Start() {
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

		go ListenReq(conn, server.Routes)
	}
}
