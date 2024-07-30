package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/codecrafters-io/http-server-starter-go/app/server"
)

var tempDirectory string

func main() {
	if len(os.Args) > 2 {
		tempDirectory = os.Args[2]
	}

	router := server.NewServer()

	router.AddRoute("/", home, "GET")
	router.AddRoute("/echo/{str}", echo, "GET")
	router.AddRoute("/user-agent", userAgent, "GET")
	router.AddRoute("/files/{filename}", getFile, "GET")
	router.AddRoute("/files/{filename}", createFile, "POST")

	router.Start()
}

func home(_ server.HTTPRequest) server.HTTPResponse {
	return server.HTTPResponse{
		Code: server.StatusOK,
	}
}

func echo(request server.HTTPRequest) server.HTTPResponse {
	content := request.Url.Parameters["str"]

	return server.HTTPResponse{
		Code:    server.StatusOK,
		Headers: map[string]string{"Content-Type": "text/plain"},
		Body:    []byte(content),
	}
}

func userAgent(request server.HTTPRequest) server.HTTPResponse {
	content := request.Headers["User-Agent"]

	return server.HTTPResponse{
		Code:    server.StatusOK,
		Headers: map[string]string{"Content-Type": "text/plain"},
		Body:    []byte(content),
	}
}

func createFile(request server.HTTPRequest) server.HTTPResponse {
	path := request.Url.Parameters["filename"]

	os.WriteFile(fmt.Sprintf("/%s/%s", tempDirectory, path), request.Body, 0666)
	return server.HTTPResponse{
		Code: server.StatusCreated,
	}
}

func getFile(request server.HTTPRequest) server.HTTPResponse {
	path := request.Url.Parameters["filename"]

	if _, err := os.Stat(fmt.Sprintf("/%s/%s", tempDirectory, path)); errors.Is(err, os.ErrNotExist) {
		return server.HTTPResponse{
			Code: server.StatusNotFound,
		}
	}

	content, _ := os.ReadFile(fmt.Sprintf("/%s/%s", tempDirectory, path))
	return server.HTTPResponse{
		Code:    server.StatusOK,
		Headers: map[string]string{"Content-Type": "application/octet-stream"},
		Body:    []byte(content),
	}
}
