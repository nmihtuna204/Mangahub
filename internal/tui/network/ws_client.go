// Package network - WebSocket Client Manager for Bubble Tea
// Non-blocking WebSocket integration using tea.Cmd pattern
// Handles real-time chat communication with the backend Hub
package network

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gorilla/websocket"
)

// =====================================
// MESSAGE TYPES - Bubble Tea Messages
// =====================================

// ChatMessageMsg represents an incoming chat message
type ChatMessageMsg struct {
	ID        string    `json:"id"`
	RoomID    string    `json:"room_id"`
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	Content   string    `json:"content"`
	Type      string    `json:"type"` // text, join, leave, system
	Timestamp time.Time `json:"timestamp"`
}

// WSConnectedMsg signals successful WebSocket connection
type WSConnectedMsg struct {
	RoomID string
}

// WSDisconnectedMsg signals WebSocket disconnection
type WSDisconnectedMsg struct {
	Reason string
}

// WSErrorMsg signals a WebSocket error
type WSErrorMsg struct {
	Err error
}

// WSReconnectingMsg signals reconnection attempt
type WSReconnectingMsg struct {
	Attempt int
	MaxWait time.Duration
}

// SendMessageCmd is returned when user wants to send a message
type SendMessageCmd struct {
	RoomID  string
	Content string
}

// JoinRoomMsg triggers room join from other views
type JoinRoomMsg struct {
	RoomID    string
	RoomName  string
	MangaID   string
	MangaName string
}

// =====================================
// WEBSOCKET CLIENT
// =====================================

// WSClient manages WebSocket connection for Bubble Tea
type WSClient struct {
	conn     *websocket.Conn
	send     chan []byte
	receive  chan []byte
	done     chan struct{}
	mu       sync.RWMutex
	url      string
	token    string
	roomID   string
	connected bool
	
	// Reconnection
	reconnectAttempt int
	maxReconnect     int
	baseBackoff      time.Duration
	maxBackoff       time.Duration
}

// NewWSClient creates a new WebSocket client
func NewWSClient() *WSClient {
	return &WSClient{
		send:         make(chan []byte, 256),
		receive:      make(chan []byte, 256),
		done:         make(chan struct{}),
		maxReconnect: 5,
		baseBackoff:  2 * time.Second,
		maxBackoff:   30 * time.Second,
	}
}

// IsConnected returns connection status (thread-safe)
func (c *WSClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// CurrentRoom returns the current room ID
func (c *WSClient) CurrentRoom() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.roomID
}

// =====================================
// BUBBLE TEA COMMANDS
// =====================================

// Connect establishes WebSocket connection - returns tea.Cmd
func (c *WSClient) Connect(baseURL, token, roomID string) tea.Cmd {
	return func() tea.Msg {
		c.mu.Lock()
		c.url = baseURL
		c.token = token
		c.roomID = roomID
		c.mu.Unlock()

		// Build WebSocket URL with auth
		wsURL := fmt.Sprintf("%s/ws/chat?room_id=%s", baseURL, roomID)
		
		// Set up headers with JWT token
		header := http.Header{}
		if token != "" {
			header.Set("Authorization", "Bearer "+token)
		}

		// Dial WebSocket
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
		if err != nil {
			return WSErrorMsg{Err: fmt.Errorf("failed to connect: %w", err)}
		}

		c.mu.Lock()
		c.conn = conn
		c.connected = true
		c.reconnectAttempt = 0
		c.mu.Unlock()

		// Start read/write loops
		go c.readLoop()
		go c.writeLoop()

		return WSConnectedMsg{RoomID: roomID}
	}
}

