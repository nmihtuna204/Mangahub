// Package network - UDP Listener for Bubble Tea
// Non-blocking UDP listener for real-time notifications
// Handles chapter release alerts and system notifications
package network

import (
	"encoding/json"
	"net"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// =====================================
// MESSAGE TYPES - Bubble Tea Messages
// =====================================

// UDPNotificationMsg represents an incoming UDP notification
type UDPNotificationMsg struct {
	Type      string    `json:"type"`       // chapter_release, system, announcement
	Title     string    `json:"title"`      // Notification title
	Content   string    `json:"content"`    // Notification content
	MangaID   string    `json:"manga_id"`   // Related manga ID (if any)
	MangaName string    `json:"manga_name"` // Related manga name
	Chapter   int       `json:"chapter"`    // Chapter number (for chapter releases)
	Timestamp time.Time `json:"timestamp"`  // When notification was sent
}

// UDPConnectedMsg signals UDP listener started
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

// =====================================
// UDP LISTENER
// =====================================

// UDPListener manages UDP connection for Bubble Tea
type UDPListener struct {
	conn   *net.UDPConn
	port   string
	done   chan struct{}
	active bool
}

// NewUDPListener creates a new UDP listener
func NewUDPListener() *UDPListener {
	return &UDPListener{
		done: make(chan struct{}),
	}
}

// =====================================
// BUBBLE TEA COMMANDS
// =====================================

// Start begins listening for UDP notifications - returns tea.Cmd
func (l *UDPListener) Start(port string) tea.Cmd {
	return func() tea.Msg {
		l.port = port

		addr, err := net.ResolveUDPAddr("udp", ":"+port)
		if err != nil {
			return UDPErrorMsg{Err: err}
		}

		conn, err := net.ListenUDP("udp", addr)
		if err != nil {
			return UDPErrorMsg{Err: err}
		}

		l.conn = conn
		l.active = true

		return UDPConnectedMsg{Port: port}
	}
}

// Stop stops the UDP listener
func (l *UDPListener) Stop() tea.Cmd {
	return func() tea.Msg {
		if l.conn != nil {
			l.conn.Close()
		}
		l.active = false
		close(l.done)
		return UDPDisconnectedMsg{Reason: "user stopped"}
	}
}

// WaitForPacket blocks waiting for a UDP packet - returns tea.Cmd
// This is a Bubble Tea subscription pattern
func (l *UDPListener) WaitForPacket() tea.Cmd {
	return func() tea.Msg {
		if l.conn == nil || !l.active {
			return UDPDisconnectedMsg{Reason: "not connected"}
		}

		buffer := make([]byte, 2048)

		// Set read timeout to allow graceful shutdown
		l.conn.SetReadDeadline(time.Now().Add(5 * time.Second))

		n, _, err := l.conn.ReadFromUDP(buffer)
		if err != nil {
			// Check if it's a timeout (expected, just retry)
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// Not an error, just no data yet - resubscribe
				return nil
			}
			return UDPErrorMsg{Err: err}
		}

		// Parse the notification
		var msg UDPNotificationMsg
		if err := json.Unmarshal(buffer[:n], &msg); err != nil {
			// Try parsing as simple text
			msg = UDPNotificationMsg{
				Type:      "system",
				Content:   string(buffer[:n]),
				Timestamp: time.Now(),
			}
		}

		if msg.Timestamp.IsZero() {
			msg.Timestamp = time.Now()
		}

		return msg
	}
}

// IsActive returns whether the listener is active
func (l *UDPListener) IsActive() bool {
	return l.active
}

// =====================================
// HELPER FUNCTIONS
// =====================================

// RegisterWithServer sends a REGISTER message to the UDP notification server
func (l *UDPListener) RegisterWithServer(serverAddr, userID string) tea.Cmd {
	return func() tea.Msg {
		addr, err := net.ResolveUDPAddr("udp", serverAddr)
		if err != nil {
			return UDPErrorMsg{Err: err}
		}

		conn, err := net.DialUDP("udp", nil, addr)
		if err != nil {
			return UDPErrorMsg{Err: err}
		}
		defer conn.Close()

		// Send REGISTER message
		registerMsg := map[string]string{
			"type":    "REGISTER",
			"user_id": userID,
		}
		data, _ := json.Marshal(registerMsg)
		_, err = conn.Write(data)
		if err != nil {
			return UDPErrorMsg{Err: err}
		}

		return nil
	}
}

// FormatNotification formats a notification for display
func FormatNotification(msg UDPNotificationMsg) string {
	switch msg.Type {
	case "chapter_release":
		if msg.MangaName != "" {
			return "📖 " + msg.MangaName + " - Chapter " + string(rune('0'+msg.Chapter)) + " released!"
		}
		return "📖 New chapter released!"
	case "announcement":
		return "📢 " + msg.Content
	default:
		if msg.Title != "" {
			return "🔔 " + msg.Title + ": " + msg.Content
		}
		return "🔔 " + msg.Content
	}
}
