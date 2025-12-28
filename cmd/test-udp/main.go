// Package main - UDP Protocol Manual Test
// Gửi/nhận UDP notifications để test push notification functionality
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"time"
)

type Notification struct {
	Type      string `json:"type"`
	MangaID   string `json:"manga_id"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
}

func main() {
	host := flag.String("host", "localhost", "UDP server host")
	port := flag.Int("port", 9095, "UDP server port")
	mangaID := flag.String("manga", "one-piece", "Manga ID")
	message := flag.String("msg", "New chapter released!", "Notification message")
	notifType := flag.String("type", "chapter_release", "Notification type (chapter_release, system)")
	flag.Parse()

	serverAddr := fmt.Sprintf("%s:%d", *host, *port)
	fmt.Printf("📡 UDP Server: %s\n", serverAddr)

	// Resolve server address
	serverUDP, err := net.ResolveUDPAddr("udp", serverAddr)
	if err != nil {
		fmt.Printf("❌ Failed to resolve server address: %v\n", err)
		return
	}

	// Create local socket
	conn, err := net.ListenUDP("udp", nil)
	if err != nil {
		fmt.Printf("❌ Failed to create local socket: %v\n", err)
		return
	}
	defer conn.Close()

	fmt.Printf("✅ Connected! Local port: %s\n\n", conn.LocalAddr())

	// Register with server
	fmt.Println("📝 Registering with server...")
	_, err = conn.WriteToUDP([]byte("REGISTER"), serverUDP)
	if err != nil {
		fmt.Printf("❌ Registration failed: %v\n", err)
		return
	}

	// Wait for confirmation
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buffer := make([]byte, 1024)
	n, _, err := conn.ReadFromUDP(buffer)
	if err == nil {
		fmt.Printf("✅ Server response: %s\n\n", string(buffer[:n]))
	}

	// Send test notification
	notification := Notification{
		Type:      *notifType,
		MangaID:   *mangaID,
		Message:   *message,
		Timestamp: time.Now().Unix(),
	}

	data, _ := json.Marshal(notification)
	fmt.Printf("📤 Sending notification:\n%s\n\n", string(data))

	// Send notification to server
	_, err = conn.WriteToUDP(data, serverUDP)
	if err != nil {
		fmt.Printf("❌ Send failed: %v\n", err)
		return
	}

	fmt.Println("✅ Notification sent!\n")

	// Listen for notifications
	fmt.Println("👂 Listening for incoming notifications (Ctrl+C to quit)...")
	conn.SetReadDeadline(time.Time{}) // Remove read deadline

	for {
		buffer := make([]byte, 2048)
		n, remoteAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			fmt.Printf("❌ Receive error: %v\n", err)
			break
		}

		// Ignore our own message
		if string(buffer[:n]) == string(data) {
			continue
		}

		fmt.Printf("\n📥 Notification from %s:\n%s\n", remoteAddr.String(), string(buffer[:n]))

		var notif Notification
		if err := json.Unmarshal(buffer[:n], &notif); err == nil {
			fmt.Printf("   Type: %s\n", notif.Type)
			fmt.Printf("   Manga: %s\n", notif.MangaID)
			fmt.Printf("   Message: %s\n", notif.Message)
			fmt.Printf("   Time: %v\n", time.Unix(notif.Timestamp, 0))
		}
	}
}
