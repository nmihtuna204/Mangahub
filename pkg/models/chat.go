// Package models - Chat and Real-time Messaging Models
// Hỗ trợ WebSocket chat và real-time features
// Chức năng:
//   - Chat messages với reply support
//   - Chat rooms (public, private, manga-specific)
//   - Typing indicators
//   - Online presence tracking
package models

import (
	"time"
)

// ChatMessage represents a single chat message
type ChatMessage struct {
	ID          string       `json:"id" db:"id"`
	RoomID      string       `json:"room_id" db:"room_id"`
	UserID      string       `json:"user_id" db:"user_id"`
	Username    string       `json:"username" db:"-"` // Joined from users table
	Content     string       `json:"content" db:"content"`
	MessageType string       `json:"message_type" db:"message_type"` // text, image, system, manga_share
	ReplyToID   string       `json:"reply_to_id,omitempty" db:"reply_to_id"`
	ReplyTo     *ChatMessage `json:"reply_to,omitempty" db:"-"` // Nested reply
	Edited      bool         `json:"edited" db:"edited"`
	Deleted     bool         `json:"deleted" db:"deleted"`
	CreatedAt   time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at" db:"updated_at"`
}

// ChatRoom represents a chat room/channel
type ChatRoom struct {
	ID          string    `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	RoomType    string    `json:"room_type" db:"room_type"` // public, private, manga, direct
	MangaID     string    `json:"manga_id,omitempty" db:"manga_id"`
	OwnerID     string    `json:"owner_id" db:"owner_id"`
	Description string    `json:"description,omitempty" db:"description"`
	MaxMembers  int       `json:"max_members" db:"max_members"`
	MemberCount int       `json:"member_count" db:"-"` // Computed
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// ChatRoomMember represents membership in a chat room
type ChatRoomMember struct {
	ID                   string    `json:"id" db:"id"`
	RoomID               string    `json:"room_id" db:"room_id"`
	UserID               string    `json:"user_id" db:"user_id"`
	Role                 string    `json:"role" db:"role"` // owner, admin, moderator, member
	JoinedAt             time.Time `json:"joined_at" db:"joined_at"`
	LastReadAt           time.Time `json:"last_read_at" db:"last_read_at"`
	NotificationsEnabled bool      `json:"notifications_enabled" db:"notifications_enabled"`
}

// TypingIndicator represents a user currently typing
type TypingIndicator struct {
	RoomID    string    `json:"room_id"`
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	StartedAt time.Time `json:"started_at"`
	ExpiresAt time.Time `json:"expires_at"` // Auto-expire after ~3 seconds
}

// OnlineStatus represents a user's online presence
type OnlineStatus struct {
	UserID      string    `json:"user_id"`
	Username    string    `json:"username"`
	Status      string    `json:"status"` // online, away, busy, offline
	LastSeenAt  time.Time `json:"last_seen_at"`
	CurrentRoom string    `json:"current_room,omitempty"`
}

// ChatEvent represents a WebSocket event for chat
type ChatEvent struct {
	Type      string      `json:"type"` // message, typing, presence, room_update
	Payload   interface{} `json:"payload"`
	Timestamp time.Time   `json:"timestamp"`
}

// Room types
const (
	RoomTypePublic  = "public"
	RoomTypePrivate = "private"
	RoomTypeManga   = "manga"  // Discussion room for a specific manga
	RoomTypeDirect  = "direct" // DM between two users
)

// Message types
const (
	MessageTypeText       = "text"
	MessageTypeImage      = "image"
	MessageTypeSystem     = "system"      // Join/leave notifications
	MessageTypeMangaShare = "manga_share" // Shared manga link
)

// Member roles
const (
	RoleMember    = "member"
	RoleModerator = "moderator"
	RoleAdmin     = "admin"
	RoleOwner     = "owner"
)

// Online status values
const (
	StatusOnline  = "online"
	StatusAway    = "away"
	StatusBusy    = "busy"
	StatusOffline = "offline"
)
