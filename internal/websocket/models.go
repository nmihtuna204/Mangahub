package websocket

import "time"

type ChatMessage struct {
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	RoomID    string `json:"room_id"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
}

type RoomMessage struct {
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
	Type      string `json:"type"` // message, join, leave
	RoomID    string `json:"room_id,omitempty"`
}

func NewRoomMessage(userID, username, message, msgType string) RoomMessage {
	return RoomMessage{
		UserID:    userID,
		Username:  username,
		Message:   message,
		Timestamp: time.Now().Unix(),
		Type:      msgType,
	}
}
