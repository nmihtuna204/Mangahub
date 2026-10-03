// Package main - TCP Protocol Manual Test
// Kết nối đến TCP server và gửi/nhận messages để test sync functionality
//
// Sends one progress update and prints every update the server relays (it
// relays to all clients, the sender included). With -listen it waits at most
// that long for its own update to come back and exits 1 if it doesn't, so
// scripts can use it.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
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
	listen := flag.Duration("listen", 0, "Wait at most this long for the update to come back, then exit (1 if it didn't); 0 = listen until Ctrl+C")
	flag.Parse()

	addr := net.JoinHostPort(*host, strconv.Itoa(*port))
	fmt.Printf("🔗 Connecting to TCP server at %s...\n", addr)

	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		fail("Connection failed: %v", err)
	}
	defer conn.Close()

	fmt.Println("✅ Connected!")

	update := ProgressUpdate{
		UserID:    *userID,
		MangaID:   *mangaID,
		Chapter:   *chapter,
		Timestamp: time.Now().Unix(),
	}
	data, _ := json.Marshal(update)
	fmt.Printf("\n📤 Sending message:\n%s\n", string(data))
	if _, err := conn.Write(append(data, '\n')); err != nil {
		fail("Send failed: %v", err)
	}
	fmt.Println("✅ Message sent!")

	if *listen > 0 {
		fmt.Printf("👂 Waiting up to %v for the server to relay it...\n", *listen)
		conn.SetReadDeadline(time.Now().Add(*listen))
	} else {
		fmt.Println("👂 Listening for relayed updates (Ctrl+C to quit)...")
	}

	gotOwn := false
	scanner := bufio.NewScanner(conn)
	for (!gotOwn || *listen == 0) && scanner.Scan() {
		line := scanner.Bytes()
		fmt.Printf("\n📥 Received: %s\n", string(line))

		var recv ProgressUpdate
		if err := json.Unmarshal(line, &recv); err == nil {
			fmt.Printf("   User: %s\n", recv.UserID)
			fmt.Printf("   Manga: %s\n", recv.MangaID)
			fmt.Printf("   Chapter: %d\n", recv.Chapter)
			fmt.Printf("   Time: %v\n", time.Unix(recv.Timestamp, 0))
			if recv == update {
				gotOwn = true
			}
		}
	}

	if *listen > 0 {
		if !gotOwn {
			fail("The update was not relayed back within %v (%v)", *listen, scanner.Err())
		}
		fmt.Println("\n✅ The server relayed the update")
		return
	}
	if scanner.Err() != nil {
		fail("Receive error: %v", scanner.Err())
	}
}

func fail(format string, args ...interface{}) {
	fmt.Printf("❌ "+format+"\n", args...)
	os.Exit(1)
}
