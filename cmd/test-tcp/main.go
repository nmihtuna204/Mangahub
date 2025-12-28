// Package main - TCP Protocol Manual Test
// Kết nối đến TCP server và gửi/nhận messages để test sync functionality
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"time"
)

type ProgressUpdate struct {
	UserID    string `json:"user_id"`
	MangaID   string `json:"manga_id"`
	Chapter   int    `json:"chapter"`
	Timestamp int64  `json:"timestamp"`
}

func main() {
	host := flag.String("host", "localhost", "TCP server host")
	port := flag.Int("port", 9090, "TCP server port")
	userID := flag.String("user", "test-user", "User ID")
	mangaID := flag.String("manga", "one-piece", "Manga ID")
	chapter := flag.Int("chapter", 100, "Chapter number")
	flag.Parse()

	addr := fmt.Sprintf("%s:%d", *host, *port)
	fmt.Printf("🔗 Connecting to TCP server at %s...\n", addr)

	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		fmt.Printf("❌ Connection failed: %v\n", err)
		return
	}
	defer conn.Close()

	fmt.Println("✅ Connected!")

	// Send test message
	update := ProgressUpdate{
		UserID:    *userID,
		MangaID:   *mangaID,
		Chapter:   *chapter,
		Timestamp: time.Now().Unix(),
	}

	data, _ := json.Marshal(update)
	fmt.Printf("\n📤 Sending message:\n%s\n", string(data))

	_, err = conn.Write(append(data, '\n'))
	if err != nil {
		fmt.Printf("❌ Send failed: %v\n", err)
		return
	}

	fmt.Println("✅ Message sent!\n")

	// Listen for responses (server may broadcast this to other clients)
	fmt.Println("👂 Listening for responses (Ctrl+C to quit)...")

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := scanner.Bytes()
		fmt.Printf("\n📥 Received: %s\n", string(line))

		var recv ProgressUpdate
		if err := json.Unmarshal(line, &recv); err == nil {
			fmt.Printf("   User: %s\n", recv.UserID)
			fmt.Printf("   Manga: %s\n", recv.MangaID)
			fmt.Printf("   Chapter: %d\n", recv.Chapter)
			fmt.Printf("   Time: %v\n", time.Unix(recv.Timestamp, 0))
		}
	}

	if scanner.Err() != nil {
		fmt.Printf("❌ Receive error: %v\n", scanner.Err())
	}
}
