// Package models - Comment System for Chapter Discussions
// Hệ thống bình luận cho manga chapters
// Chức năng:
//   - Comments on manga/chapters with spoiler support
//   - Threaded replies via parent_id
//   - Like/unlike comments
//   - Edit and soft-delete support
package models

import (
	"time"
)

// Comment represents a user comment on a manga or chapter
type Comment struct {
	ID            string    `json:"id" db:"id"`
	MangaID       string    `json:"manga_id" db:"manga_id"`
	ChapterNumber *int      `json:"chapter_number,omitempty" db:"chapter_number"` // nil = manga-level comment
	UserID        string    `json:"user_id" db:"user_id"`
	Content       string    `json:"content" db:"content"`
	IsSpoiler     bool      `json:"is_spoiler" db:"is_spoiler"`
	ParentID      *string   `json:"parent_id,omitempty" db:"parent_id"` // For threaded replies
	LikesCount    int       `json:"likes_count" db:"likes_count"`
	IsEdited      bool      `json:"is_edited" db:"is_edited"`
	IsDeleted     bool      `json:"is_deleted" db:"is_deleted"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}

// CommentLike tracks which users liked a comment
type CommentLike struct {
	ID        string    `json:"id" db:"id"`
	CommentID string    `json:"comment_id" db:"comment_id"`
	UserID    string    `json:"user_id" db:"user_id"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// CommentWithUser includes user info for display
type CommentWithUser struct {
	Comment
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url,omitempty"`
	LikedByMe   bool   `json:"liked_by_me"` // Whether current user liked this comment
}

// CommentWithReplies includes nested replies
type CommentWithReplies struct {
	CommentWithUser
	Replies []CommentWithUser `json:"replies,omitempty"`
}

// ===== Request/Response Types for Comment API =====

// CreateCommentRequest is the payload for creating a comment
type CreateCommentRequest struct {
	Content       string `json:"content" validate:"required,min=1,max=2000"`
	ChapterNumber *int   `json:"chapter_number,omitempty"`
	IsSpoiler     bool   `json:"is_spoiler"`
	ParentID      string `json:"parent_id,omitempty"` // For replies
}

// UpdateCommentRequest is the payload for editing a comment
type UpdateCommentRequest struct {
	Content   string `json:"content" validate:"required,min=1,max=2000"`
	IsSpoiler bool   `json:"is_spoiler"`
}

// CommentListResponse is paginated list of comments
type CommentListResponse struct {
	Comments   []CommentWithReplies `json:"comments"`
	TotalCount int                  `json:"total_count"`
	Page       int                  `json:"page"`
	PageSize   int                  `json:"page_size"`
	HasMore    bool                 `json:"has_more"`
}

// Activity represents a user action for the activity feed
// Auto-populated by database triggers
type Activity struct {
	ID            string    `json:"id" db:"id"`
	UserID        string    `json:"user_id" db:"user_id"`
	Username      string    `json:"username" db:"username"`
	ActivityType  string    `json:"activity_type" db:"activity_type"` // comment, rating, progress, list_add
	MangaID       string    `json:"manga_id" db:"manga_id"`
	MangaTitle    string    `json:"manga_title" db:"manga_title"`
	ChapterNumber *int      `json:"chapter_number,omitempty" db:"chapter_number"`
	Rating        *float64  `json:"rating,omitempty" db:"rating"`
	CommentText   string    `json:"comment_text,omitempty" db:"comment_text"` // Empty string if null
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}

// ActivityWithDetails includes related info for display (kept for compatibility)
type ActivityWithDetails struct {
	Activity
}

// Activity action types
const (
	ActivityComment  = "comment"  // User commented
	ActivityRating   = "rating"   // User rated a manga
	ActivityProgress = "progress" // User updated reading progress
	ActivityListAdd  = "list_add" // User added manga to custom list
)
