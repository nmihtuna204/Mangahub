// Package websocket - WebSocket Chat Hub Implementation
// Quản lý WebSocket connections và chat rooms
// Chức năng:
//   - Quản lý nhiều chat rooms (theo manga_id)
//   - Client registration/unregistration cho mỗi room
//   - Real-time message broadcasting trong room
//   - Join/leave notifications
//   - Bidirectional communication
//   - Concurrent-safe với mutex
package websocket

import (
	"sync"

	"mangahub/pkg/logger"
)

type Hub struct {
	rooms      map[string]map[*Client]bool
	mu         sync.RWMutex
	register   chan *Client
	unregister chan *Client
	broadcast  chan RoomMessage
	stop       chan struct{}
}

func NewHub() *Hub {
	return &Hub{
		rooms:      make(map[string]map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan RoomMessage, 256),
		stop:       make(chan struct{}),
	}
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

	logger.Infof("Client %s joined room %s", c.username, c.roomID)

	joinNotice := NewRoomMessage(c.userID, c.username, c.username+" joined the chat", "join")
	h.broadcastToRoom(c.roomID, joinNotice)
}

func (h *Hub) unregisterClient(c *Client) {
	h.mu.Lock()
	if room, exists := h.rooms[c.roomID]; exists {
		if _, ok := room[c]; ok {
			delete(room, c)
			close(c.send)

			logger.Infof("Client %s left room %s", c.username, c.roomID)

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
	h.mu.RLock()
	defer h.mu.RUnlock()

	if room, exists := h.rooms[msg.RoomID]; exists {
		logger.Debugf("Broadcasting message in room %s from %s", msg.RoomID, msg.Username)
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

func (h *Hub) Stop() {
	close(h.stop)
}
