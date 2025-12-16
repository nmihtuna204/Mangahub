// Package websocket - WebSocket Chat Hub Implementation
// Quản lý WebSocket connections và chat rooms
// Chức năng:
//   - Quản lý nhiều chat rooms (theo manga_id)
//   - Client registration/unregistration cho mỗi room
//   - Real-time message broadcasting trong room
//   - Join/leave notifications
//   - Bidirectional communication
//   - Concurrent-safe với mutex
//   - Message persistence to database (Phase 2)
package websocket

import (
	"context"
	"sync"

	"mangahub/internal/chat"
	"mangahub/pkg/logger"
)

// Hub manages WebSocket connections and message routing
// Integrates with chat.Repository for message persistence
type Hub struct {
	rooms      map[string]map[*Client]bool
	mu         sync.RWMutex
	register   chan *Client
	unregister chan *Client
	broadcast  chan RoomMessage
	stop       chan struct{}

	// Chat repository for message persistence (Phase 2)
	// Optional: if nil, messages are not persisted
	chatRepo chat.Repository
}

// NewHub creates a new hub without persistence
// Use SetChatRepository to enable message persistence
func NewHub() *Hub {
	return &Hub{
		rooms:      make(map[string]map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan RoomMessage, 256),
		stop:       make(chan struct{}),
	}
}

// SetChatRepository sets the chat repository for message persistence
// Call this after creating the hub to enable persistence
func (h *Hub) SetChatRepository(repo chat.Repository) {
	h.chatRepo = repo
	logger.Info("Chat message persistence enabled")
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.registerClient(client)
		case client := <-h.unregister:
			h.unregisterClient(client)
		case msg := <-h.broadcast:
			h.broadcastMessage(msg)
		case <-h.stop:
			logger.Info("WebSocket hub stopping...")
			return
		}
	}
}

func (h *Hub) registerClient(c *Client) {
	h.mu.Lock()
	if _, exists := h.rooms[c.roomID]; !exists {
		h.rooms[c.roomID] = make(map[*Client]bool)
	}
	h.rooms[c.roomID][c] = true
	h.mu.Unlock()

	// Protocol trace logging
	logger.WebSocket("JOIN", c.roomID, c.userID, c.username+" connected")

	joinNotice := NewRoomMessage(c.userID, c.username, c.username+" joined the chat", "join")
	h.broadcastToRoom(c.roomID, joinNotice)
}

func (h *Hub) unregisterClient(c *Client) {
	h.mu.Lock()
	if room, exists := h.rooms[c.roomID]; exists {
		if _, ok := room[c]; ok {
			delete(room, c)
			close(c.send)

			// Protocol trace logging
			logger.WebSocket("LEAVE", c.roomID, c.userID, c.username+" disconnected")

			leaveNotice := NewRoomMessage(c.userID, c.username, c.username+" left the chat", "leave")
			h.mu.Unlock()
			h.broadcastToRoom(c.roomID, leaveNotice)
			h.mu.Lock()

			if len(room) == 0 {
				delete(h.rooms, c.roomID)
				logger.Infof("Room %s is now empty", c.roomID)
			}
		}
	}
	h.mu.Unlock()
}

func (h *Hub) broadcastMessage(msg RoomMessage) {
	// Persist message to database if repository is configured
	// Chỉ lưu message type "message", không lưu join/leave notifications
	if h.chatRepo != nil && msg.Type == "message" {
		chatMsg := &chat.Message{
			RoomID:      msg.RoomID,
			UserID:      msg.UserID,
			Content:     msg.Message,
			MessageType: msg.Type,
		}
		if err := h.chatRepo.SaveMessage(context.Background(), chatMsg); err != nil {
			logger.Errorf("Failed to persist chat message: %v", err)
			// Continue broadcasting even if persistence fails
		}
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	if room, exists := h.rooms[msg.RoomID]; exists {
		// Protocol trace logging
		logger.WebSocket("BROADCAST", msg.RoomID, msg.UserID, "type="+msg.Type+" from="+msg.Username)
		for client := range room {
			select {
			case client.send <- msg:
			default:
				logger.Warnf("Client %s send buffer full, closing connection", client.username)
				close(client.send)
				go func(c *Client) {
					h.unregister <- c
				}(client)
			}
		}
	}
}

func (h *Hub) broadcastToRoom(roomID string, msg RoomMessage) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if room, exists := h.rooms[roomID]; exists {
		for client := range room {
			select {
			case client.send <- msg:
			default:
				logger.Warnf("Client %s send buffer full, closing connection", client.username)
				close(client.send)
				go func(c *Client) {
					h.unregister <- c
				}(client)
			}
		}
	}
}

func (h *Hub) GetRoomClients(roomID string) []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var clients []string
	if room, exists := h.rooms[roomID]; exists {
		for client := range room {
			clients = append(clients, client.username)
		}
	}
	return clients
}

// GetRoomHistory retrieves message history for a room
// Được gọi khi user join room để load tin nhắn cũ
func (h *Hub) GetRoomHistory(ctx context.Context, roomID string, limit, offset int) (*chat.MessageListResponse, error) {
	if h.chatRepo == nil {
		return &chat.MessageListResponse{
			Messages: []chat.Message{},
			Total:    0,
			Limit:    limit,
			Offset:   offset,
			HasMore:  false,
		}, nil
	}

	messages, total, err := h.chatRepo.GetMessagesByRoom(ctx, roomID, limit, offset)
	if err != nil {
		return nil, err
	}

	return &chat.MessageListResponse{
		Messages: messages,
		Total:    total,
		Limit:    limit,
		Offset:   offset,
		HasMore:  offset+len(messages) < total,
	}, nil
}

func (h *Hub) Stop() {
	close(h.stop)
}
