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
	"time"

	"github.com/google/uuid"

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
	stopOnce   sync.Once
	stopped    bool // set under mu by Stop; late registrations are refused

	// Chat repository for message persistence (Phase 2)
	// Optional: if nil, messages are not persisted
	chatRepo chat.Repository
	// User messages to save, queued by the hub loop in broadcast order and
	// written by persistWorker, so history matches what clients saw live
	// without database latency stalling the hub
	persistQueue chan RoomMessage
	persistDone  chan struct{} // closed when persistWorker has drained the queue on Stop
	// Rooms already known to exist in chat_rooms
	knownRooms map[string]bool
}

// persistQueueLen bounds how far message persistence may lag behind live chat
const persistQueueLen = 1024

// persistDrainTimeout bounds how long Stop waits for queued messages to be saved
const persistDrainTimeout = 5 * time.Second

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
// Call this after creating the hub (and before Run) to enable persistence
func (h *Hub) SetChatRepository(repo chat.Repository) {
	h.chatRepo = repo
	h.persistQueue = make(chan RoomMessage, persistQueueLen)
	h.persistDone = make(chan struct{})
	h.knownRooms = make(map[string]bool)
	go h.persistWorker()
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
	if h.stopped {
		h.mu.Unlock()
		c.closeConn()
		return
	}
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
	// Only user chat messages are saved; join/leave/system notices are not
	if h.persistQueue != nil && msg.Type == "message" {
		select {
		case h.persistQueue <- msg:
		default:
			logger.Warnf("Chat persistence queue full, message in %s not saved", msg.RoomID)
		}
	}

	h.mu.RLock()
	_, exists := h.rooms[msg.RoomID]
	h.mu.RUnlock()
	if exists {
		// Protocol trace logging
		logger.WebSocket("BROADCAST", msg.RoomID, msg.UserID, "type="+msg.Type+" from="+msg.Username)
	}
	h.broadcastToRoom(msg.RoomID, msg)
}

func (h *Hub) broadcastToRoom(roomID string, msg RoomMessage) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if room, exists := h.rooms[roomID]; exists {
		for client := range room {
			select {
			case client.send <- msg:
			default:
				// Don't close client.send here: unregisterClient closes it,
				// and closing twice panics.
				logger.Warnf("Client %s send buffer full, disconnecting", client.username)
				go func(c *Client) {
					select {
					case h.unregister <- c:
					case <-h.stop:
					}
				}(client)
			}
		}
	}
}

// NotifyRoom queues a server-generated message (e.g. a protocol bridge event)
// for every client in the room. It never blocks the caller.
func (h *Hub) NotifyRoom(roomID, userID, username, content, msgType string) {
	msg := NewRoomMessage(userID, username, content, msgType)
	msg.RoomID = roomID
	select {
	case h.broadcast <- msg:
	default:
		logger.Warnf("WebSocket broadcast queue full, dropping notice for room %s", roomID)
	}
}

// persistWorker saves queued user messages one at a time, in broadcast order.
// On Stop it saves whatever is still queued before returning.
func (h *Hub) persistWorker() {
	defer close(h.persistDone)
	for {
		select {
		case msg := <-h.persistQueue:
			h.persist(msg)
		case <-h.stop:
			for {
				select {
				case msg := <-h.persistQueue:
					h.persist(msg)
				default:
					return
				}
			}
		}
	}
}

func (h *Hub) persist(msg RoomMessage) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if !h.knownRooms[msg.RoomID] {
		if err := h.chatRepo.EnsureRoom(ctx, msg.RoomID, msg.UserID); err != nil {
			logger.Errorf("Failed to create chat room %s: %v", msg.RoomID, err)
			return
		}
		h.knownRooms[msg.RoomID] = true
	}

	chatMsg := &chat.Message{
		ID:      uuid.New().String(),
		RoomID:  msg.RoomID,
		UserID:  msg.UserID,
		Content: msg.Message,
	}
	if err := h.chatRepo.SaveMessage(ctx, chatMsg); err != nil {
		logger.Errorf("Failed to persist chat message: %v", err)
	}
}

func (h *Hub) GetRoomClients(roomID string) []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	clients := []string{}
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

// Stop shuts the hub down for server shutdown: every connected client gets
// a "going away" close frame, chat messages still queued are saved (waits up
// to persistDrainTimeout, so call it before closing the database), and
// goroutines blocked on the hub are released. Safe to call more than once.
func (h *Hub) Stop() {
	h.stopOnce.Do(func() {
		close(h.stop)

		h.mu.Lock()
		h.stopped = true
		rooms := h.rooms
		h.rooms = make(map[string]map[*Client]bool)
		h.mu.Unlock()

		closed := 0
		for _, room := range rooms {
			for c := range room {
				c.closeConn()
				closed++
			}
		}
		if closed > 0 {
			logger.Infof("WebSocket hub closed %d client connection(s)", closed)
		}

		if h.persistDone != nil {
			select {
			case <-h.persistDone:
			case <-time.After(persistDrainTimeout):
				logger.Warnf("WebSocket hub: chat messages still unsaved after %v", persistDrainTimeout)
			}
		}
	})
}
