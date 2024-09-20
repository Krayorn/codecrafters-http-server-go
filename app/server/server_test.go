package server

import (
	"net"
	"testing"
	"time"
)

func TestServerStart(t *testing.T) {
	// Start the server

	router := NewServer()

	go router.Start()

	time.Sleep(200 * time.Millisecond)

	conn, err := net.Dial("tcp", "localhost:4221")
	if err != nil {
		t.Fatalf("Could not connect to server: %v", err)
	}
	defer conn.Close()

	t.Log("Successfully connected to the server")
}

func TestParseRequest(t *testing.T) {
	rawRequest := "GET /index.html HTTP/1.1\r\n" +
		"Host: www.example.com\r\n" +
		"User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36\r\n" +
		"Content-Length: 3\r\n" +
		"\r\n" +
		"abc"

	request, err := parseRequest([]byte(rawRequest))
	if err != nil {
		t.Errorf("Expected no error, got %s", err)
	}

	if request.Method != "GET" {
		t.Errorf("Expected method GET, got %s", request.Method)
	}

	if request.Url.Original != "/index.html" {
		t.Errorf("Expected path /index.html, got %s", request.Url.Original)
	}

	if request.Headers.Get("Host") != "www.example.com" {
		t.Errorf("Expected Host header www.example.com, got %s", request.Headers["Host"])
	}

	expectedUserAgent := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36"
	if request.Headers.Get("User-Agent") != expectedUserAgent {
		t.Errorf("Expected User-Agent header %s, got %s", expectedUserAgent, request.Headers["User-Agent"])
	}

	if string(request.Body) != "abc" {
		t.Errorf("Expected body %s, got %s", "abc", string(request.Body))
	}
}
