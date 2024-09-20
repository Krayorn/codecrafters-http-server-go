package server

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

type Header map[string][]string

func (header Header) Get(key string) string {
	if values, ok := header[strings.ToUpper(key)]; ok && len(values) > 0 {
		return values[0]
	}
	return ""
}

func (header Header) Set(key string, value string) {
	header[strings.ToUpper(key)] = []string{value}
}

func (header Header) Add(key string, value string) {
	header[strings.ToUpper(key)] = append(header[strings.ToUpper(key)], value)
}

type HTTPRequest struct {
	Headers Header
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
	Headers Header
	Code    int
	Body    []byte
}

func (response HTTPResponse) Write(request HTTPRequest) []byte {
	str := fmt.Sprintf("HTTP/1.1 %d %s\r\n", response.Code, StatusText(response.Code))

	if encodingsStr := request.Headers.Get("Accept-Encoding"); encodingsStr != "" {
		encodings := strings.Split(encodingsStr, ", ")
		for _, encoding := range encodings {
			if encoding == "gzip" {
				var encodedContent bytes.Buffer
				gz := gzip.NewWriter(&encodedContent)
				if _, err := gz.Write(response.Body); err != nil {
					log.Fatal(err)
				}
				gz.Close()

				response.Headers.Set("Content-Encoding", encoding)
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

func parseRequest(rawReq []byte) (*HTTPRequest, error) {
	parts := strings.Split(string(rawReq), "\r\n\r\n")
	metaParts := strings.Split(parts[0], "\r\n")
	requestLineParts := strings.Split(metaParts[0], " ")

	if len(requestLineParts) < 2 {
		return nil, fmt.Errorf("request line does not contain enough parts")
	}

	headers := make(Header)
	for i := 1; i < len(metaParts); i++ {
		headerParts := strings.Split(metaParts[i], ": ")
		if len(headerParts) >= 2 {
			headerCanonical := strings.ToUpper(headerParts[0])
			if _, ok := headers[headerCanonical]; !ok {
				headers[headerCanonical] = make([]string, 0)
			}
			headers[headerCanonical] = append(headers[headerCanonical], strings.Join(headerParts[1:], ""))
		}
	}

	contentLength, err := strconv.Atoi(headers.Get("Content-Length"))
	if err != nil {
		contentLength = 0
	}

	body := []byte{}
	if len(parts) > 1 {
		body = []byte(parts[1][:contentLength])
	}

	return &HTTPRequest{
		Url: URL{
			Original: requestLineParts[1],
		},
		Headers: headers,
		Method:  requestLineParts[0],
		Body:    body,
	}, nil
}

func listenReq(conn net.Conn, routes []Route) {
	rawReq := make([]byte, 4096)
	conn.Read(rawReq)

	defer conn.Close()

	request, err := parseRequest(rawReq)
	if err != nil {
		return
	}

	uriParts := strings.Split(request.Url.Original, "/")

ROUTELOOP:
	for _, route := range routes {
		if request.Method != route.Method {
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
		conn.Write(route.Callback(*request).Write(*request))
		return
	}

	response := HTTPResponse{
		Code:    StatusNotFound,
		Headers: make(Header),
	}

	conn.Write(response.Write(*request))
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

		go listenReq(conn, server.Routes)
	}
}
