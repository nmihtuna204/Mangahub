package websocket

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/yourusername/mangahub/internal/auth"
	"github.com/yourusername/mangahub/pkg/logger"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for development
	},
}

type Handler struct {
	hub *Hub
}

func NewHandler(hub *Hub) *Handler {
	return &Handler{hub: hub}
}

func (h *Handler) ServeWS(c *gin.Context) {
	user := auth.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	roomID := c.Query("room_id")
	if roomID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "room_id required"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Errorf("Failed to upgrade connection: %v", err)
		return
	}

	client := &Client{
		hub:      h.hub,
		conn:     conn,
		send:     make(chan RoomMessage, 256),
		userID:   user.ID,
		username: user.Username,
		roomID:   roomID,
	}

	h.hub.register <- client

	go client.writePump()
	go client.readPump()
}

func (h *Handler) GetRoomInfo(c *gin.Context) {
	roomID := c.Param("room_id")
	if roomID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "room_id required"})
		return
	}

	clients := h.hub.GetRoomClients(roomID)
	c.JSON(http.StatusOK, gin.H{
		"room_id": roomID,
		"clients": clients,
		"count":   len(clients),
	})
}
