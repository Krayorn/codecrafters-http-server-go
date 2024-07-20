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

func addRoute(routes *[]Route, path string, callback func(HTTPRequest) HTTPResponse, method string) {
	*routes = append(*routes, Route{
		Callback: callback,
		Method:   method,
		Path:     path,
	})
}

func listenReq(conn net.Conn, routes []Route) {
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
		Code: StatusNotFound,
	}

	conn.Write(response.Write(request))
}

var tempDirectory string

func main() {
	// TODO: - read request more cleanly, http.ReadRequest
	// 		 - get rid of the routes array / and infinite for loop for the user and instead create HTTPListener struct ?

	if len(os.Args) > 2 {
		tempDirectory = os.Args[2]
	}

	l, err := net.Listen("tcp", "0.0.0.0:4221")
	if err != nil {
		fmt.Println("Failed to bind to port 4221")
		os.Exit(1)
	}

	routes := make([]Route, 0)

	addRoute(&routes, "/", home, "GET")
	addRoute(&routes, "/echo/{str}", echo, "GET")
	addRoute(&routes, "/user-agent", userAgent, "GET")
	addRoute(&routes, "/files/{filename}", getFile, "GET")
	addRoute(&routes, "/files/{filename}", createFile, "POST")

	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting connection: ", err.Error())
			os.Exit(1)
		}

		go listenReq(conn, routes)
	}

}

func home(_ HTTPRequest) HTTPResponse {
	return HTTPResponse{
		Code: StatusOK,
	}
}

func echo(request HTTPRequest) HTTPResponse {
	content := request.Url.Parameters["str"]

	return HTTPResponse{
		Code:    StatusOK,
		Headers: map[string]string{"Content-Type": "text/plain"},
		Body:    []byte(content),
	}
}

func userAgent(request HTTPRequest) HTTPResponse {
	content := request.Headers["User-Agent"]

	return HTTPResponse{
		Code:    StatusOK,
		Headers: map[string]string{"Content-Type": "text/plain"},
		Body:    []byte(content),
	}
}

func createFile(request HTTPRequest) HTTPResponse {
	path := request.Url.Parameters["filename"]

	os.WriteFile(fmt.Sprintf("/%s/%s", tempDirectory, path), request.Body, 0666)
	return HTTPResponse{
		Code: StatusCreated,
	}
}

func getFile(request HTTPRequest) HTTPResponse {
	path := request.Url.Parameters["filename"]

	if _, err := os.Stat(fmt.Sprintf("/%s/%s", tempDirectory, path)); errors.Is(err, os.ErrNotExist) {
		return HTTPResponse{
			Code: StatusNotFound,
		}
	}

	content, _ := os.ReadFile(fmt.Sprintf("/%s/%s", tempDirectory, path))
	return HTTPResponse{
		Code:    StatusOK,
		Headers: map[string]string{"Content-Type": "application/octet-stream"},
		Body:    []byte(content),
	}
}
