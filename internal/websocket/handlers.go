package websocket

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"mangahub/internal/auth"
	"mangahub/internal/chat"
	"mangahub/pkg/logger"
	"mangahub/pkg/models"
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

	select {
	case h.hub.register <- client:
	case <-h.hub.stop:
		client.closeConn()
		return
	}

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

// GetRoomMessages handles GET /rooms/:room_id/messages?limit=50&offset=0
// Returns persisted chat history, oldest first.
func (h *Handler) GetRoomMessages(c *gin.Context) {
	limit, offset := 50, 0
	if v, err := strconv.Atoi(c.Query("limit")); err == nil && v > 0 && v <= 200 {
		limit = v
	}
	if v, err := strconv.Atoi(c.Query("offset")); err == nil && v >= 0 {
		offset = v
	}

	history, err := h.hub.GetRoomHistory(c.Request.Context(), c.Param("room_id"), limit, offset)
	if err != nil {
		logger.Errorf("load chat history: %v", err)
		c.JSON(http.StatusInternalServerError,
			models.NewErrorResponse(models.ErrCodeInternal, "failed to load chat history", nil))
		return
	}
	if history.Messages == nil {
		history.Messages = []chat.Message{}
	}
	c.JSON(http.StatusOK, models.NewSuccessResponse(history, "chat history"))
}
