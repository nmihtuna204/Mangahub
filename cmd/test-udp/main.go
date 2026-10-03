// Package main - UDP Protocol Manual Test
// Gửi/nhận UDP notifications để test push notification functionality
//
// Registers with the notification server, sends one notification as a
// "BROADCAST <json>" request (it used to send bare JSON, which the server
// ignores) and prints every notification that arrives. With -listen it waits
// at most that long for its own notification to come back and exits 1 if it
// doesn't, so scripts can use it.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"time"

	"mangahub/internal/udp"
)

func main() {
	host := flag.String("host", "localhost", "UDP server host")
	port := flag.Int("port", 9091, "UDP server port")
	mangaID := flag.String("manga", "one-piece", "Manga ID")
	message := flag.String("msg", "New chapter released!", "Notification message")
	notifType := flag.String("type", "chapter_release", "Notification type (chapter_release, system)")
	listen := flag.Duration("listen", 0, "Wait at most this long for the notification to come back, then exit (1 if it didn't); 0 = listen until Ctrl+C")
	flag.Parse()

	serverAddr := fmt.Sprintf("%s:%d", *host, *port)
	fmt.Printf("📡 UDP Server: %s\n", serverAddr)

	raddr, err := net.ResolveUDPAddr("udp", serverAddr)
	if err != nil {
		fail("Failed to resolve server address: %v", err)
	}
	// A connected socket: the server sends notifications to the address we
	// registered from
	conn, err := net.DialUDP("udp", nil, raddr)
	if err != nil {
		fail("Failed to create socket: %v", err)
	}
	defer conn.Close()
	fmt.Printf("✅ Local address: %s\n\n", conn.LocalAddr())

	fmt.Println("📝 Registering with server...")
	if _, err := conn.Write([]byte("REGISTER")); err != nil {
		fail("Registration failed: %v", err)
	}
	buf := make([]byte, 65536)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := conn.Read(buf)
	if err != nil {
		fail("No reply to REGISTER (is the UDP server running?): %v", err)
	}
	if reply := string(buf[:n]); reply != "REGISTERED" {
		fail("Unexpected reply to REGISTER: %q", reply)
	}
	fmt.Println("✅ Server response: REGISTERED")

	sent := udp.Notification{Type: *notifType, MangaID: *mangaID, Message: *message, Timestamp: time.Now().Unix()}
	req, err := udp.BroadcastRequest(sent)
	if err != nil {
		fail("Encoding failed: %v", err)
	}
	fmt.Printf("\n📤 Sending: %s\n", req)
	if _, err := conn.Write(req); err != nil {
		fail("Send failed: %v", err)
	}
	fmt.Println("✅ Notification sent!")

	// Ctrl+C: unsubscribe before leaving (the server would otherwise forget
	// this address only after its subscriber TTL)
	interrupted := make(chan os.Signal, 1)
	signal.Notify(interrupted, os.Interrupt)
	go func() {
		<-interrupted
		conn.SetReadDeadline(time.Now()) // ends the read loop below
	}()

	if *listen > 0 {
		fmt.Printf("👂 Waiting up to %v for it to come back...\n", *listen)
		conn.SetReadDeadline(time.Now().Add(*listen))
	} else {
		fmt.Println("👂 Listening for incoming notifications (Ctrl+C to quit)...")
		conn.SetReadDeadline(time.Time{})
	}

	gotOwn := false
	for !gotOwn || *listen == 0 {
		n, err := conn.Read(buf)
		if err != nil {
			var ne net.Error
			if !errors.As(err, &ne) || !ne.Timeout() {
				fmt.Printf("❌ Receive error: %v\n", err)
			}
			break
		}
		raw := buf[:n]
		if s := string(raw); s == "REGISTERED" || s == "UNREGISTERED" {
			continue
		}
		var got udp.Notification
		if err := json.Unmarshal(raw, &got); err != nil {
			fmt.Printf("\n📥 %s\n", raw)
			continue
		}
		fmt.Printf("\n📥 Notification:\n")
		fmt.Printf("   Type: %s\n", got.Type)
		fmt.Printf("   Manga: %s\n", got.MangaID)
		fmt.Printf("   Message: %s\n", got.Message)
		fmt.Printf("   Time: %v\n", time.Unix(got.Timestamp, 0))
		if got.Type == sent.Type && got.Message == sent.Message && got.MangaID == sent.MangaID {
			gotOwn = true
		}
	}

	conn.Write([]byte("UNREGISTER"))
	if *listen > 0 && !gotOwn {
		fail("The notification did not come back within %v", *listen)
	}
	if gotOwn {
		fmt.Println("\n✅ The server delivered the notification to this subscriber")
	}
}

func fail(format string, args ...interface{}) {
	fmt.Printf("❌ "+format+"\n", args...)
	os.Exit(1)
}
