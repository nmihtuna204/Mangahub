// Package network - WebSocket Client Manager for Bubble Tea
// Non-blocking WebSocket integration using tea.Cmd pattern
// Handles real-time chat communication with the backend Hub
package network

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
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
	Type      string    `json:"type"` // message, join, leave, system
	Timestamp time.Time `json:"-"`
}

// UnmarshalJSON accepts the server's unix-seconds timestamp as well as RFC 3339 strings.
func (m *ChatMessageMsg) UnmarshalJSON(data []byte) error {
	type plain ChatMessageMsg
	aux := struct {
		*plain
		Timestamp json.RawMessage `json:"timestamp"`
	}{plain: (*plain)(m)}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	m.Timestamp = time.Now()
	var secs int64
	var str string
	switch {
	case len(aux.Timestamp) == 0:
	case json.Unmarshal(aux.Timestamp, &secs) == nil && secs > 0:
		m.Timestamp = time.Unix(secs, 0)
	case json.Unmarshal(aux.Timestamp, &str) == nil:
		if t, err := time.Parse(time.RFC3339, str); err == nil {
			m.Timestamp = t
		}
	}
	return nil
}

// WSConnectedMsg signals successful WebSocket connection
type WSConnectedMsg struct {
	RoomID string
}

// WSDisconnectedMsg signals an unexpected WebSocket disconnection
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

// session is one live WebSocket connection with its own channels, so loops
// and listeners from an old connection can never mix with a new one.
type session struct {
	conn     *websocket.Conn
	roomID   string
	send     chan []byte
	receive  chan []byte
	done     chan struct{} // closed when we close the session on purpose
	lost     chan struct{} // closed when the connection drops by itself
	stopOnce sync.Once
}

func (s *session) stop() {
	s.stopOnce.Do(func() {
		close(s.done)
		_ = s.conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(time.Second))
		_ = s.conn.Close()
	})
}

// ErrReconnectGaveUp is returned (inside WSErrorMsg) once reconnection attempts are exhausted
var ErrReconnectGaveUp = errors.New("max reconnection attempts reached")

// WSClient manages WebSocket connection for Bubble Tea
type WSClient struct {
	mu      sync.Mutex
	current *session
	url     string
	token   string
	roomID  string

	// Reconnection
	reconnectAttempt int
	maxReconnect     int
	baseBackoff      time.Duration
	maxBackoff       time.Duration
}

// NewWSClient creates a new WebSocket client
func NewWSClient() *WSClient {
	return &WSClient{
		maxReconnect: 5,
		baseBackoff:  2 * time.Second,
		maxBackoff:   30 * time.Second,
	}
}

// IsConnected returns connection status (thread-safe)
func (c *WSClient) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.current != nil
}

// CurrentRoom returns the current room ID
func (c *WSClient) CurrentRoom() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.roomID
}

// IsConnectedTo reports whether there is a live connection to roomID
func (c *WSClient) IsConnectedTo(roomID string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.current != nil && c.current.roomID == roomID
}

// =====================================
// BUBBLE TEA COMMANDS
// =====================================

// Connect establishes a WebSocket connection to roomID, replacing any existing one - returns tea.Cmd
func (c *WSClient) Connect(baseURL, token, roomID string) tea.Cmd {
	return func() tea.Msg {
		c.mu.Lock()
		c.url, c.token, c.roomID = baseURL, token, roomID
		c.reconnectAttempt = 0
		old := c.current
		c.current = nil
		c.mu.Unlock()
		if old != nil {
			old.stop()
		}

		if err := c.dial(baseURL, token, roomID); err != nil {
			return WSErrorMsg{Err: fmt.Errorf("failed to connect: %w", err)}
		}
		return WSConnectedMsg{RoomID: roomID}
	}
}

// Disconnect closes the WebSocket connection on purpose (e.g. logout) - returns tea.Cmd
func (c *WSClient) Disconnect() tea.Cmd {
	return func() tea.Msg {
		c.mu.Lock()
		old := c.current
		c.current = nil
		c.roomID = ""
		c.mu.Unlock()
		if old != nil {
			old.stop()
		}
		return nil
	}
}

