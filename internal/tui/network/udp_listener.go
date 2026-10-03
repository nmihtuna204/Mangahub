// Package network - UDP Listener for Bubble Tea
// Non-blocking UDP listener for real-time notifications
// Handles chapter release alerts and system notifications
package network

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// =====================================
// MESSAGE TYPES - Bubble Tea Messages
// =====================================

// UDPNotificationMsg represents an incoming UDP notification
type UDPNotificationMsg struct {
	Type      string    // chapter_release, progress_update, system, announcement
	Title     string    // Notification title
	Content   string    // Notification content
	MangaID   string    // Related manga ID (if any)
	MangaName string    // Related manga name
	Chapter   int       // Chapter number (for chapter releases)
	Timestamp time.Time // When notification was sent
}

// UDPConnectedMsg signals the listener registered with the UDP server
type UDPConnectedMsg struct {
	Port string
}

// UDPErrorMsg signals a UDP error
type UDPErrorMsg struct {
	Err error
}

// UDPDisconnectedMsg signals UDP listener stopped
type UDPDisconnectedMsg struct {
	Reason string
}

// wireNotification is the JSON the UDP server sends (internal/udp.Notification)
type wireNotification struct {
	Type       string `json:"type"`
	MangaID    string `json:"manga_id"`
	Message    string `json:"message"`
	Timestamp  int64  `json:"timestamp"` // unix seconds
	MangaTitle string `json:"manga_title"`
	Chapter    int    `json:"chapter"`
	Title      string `json:"title"`
}

// =====================================
// UDP LISTENER
// =====================================

// UDPListener subscribes to the UDP notification server.
// It uses one connected socket for both REGISTER and receiving, so the
// server sends notifications back to exactly the address we registered from.
type UDPListener struct {
	mu        sync.Mutex
	conn      *net.UDPConn
	register  string // what the heartbeat re-sends
	stopBeat  chan struct{}
	heartbeat time.Duration
}

// NewUDPListener creates a new UDP listener
func NewUDPListener() *UDPListener {
	return &UDPListener{heartbeat: time.Minute} // the server forgets silent subscribers after 5 minutes
}

// =====================================
// BUBBLE TEA COMMANDS
// =====================================

// Start registers with the UDP server at serverAddr - returns tea.Cmd.
// With a token ("REGISTER <jwt>") the server knows which user this is and
// also delivers notifications meant only for them (new chapters of manga in
// their library); without one it receives only general broadcasts.
func (l *UDPListener) Start(serverAddr, token string) tea.Cmd {
	return func() tea.Msg {
		l.close() // drop any previous subscription

		raddr, err := net.ResolveUDPAddr("udp", serverAddr)
		if err != nil {
			return UDPErrorMsg{Err: err}
		}
		conn, err := net.DialUDP("udp", nil, raddr)
		if err != nil {
			return UDPErrorMsg{Err: err}
		}

		register := "REGISTER"
		if token != "" {
			register += " " + token
		}
		if _, err := conn.Write([]byte(register)); err != nil {
			conn.Close()
			return UDPErrorMsg{Err: fmt.Errorf("udp register: %w", err)}
		}
		// Wait for the server's confirmation so a missing server shows up as an error
		buf := make([]byte, 256) // room for an "ERROR ..." reply
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, err := conn.Read(buf)
		if err == nil && strings.HasPrefix(string(buf[:n]), "ERROR") {
			conn.Close()
			return UDPErrorMsg{Err: fmt.Errorf("udp notification server refused registration: %s", buf[:n])}
		}
		if err != nil || string(buf[:n]) != "REGISTERED" {
			conn.Close()
			if err == nil {
				err = fmt.Errorf("unexpected reply %q", buf[:n])
			}
			return UDPErrorMsg{Err: fmt.Errorf("udp notification server %s not reachable: %w", serverAddr, err)}
		}
		_ = conn.SetReadDeadline(time.Time{})

		stop := make(chan struct{})
		l.mu.Lock()
		l.conn = conn
		l.register = register
		l.stopBeat = stop
		l.mu.Unlock()

		// Heartbeat: re-register periodically so the server keeps (or, after
		// a server restart, regains) this subscription. Replies are handled
		// by WaitForPacket.
		go func() {
			ticker := time.NewTicker(l.heartbeat)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					l.mu.Lock()
					msg := l.register
					l.mu.Unlock()
					_, _ = conn.Write([]byte(msg))
				case <-stop:
					return
				}
			}
		}()

		_, port, _ := net.SplitHostPort(conn.LocalAddr().String())
		return UDPConnectedMsg{Port: port}
	}
}

