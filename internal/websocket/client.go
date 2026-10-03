package websocket

import (
	"strings"
	"sync"
	"time"

	"mangahub/pkg/logger"

	"github.com/gorilla/websocket"
)

const (
	writeWait       = 10 * time.Second
	pongWait        = 60 * time.Second
	pingPeriod      = (pongWait * 9) / 10
	maxMessageSize  = 8192 // whole JSON frame; content itself is capped at maxContentRunes
	maxContentRunes = 2000
)

type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan RoomMessage
	userID   string
	username string
	roomID   string

	closeOnce sync.Once
}

func (c *Client) readPump() {
	defer func() {
		select {
		case c.hub.unregister <- c:
		case <-c.hub.stop: // the hub is gone
		}
		c.closeConn()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		var msg struct {
			Content string `json:"content"`
			Type    string `json:"type"`
		}
		if err := c.conn.ReadJSON(&msg); err != nil {
			// Clients closing normally (1000), leaving (1001), closing without a
			// code (1005) or just dropping (1006) aren't worth a warning
			if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway,
				websocket.CloseNoStatusReceived, websocket.CloseAbnormalClosure) {
				logger.Warnf("WebSocket connection of %s closed unexpectedly: %v", c.username, err)
			}
			break
		}

		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		if runes := []rune(content); len(runes) > maxContentRunes {
			content = string(runes[:maxContentRunes])
		}
		// Clients may only send chat messages; join/leave/system notices come
		// from the server, so any client-supplied type is ignored.
		roomMsg := NewRoomMessage(c.userID, c.username, content, "message")
		roomMsg.RoomID = c.roomID
		select {
		case c.hub.broadcast <- roomMsg:
		case <-c.hub.stop:
			return
		}
	}
}

// closeConn closes the connection exactly once, from whichever goroutine
// gets there first (readPump, writePump or Hub.Stop). While the hub is
// shutting down it first sends a "going away" close frame, so clients can tell
// a server restart from a network failure. Every path that closes the
// connection must come through here: a plain Close from one goroutine used to
// beat Stop's close frame. WriteControl may run concurrently with writePump.
func (c *Client) closeConn() {
	if c.conn == nil {
		return
	}
	c.closeOnce.Do(func() {
		select {
		case <-c.hub.stop:
			_ = c.conn.WriteControl(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseGoingAway, "server shutting down"),
				time.Now().Add(time.Second))
		default:
		}
		_ = c.conn.Close()
	})
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.closeConn()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteJSON(msg); err != nil {
				// Usually the client has just gone away; not a server error
				logger.Debugf("websocket write to %s failed: %v", c.username, err)
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		case <-c.hub.stop:
			return
		}
	}
}
