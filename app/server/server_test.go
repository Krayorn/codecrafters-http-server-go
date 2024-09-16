package server_test

import (
	"net"
	"testing"
	"time"

	"github.com/krayorn/http-server-starter-go/app/server"
)

func TestServerStart(t *testing.T) {
	// Start the server

	router := server.NewServer()

	go router.Start()

	// Give the server a moment to start
	time.Sleep(100 * time.Millisecond)

	// Try to connect to the server
	conn, err := net.Dial("tcp", "localhost:4221")
	if err != nil {
		t.Fatalf("Could not connect to server: %v", err)
	}
	defer conn.Close()

	t.Log("Successfully connected to the server")
}
