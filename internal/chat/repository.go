// Package chat - Chat Repository
// Data access layer cho chat messages & rooms
// Chức năng:
//   - Lưu trữ chat messages vào database
//   - Load lịch sử chat khi user join room
//   - Quản lý chat rooms
//   - Support pagination cho message history
package chat

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/google/uuid"
)

// =====================================
// MODELS - Chat related data structures
// =====================================

// Message represents a persisted chat message
type Message struct {
	ID        string    `json:"id"`
	RoomID    string    `json:"room_id"`
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"` // Populated from JOIN
	Content   string    `json:"content"`
	ReplyToID *string   `json:"reply_to_id,omitempty"`
	IsEdited  bool      `json:"is_edited"`
	IsDeleted bool      `json:"is_deleted"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Room represents a chat room
type Room struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	RoomType    string    `json:"room_type"` // general, manga
	MangaID     *string   `json:"manga_id,omitempty"`
	OwnerID     string    `json:"owner_id"`
	Description string    `json:"description"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// MessageListResponse for paginated message history
type MessageListResponse struct {
	Messages []Message `json:"messages"`
	Total    int       `json:"total"`
	Limit    int       `json:"limit"`
	Offset   int       `json:"offset"`
	HasMore  bool      `json:"has_more"`
}

// =====================================
// REPOSITORY - Database operations
// =====================================

// Repository interface for chat data access
type Repository interface {
	// Message operations
	SaveMessage(ctx context.Context, msg *Message) error
	GetMessagesByRoom(ctx context.Context, roomID string, limit, offset int) ([]Message, int, error)
	DeleteMessage(ctx context.Context, messageID, userID string) error

	// Room operations
	EnsureRoom(ctx context.Context, roomID, ownerID string) error
	CreateRoom(ctx context.Context, room *Room) error
	GetRoom(ctx context.Context, roomID string) (*Room, error)
	GetRoomByMangaID(ctx context.Context, mangaID string) (*Room, error)
}

type repository struct {
	db *sql.DB
}

// NewRepository creates a new chat repository
func NewRepository(db *sql.DB) Repository {
	return &repository{db: db}
}

// SaveMessage persists a chat message to database
// Được gọi mỗi khi có message được broadcast
func (r *repository) SaveMessage(ctx context.Context, msg *Message) error {
	if msg.ID == "" {
		msg.ID = uuid.New().String()
	}
	msg.CreatedAt = time.Now()
	msg.UpdatedAt = time.Now()

	query := `
		INSERT INTO chat_messages (id, room_id, user_id, content, reply_to_id, is_edited, is_deleted, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := r.db.ExecContext(ctx, query,
		msg.ID, msg.RoomID, msg.UserID, msg.Content,
		msg.ReplyToID, msg.IsEdited, msg.IsDeleted, msg.CreatedAt, msg.UpdatedAt)
	return err
}

// GetMessagesByRoom loads message history for a room
// Dùng để load lịch sử khi user join room
// Returns messages in chronological order (oldest first)
func (r *repository) GetMessagesByRoom(ctx context.Context, roomID string, limit, offset int) ([]Message, int, error) {
	// Get total count first
	var total int
	countQuery := `SELECT COUNT(*) FROM chat_messages WHERE room_id = ? AND is_deleted = 0`
	if err := r.db.QueryRowContext(ctx, countQuery, roomID).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Get messages with username from users table
	// Order by created_at ASC để tin nhắn cũ hiển thị trước
	query := `
		SELECT cm.id, cm.room_id, cm.user_id, COALESCE(u.username, 'Anonymous') as username,
		       cm.content, cm.reply_to_id, cm.is_edited, cm.is_deleted, 
		       cm.created_at, cm.updated_at
		FROM chat_messages cm
		LEFT JOIN users u ON cm.user_id = u.id
		WHERE cm.room_id = ? AND cm.is_deleted = 0
		ORDER BY cm.created_at DESC, cm.rowid DESC -- rowid breaks ties between messages sent in the same clock tick
		LIMIT ? OFFSET ?`

	rows, err := r.db.QueryContext(ctx, query, roomID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var msg Message
		err := rows.Scan(
			&msg.ID, &msg.RoomID, &msg.UserID, &msg.Username,
			&msg.Content, &msg.ReplyToID,
			&msg.IsEdited, &msg.IsDeleted, &msg.CreatedAt, &msg.UpdatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		messages = append(messages, msg)
	}

	// Reverse để có chronological order (oldest first)
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, total, nil
}

// DeleteMessage soft-deletes a message
// Chỉ user tạo message mới được xóa
func (r *repository) DeleteMessage(ctx context.Context, messageID, userID string) error {
	query := `UPDATE chat_messages SET is_deleted = 1, updated_at = ? WHERE id = ? AND user_id = ?`
	_, err := r.db.ExecContext(ctx, query, time.Now(), messageID, userID)
	return err
}

// EnsureRoom creates the chat_rooms row for roomID if it does not exist yet,
// so messages for rooms that clients join by ID can be persisted.
// "manga_<mangaID>" becomes that manga's discussion room; anything else is a general room.
func (r *repository) EnsureRoom(ctx context.Context, roomID, ownerID string) error {
	name, roomType := roomID, "general"
	var mangaID interface{}
	if id, ok := strings.CutPrefix(roomID, "manga_"); ok {
		var title string
		err := r.db.QueryRowContext(ctx, "SELECT title FROM manga WHERE id = ?", id).Scan(&title)
		switch {
		case err == nil:
			name, roomType, mangaID = title+" Discussion", "manga", id
		case err != sql.ErrNoRows:
			return err
		}
	}

	now := time.Now()
	_, err := r.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO chat_rooms (id, name, room_type, manga_id, owner_id, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 1, ?, ?)`,
		roomID, name, roomType, mangaID, ownerID, now, now)
	return err
}

// CreateRoom creates a new chat room
func (r *repository) CreateRoom(ctx context.Context, room *Room) error {
	if room.ID == "" {
		room.ID = uuid.New().String()
	}
	room.CreatedAt = time.Now()
	room.UpdatedAt = time.Now()

	query := `
		INSERT INTO chat_rooms (id, name, room_type, manga_id, owner_id, description, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := r.db.ExecContext(ctx, query,
		room.ID, room.Name, room.RoomType, room.MangaID, room.OwnerID,
		room.Description, room.IsActive, room.CreatedAt, room.UpdatedAt)
	return err
}

// GetRoom retrieves a room by ID
func (r *repository) GetRoom(ctx context.Context, roomID string) (*Room, error) {
	query := `SELECT id, name, room_type, manga_id, owner_id, COALESCE(description, ''), is_active, created_at, updated_at
	          FROM chat_rooms WHERE id = ?`

	var room Room
	err := r.db.QueryRowContext(ctx, query, roomID).Scan(
		&room.ID, &room.Name, &room.RoomType, &room.MangaID, &room.OwnerID,
		&room.Description, &room.IsActive, &room.CreatedAt, &room.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &room, nil
}

// GetRoomByMangaID retrieves a room by manga ID
func (r *repository) GetRoomByMangaID(ctx context.Context, mangaID string) (*Room, error) {
	query := `SELECT id, name, room_type, manga_id, owner_id, COALESCE(description, ''), is_active, created_at, updated_at
	          FROM chat_rooms WHERE manga_id = ?`

	var room Room
	err := r.db.QueryRowContext(ctx, query, mangaID).Scan(
		&room.ID, &room.Name, &room.RoomType, &room.MangaID, &room.OwnerID,
		&room.Description, &room.IsActive, &room.CreatedAt, &room.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &room, nil
}