// Disconnect closes the WebSocket connection
func (c *WSClient) Disconnect() tea.Cmd {
	return func() tea.Msg {
		c.mu.Lock()
		defer c.mu.Unlock()

		if c.conn != nil {
			// Send close message
			c.conn.WriteMessage(websocket.CloseMessage, 
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			c.conn.Close()
			c.conn = nil
		}
		c.connected = false
		close(c.done)

		return WSDisconnectedMsg{Reason: "user disconnect"}
	}
}

// ListenForMessages is a Bubble Tea subscription that listens for incoming messages
// It blocks waiting for a message, then returns it and re-subscribes
func (c *WSClient) ListenForMessages() tea.Cmd {
	return func() tea.Msg {
		select {
		case data, ok := <-c.receive:
			if !ok {
				// Channel closed, connection lost
				return WSDisconnectedMsg{Reason: "connection closed"}
			}

			// Parse the message
			var msg ChatMessageMsg
			if err := json.Unmarshal(data, &msg); err != nil {
				// Try to handle as raw text
				msg = ChatMessageMsg{
					Content:   string(data),
					Type:      "text",
					Timestamp: time.Now(),
				}
			}
			return msg

		case <-c.done:
			return WSDisconnectedMsg{Reason: "client shutdown"}
		}
	}
}

// SendMessage sends a chat message through the WebSocket
func (c *WSClient) SendMessage(roomID, content string) tea.Cmd {
	return func() tea.Msg {
		c.mu.RLock()
		connected := c.connected
		c.mu.RUnlock()

		if !connected {
			return WSErrorMsg{Err: fmt.Errorf("not connected")}
		}

		msg := map[string]interface{}{
			"room_id": roomID,
			"content": content,
			"type":    "text",
		}

		data, err := json.Marshal(msg)
		if err != nil {
			return WSErrorMsg{Err: err}
		}

		select {
		case c.send <- data:
			// Message queued successfully
			return nil
		default:
			return WSErrorMsg{Err: fmt.Errorf("send buffer full")}
		}
	}
}

// Reconnect attempts to reconnect with exponential backoff
func (c *WSClient) Reconnect() tea.Cmd {
	return func() tea.Msg {
		c.mu.Lock()
		c.reconnectAttempt++
		attempt := c.reconnectAttempt
		
		if attempt > c.maxReconnect {
			c.mu.Unlock()
			return WSErrorMsg{Err: fmt.Errorf("max reconnection attempts reached")}
		}

		// Calculate backoff with exponential increase
		backoff := c.baseBackoff * time.Duration(1<<uint(attempt-1))
		if backoff > c.maxBackoff {
			backoff = c.maxBackoff
		}

		url := c.url
		token := c.token
		roomID := c.roomID
		c.mu.Unlock()

		// Wait before reconnecting
		time.Sleep(backoff)

		// Attempt reconnection
		wsURL := fmt.Sprintf("%s/ws/chat?room_id=%s", url, roomID)
		header := http.Header{}
		if token != "" {
			header.Set("Authorization", "Bearer "+token)
		}

		conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
		if err != nil {
			return WSReconnectingMsg{Attempt: attempt, MaxWait: backoff * 2}
		}

		c.mu.Lock()
		c.conn = conn
		c.connected = true
		c.reconnectAttempt = 0
		c.done = make(chan struct{}) // Reset done channel
		c.mu.Unlock()

		go c.readLoop()
		go c.writeLoop()

		return WSConnectedMsg{RoomID: roomID}
	}
}

// =====================================
// INTERNAL GOROUTINES
// =====================================

// readLoop runs in a goroutine, reading messages from WebSocket
func (c *WSClient) readLoop() {
	defer func() {
		c.mu.Lock()
		c.connected = false
		c.mu.Unlock()
	}()

	for {
		c.mu.RLock()
		conn := c.conn
		c.mu.RUnlock()

		if conn == nil {
			return
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			// Connection error - signal disconnect
			select {
			case c.receive <- []byte(`{"type":"error","content":"connection lost"}`):
			default:
			}
			return
		}

		select {
		case c.receive <- message:
		case <-c.done:
			return
		default:
			// Buffer full, drop message (log in production)
		}
	}
}

// writeLoop runs in a goroutine, writing messages to WebSocket
func (c *WSClient) writeLoop() {
	ticker := time.NewTicker(54 * time.Second) // Ping interval
	defer ticker.Stop()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				return
			}

			c.mu.RLock()
			conn := c.conn
			c.mu.RUnlock()

			if conn == nil {
				return
			}

			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			c.mu.RLock()
			conn := c.conn
			c.mu.RUnlock()

			if conn == nil {
				return
			}

			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		case <-c.done:
			return
		}
	}
}

// =====================================
// HELPER FUNCTIONS
// =====================================

// FormatTimestamp formats a timestamp for display
func FormatTimestamp(t time.Time) string {
	now := time.Now()
	if t.Day() == now.Day() && t.Month() == now.Month() && t.Year() == now.Year() {
		return t.Format("15:04")
	}
	return t.Format("Jan 2 15:04")
}