// ListenForMessages is a Bubble Tea subscription that waits for the next
// message on the current connection. It returns nil (no message) if that
// connection is closed on purpose, and WSDisconnectedMsg if it drops.
func (c *WSClient) ListenForMessages() tea.Cmd {
	c.mu.Lock()
	s := c.current
	c.mu.Unlock()
	if s == nil {
		return nil
	}

	return func() tea.Msg {
		select {
		case data := <-s.receive:
			var msg ChatMessageMsg
			if err := json.Unmarshal(data, &msg); err != nil {
				msg = ChatMessageMsg{Content: string(data), Type: "message", Timestamp: time.Now()}
			}
			if msg.RoomID == "" {
				msg.RoomID = s.roomID
			}
			return msg
		case <-s.lost:
			c.mu.Lock()
			if c.current == s {
				c.current = nil
			}
			c.mu.Unlock()
			return WSDisconnectedMsg{Reason: "connection lost"}
		case <-s.done:
			return nil
		}
	}
}

// SendMessage sends a chat message through the WebSocket
func (c *WSClient) SendMessage(roomID, content string) tea.Cmd {
	return func() tea.Msg {
		c.mu.Lock()
		s := c.current
		c.mu.Unlock()

		if s == nil {
			return WSErrorMsg{Err: fmt.Errorf("not connected")}
		}

		data, err := json.Marshal(map[string]interface{}{
			"room_id": roomID,
			"content": content,
			"type":    "message",
		})
		if err != nil {
			return WSErrorMsg{Err: err}
		}

		select {
		case s.send <- data:
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
		if c.current != nil || c.roomID == "" {
			// Already reconnected, or the user left chat / logged out
			c.mu.Unlock()
			return nil
		}
		c.reconnectAttempt++
		attempt := c.reconnectAttempt
		if attempt > c.maxReconnect {
			c.mu.Unlock()
			return WSErrorMsg{Err: ErrReconnectGaveUp}
		}

		backoff := c.baseBackoff * time.Duration(1<<uint(attempt-1))
		if backoff > c.maxBackoff {
			backoff = c.maxBackoff
		}
		baseURL, token, roomID := c.url, c.token, c.roomID
		c.mu.Unlock()

		time.Sleep(backoff)

		if err := c.dial(baseURL, token, roomID); err != nil {
			return WSReconnectingMsg{Attempt: attempt, MaxWait: backoff * 2}
		}
		c.mu.Lock()
		c.reconnectAttempt = 0
		c.mu.Unlock()
		return WSConnectedMsg{RoomID: roomID}
	}
}

// dial opens a connection and installs it as the current session
func (c *WSClient) dial(baseURL, token, roomID string) error {
	wsURL := fmt.Sprintf("%s/ws/chat?room_id=%s", baseURL, url.QueryEscape(roomID))
	header := http.Header{}
	if token != "" {
		header.Set("Authorization", "Bearer "+token)
	}

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		return err
	}

	s := &session{
		conn:    conn,
		roomID:  roomID,
		send:    make(chan []byte, 256),
		receive: make(chan []byte, 256),
		done:    make(chan struct{}),
		lost:    make(chan struct{}),
	}

	c.mu.Lock()
	old := c.current
	c.current = s
	c.mu.Unlock()
	if old != nil {
		old.stop()
	}

	go c.readLoop(s)
	go c.writeLoop(s)
	return nil
}

// =====================================
// INTERNAL GOROUTINES
// =====================================

// readLoop runs in a goroutine, reading messages from WebSocket
func (c *WSClient) readLoop(s *session) {
	for {
		_, message, err := s.conn.ReadMessage()
		if err != nil {
			select {
			case <-s.done: // closed on purpose
			default:
				close(s.lost)
			}
			return
		}

		select {
		case s.receive <- message:
		case <-s.done:
			return
		}
	}
}

// writeLoop runs in a goroutine, writing messages to WebSocket
func (c *WSClient) writeLoop(s *session) {
	ticker := time.NewTicker(54 * time.Second) // Ping interval
	defer ticker.Stop()

	for {
		select {
		case message := <-s.send:
			_ = s.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := s.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				_ = s.conn.Close() // readLoop notices and reports the loss
				return
			}

		case <-ticker.C:
			_ = s.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := s.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				_ = s.conn.Close()
				return
			}

		case <-s.done:
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