// Stop unregisters and closes the listener. Safe to call more than once.
func (l *UDPListener) Stop() tea.Cmd {
	return func() tea.Msg {
		l.close()
		return UDPDisconnectedMsg{Reason: "user stopped"}
	}
}

func (l *UDPListener) close() {
	l.mu.Lock()
	conn := l.conn
	l.conn = nil
	if l.stopBeat != nil {
		close(l.stopBeat)
		l.stopBeat = nil
	}
	l.mu.Unlock()
	if conn != nil {
		_, _ = conn.Write([]byte("UNREGISTER"))
		_ = conn.Close()
	}
}

// WaitForPacket blocks until the next notification arrives - returns tea.Cmd.
// The app re-issues it after every UDPNotificationMsg (subscription pattern).
func (l *UDPListener) WaitForPacket() tea.Cmd {
	return func() tea.Msg {
		l.mu.Lock()
		conn := l.conn
		l.mu.Unlock()
		if conn == nil {
			return UDPDisconnectedMsg{Reason: "not connected"}
		}

		buffer := make([]byte, 2048)
		for {
			n, err := conn.Read(buffer)
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					return UDPDisconnectedMsg{Reason: "listener closed"}
				}
				// Transient errors (e.g. an ICMP reset reported on Windows): keep listening
				time.Sleep(time.Second)
				continue
			}

			data := buffer[:n]
			if s := string(data); s == "REGISTERED" || s == "UNREGISTERED" {
				continue
			}
			if strings.HasPrefix(string(data), "ERROR") {
				// A heartbeat was refused, i.e. the login token expired: keep
				// the general broadcasts with an anonymous subscription
				l.mu.Lock()
				wasUser := l.register != "REGISTER"
				l.register = "REGISTER"
				l.mu.Unlock()
				if !wasUser {
					continue
				}
				_, _ = conn.Write([]byte("REGISTER"))
				return UDPNotificationMsg{
					Type:      "system",
					Content:   "Login expired: only general notifications until you log in again",
					Timestamp: time.Now(),
				}
			}

			var wire wireNotification
			if err := json.Unmarshal(data, &wire); err != nil {
				return UDPNotificationMsg{Type: "system", Content: string(data), Timestamp: time.Now()}
			}
			msg := UDPNotificationMsg{
				Type:      wire.Type,
				Title:     wire.Title,
				Content:   wire.Message,
				MangaID:   wire.MangaID,
				MangaName: wire.MangaTitle,
				Chapter:   wire.Chapter,
				Timestamp: time.Now(),
			}
			if wire.Timestamp > 0 {
				msg.Timestamp = time.Unix(wire.Timestamp, 0)
			}
			return msg
		}
	}
}

// IsActive returns whether the listener is registered
func (l *UDPListener) IsActive() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.conn != nil
}

// =====================================
// HELPER FUNCTIONS
// =====================================

// FormatNotification formats a notification for display
func FormatNotification(msg UDPNotificationMsg) string {
	switch msg.Type {
	case "chapter_release":
		if msg.MangaName != "" && msg.Chapter > 0 {
			return fmt.Sprintf("📖 %s - Chapter %d released!", msg.MangaName, msg.Chapter)
		}
		if msg.Content != "" {
			return "📖 " + msg.Content
		}
		return "📖 New chapter released!"
	case "progress_update":
		return "📚 " + msg.Content
	case "announcement":
		return "📢 " + msg.Content
	default:
		if msg.Title != "" {
			return "🔔 " + msg.Title + ": " + msg.Content
		}
		return "🔔 " + msg.Content
	}
}
