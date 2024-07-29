package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/codecrafters-io/http-server-starter-go/app/client"
)

var tempDirectory string

func main() {
	if len(os.Args) > 2 {
		tempDirectory = os.Args[2]
	}

	router := client.NewServer()

	router.AddRoute("/", home, "GET")
	router.AddRoute("/echo/{str}", echo, "GET")
	router.AddRoute("/user-agent", userAgent, "GET")
	router.AddRoute("/files/{filename}", getFile, "GET")
	router.AddRoute("/files/{filename}", createFile, "POST")

	router.Start()
}

func home(_ client.HTTPRequest) client.HTTPResponse {
	return client.HTTPResponse{
		Code: client.StatusOK,
	}
}

func echo(request client.HTTPRequest) client.HTTPResponse {
	content := request.Url.Parameters["str"]

	return client.HTTPResponse{
		Code:    client.StatusOK,
		Headers: map[string]string{"Content-Type": "text/plain"},
		Body:    []byte(content),
	}
}

func userAgent(request client.HTTPRequest) client.HTTPResponse {
	content := request.Headers["User-Agent"]

	return client.HTTPResponse{
		Code:    client.StatusOK,
		Headers: map[string]string{"Content-Type": "text/plain"},
		Body:    []byte(content),
	}
}

func createFile(request client.HTTPRequest) client.HTTPResponse {
	path := request.Url.Parameters["filename"]

	os.WriteFile(fmt.Sprintf("/%s/%s", tempDirectory, path), request.Body, 0666)
	return client.HTTPResponse{
		Code: client.StatusCreated,
	}
}

func getFile(request client.HTTPRequest) client.HTTPResponse {
	path := request.Url.Parameters["filename"]

	if _, err := os.Stat(fmt.Sprintf("/%s/%s", tempDirectory, path)); errors.Is(err, os.ErrNotExist) {
		return client.HTTPResponse{
			Code: client.StatusNotFound,
		}
	}

	content, _ := os.ReadFile(fmt.Sprintf("/%s/%s", tempDirectory, path))
	return client.HTTPResponse{
		Code:    client.StatusOK,
		Headers: map[string]string{"Content-Type": "application/octet-stream"},
		Body:    []byte(content),
	}
}
