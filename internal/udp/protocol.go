package udp

import (
	"fmt"
	"time"
)

// Notification represents a UDP notification message
type Notification struct {
	Type       string `json:"type"`                  // notification type: chapter_release, progress_update, system, etc.
	MangaID    string `json:"manga_id"`              // manga identifier
	Message    string `json:"message"`               // notification message
	Timestamp  int64  `json:"timestamp"`             // unix timestamp
	MangaTitle string `json:"manga_title,omitempty"` // optional display name for the manga
	Chapter    int    `json:"chapter,omitempty"`     // optional chapter number
	Username   string `json:"username,omitempty"`    // optional user who triggered the notification

	// UserIDs limits delivery to subscribers registered as these users. The
	// server uses it for routing and strips it before sending.
	UserIDs []string `json:"user_ids,omitempty"`
}

// NewChapterNotification creates a chapter release notification
func NewChapterNotification(mangaID, message string) Notification {
	return Notification{
		Type:      "chapter_release",
		MangaID:   mangaID,
		Message:   message,
		Timestamp: time.Now().Unix(),
	}
}

// NewChapterReleaseNotification announces a new chapter to the given users
func NewChapterReleaseNotification(mangaID, mangaTitle string, chapter int, userIDs []string) Notification {
	return Notification{
		Type:       "chapter_release",
		MangaID:    mangaID,
		Message:    fmt.Sprintf("Chapter %d of %s is out!", chapter, mangaTitle),
		Timestamp:  time.Now().Unix(),
		MangaTitle: mangaTitle,
		Chapter:    chapter,
		UserIDs:    userIDs,
	}
}

// NewProgressNotification creates a notification about a user's reading progress
func NewProgressNotification(mangaID, mangaTitle, username string, chapter int, message string) Notification {
	return Notification{
		Type:       "progress_update",
		MangaID:    mangaID,
		Message:    message,
		Timestamp:  time.Now().Unix(),
		MangaTitle: mangaTitle,
		Chapter:    chapter,
		Username:   username,
	}
}

// NewSystemNotification creates a system notification
func NewSystemNotification(message string) Notification {
	return Notification{
		Type:      "system",
		MangaID:   "",
		Message:   message,
		Timestamp: time.Now().Unix(),
	}
}
