package server

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"maps"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

type Server struct {
	Routes      []Route
	SubRouters  []*Server
	Middlewares []func(Handler) Handler
	Prefix      string
}

func (server *Server) SubRouter(prefix string) *Server {
	subRouter := Server{
		Prefix: prefix,
	}

	server.SubRouters = append(server.SubRouters, &subRouter)

	return &subRouter
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
	Original        string
	Parameters      map[string]string
	QueryParameters map[string]string
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
	Request *HTTPRequest
}

func (response HTTPResponse) Write(w io.Writer) error {

	if _, err := io.WriteString(w, fmt.Sprintf("HTTP/1.1 %d %s\r\n", response.Code, StatusText(response.Code))); err != nil {
		return err
	}

	if encodingsStr := response.Request.Headers.Get("Accept-Encoding"); encodingsStr != "" {
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
		if _, err := io.WriteString(w, fmt.Sprintf("%s: %s\r\n", header, value)); err != nil {
			return err
		}
	}

	if len(response.Body) > 0 {
		if _, err := io.WriteString(w, fmt.Sprintf("Content-Length: %d\r\n", len(response.Body))); err != nil {
			return err
		}
	}

	if _, err := io.WriteString(w, "\r\n"); err != nil {
		return err
	}

	if len(response.Body) > 0 {
		if _, err := io.WriteString(w, string(response.Body)); err != nil {
			return err
		}
	}

	return nil
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

type Handler func(request HTTPRequest) HTTPResponse

func (server *Server) Use(middleware func(Handler) Handler) {
	server.Middlewares = append(server.Middlewares, middleware)
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

func match(request HTTPRequest, uriParts []string, server Server) (func(HTTPRequest) HTTPResponse, map[string]string, []func(Handler) Handler) {
ROUTELOOP:
	for _, route := range server.Routes {
		if request.Method != route.Method {
			continue
		}

		routeParts := strings.Split(route.Path, "/")[1:]

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

		return route.Callback, parameters, server.Middlewares
	}

SUBROUTERS:
	for _, subrouter := range server.SubRouters {
		prefixParts := strings.Split(subrouter.Prefix, "/")[1:]

		parametersPrefix := make(map[string]string)
		for i := 0; i < len(prefixParts); i++ {
			if strings.HasPrefix(prefixParts[i], "{") && strings.HasSuffix(prefixParts[i], "}") {
				parametersPrefix[prefixParts[i][1:len(prefixParts[i])-1]] = uriParts[i]
				continue
			}

			if prefixParts[i] == uriParts[i] {
				continue
			}

			continue SUBROUTERS
		}

		res, parameters, middlewares := match(request, uriParts[len(prefixParts):], *subrouter)
		if res != nil {
			maps.Copy(parametersPrefix, parameters)
			return res, parametersPrefix, append(server.Middlewares, middlewares...)
		}
	}

	return nil, map[string]string{}, server.Middlewares
}

func listenReq(conn net.Conn, server Server) {
	conn.SetReadDeadline(time.Now().Add(10 * time.Millisecond)) // ensure the connection is not hanging waiting for data for no reason

	rawReq := make([]byte, 0)
	for {
		buffer := make([]byte, 4096)
		n, err := conn.Read(buffer)
		if n > 0 {
			rawReq = append(rawReq, buffer[:n]...)
		}
		if n < 4096 || err != nil {
			break
		}
	}

	defer conn.Close()

	request, err := parseRequest(rawReq)
	if err != nil {
		return
	}

	parts := strings.Split(request.Url.Original, "?")
	uriParts := strings.Split(parts[0], "/")[1:]
	queryParameters := make(map[string]string)
	if len(parts) > 1 {
		for _, parameter := range strings.Split(parts[1], "&") {
			keyValue := strings.Split(parameter, "=")
			if len(keyValue) > 1 {
				queryParameters[keyValue[0]] = keyValue[1]
			} else {
				queryParameters[keyValue[0]] = "true"
			}
		}
	}

	callback, parameters, middlewares := match(*request, uriParts, server)

	if callback != nil {
		request.Url.Parameters = parameters
		request.Url.QueryParameters = queryParameters

		nextRequest := callback
		for i := len(middlewares) - 1; i >= 0; i-- {
			nextRequest = middlewares[i](nextRequest)
		}

		err = nextRequest(*request).Write(conn)
		if err != nil {
			fmt.Println("Error while writing the response", err)
		}
		return
	}

	nextRequest := func(req HTTPRequest) HTTPResponse {
		return HTTPResponse{
			Code:    StatusNotFound,
			Headers: make(Header),
			Request: &req,
		}
	}
	for i := len(middlewares) - 2; i >= 0; i-- {
		nextRequest = middlewares[i](nextRequest)
	}

	err = nextRequest(*request).Write(conn)
	if err != nil {
		fmt.Println("Error while writing the response", err)
	}
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

		go listenReq(conn, server)
	}
}
