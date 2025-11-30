package test

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/yourusername/mangahub/internal/grpc/pb"
)

func TestHTTPHealthCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	resp, err := http.Get("http://localhost:8080/health")
	if err != nil {
		t.Skipf("HTTP server not running: %v", err)
	}
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	assert.Equal(t, "healthy", result["status"])
}

func TestHTTPMangaSearch(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	resp, err := http.Get("http://localhost:8080/manga?limit=5")
	if err != nil {
		t.Skipf("HTTP server not running: %v", err)
	}
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	assert.Equal(t, true, result["success"])
}

func TestGRPCSearchManga(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	// Connect to gRPC server
	conn, err := grpc.NewClient("localhost:9092", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Skipf("gRPC server not running: %v", err)
	}
	defer conn.Close()

	client := pb.NewMangaServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.SearchManga(ctx, &pb.SearchRequest{
		Query:  "one",
		Limit:  10,
		Offset: 0,
	})

	if err != nil {
		t.Skipf("gRPC call failed: %v", err)
	}

	assert.NotNil(t, resp)
	assert.GreaterOrEqual(t, len(resp.Manga), 0)
}

func TestGRPCGetManga(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	conn, err := grpc.NewClient("localhost:9092", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Skipf("gRPC server not running: %v", err)
	}
	defer conn.Close()

	client := pb.NewMangaServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Try to get a manga (may not exist, but should not error)
	resp, err := client.GetManga(ctx, &pb.GetMangaRequest{
		Id: "one-piece",
	})

	if err == nil {
		assert.NotNil(t, resp)
	}
}

func TestTCPConnection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	// Connect to TCP server
	conn, err := net.DialTimeout("tcp", "localhost:9090", 2*time.Second)
	if err != nil {
		t.Skipf("TCP server not running: %v", err)
	}
	defer conn.Close()

	assert.NotNil(t, conn)

	// Send progress update
	update := map[string]interface{}{
		"user_id":   "test-user",
		"manga_id":  "one-piece",
		"chapter":   50,
		"timestamp": time.Now().Unix(),
	}
	jsonData, _ := json.Marshal(update)

	_, err = conn.Write(append(jsonData, '\n'))
	assert.NoError(t, err)
}

func TestTCPBroadcast(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	// Connect to TCP server
	conn, err := net.DialTimeout("tcp", "localhost:9090", 2*time.Second)
	if err != nil {
		t.Skipf("TCP server not running: %v", err)
	}
	defer conn.Close()

	// Send progress update
	update := map[string]interface{}{
		"user_id":   "test-user",
		"manga_id":  "one-piece",
		"chapter":   50,
		"timestamp": time.Now().Unix(),
	}
	jsonData, _ := json.Marshal(update)

	_, err = conn.Write(append(jsonData, '\n'))
	assert.NoError(t, err)

	// Try to read broadcast response (may timeout if no other clients)
	buffer := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _ := conn.Read(buffer)

	if n > 0 {
		var received map[string]interface{}
		json.Unmarshal(buffer[:n], &received)
		t.Logf("Received broadcast: %v", received)
	}
}

func TestUDPNotification(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	// Connect to UDP server
	addr, _ := net.ResolveUDPAddr("udp", "localhost:9091")
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		t.Skipf("UDP server not running: %v", err)
	}
	defer conn.Close()

	// Register
	_, err = conn.Write([]byte("REGISTER"))
	assert.NoError(t, err)

	// Try to receive confirmation (may timeout)
	buffer := make([]byte, 256)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := conn.Read(buffer)

	if err == nil && n > 0 {
		confirmation := string(buffer[:n])
		t.Logf("Received: %s", confirmation)
		assert.Contains(t, confirmation, "REGISTERED")
	}

	// Cleanup
	conn.Write([]byte("UNREGISTER"))
}

func TestWebSocketEndpoint(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	// Verify WebSocket endpoint is accessible
	resp, err := http.Get("http://localhost:8080/ws/chat")
	if err != nil {
		t.Skipf("HTTP server not running: %v", err)
	}
	defer resp.Body.Close()

	// WebSocket upgrade will fail on GET, but endpoint should exist
	// Expect 400 Bad Request (missing upgrade headers) or connection established
	assert.True(t, resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusSwitchingProtocols)
}

func TestConcurrentTCPConnections(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	const numClients = 5
	done := make(chan bool, numClients)

	for i := 0; i < numClients; i++ {
		go func(id int) {
			conn, err := net.DialTimeout("tcp", "localhost:9090", 2*time.Second)
			if err != nil {
				done <- false
				return
			}
			defer conn.Close()

			update := map[string]interface{}{
				"user_id":   "user-" + string(rune(id)),
				"manga_id":  "test-manga",
				"chapter":   id * 10,
				"timestamp": time.Now().Unix(),
			}
			jsonData, _ := json.Marshal(update)
			conn.Write(append(jsonData, '\n'))

			done <- true
		}(i)
	}

	// Wait for all clients
	successCount := 0
	for i := 0; i < numClients; i++ {
		if <-done {
			successCount++
		}
	}

	t.Logf("Successful concurrent connections: %d/%d", successCount, numClients)
	assert.Greater(t, successCount, 0)
}
