package udp

import "time"

// Notification represents a UDP notification message
type Notification struct {
	Type      string `json:"type"`       // notification type: chapter_release, system, etc.
	MangaID   string `json:"manga_id"`   // manga identifier
	Message   string `json:"message"`    // notification message
	Timestamp int64  `json:"timestamp"`  // unix timestamp
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

// NewSystemNotification creates a system notification
func NewSystemNotification(message string) Notification {
	return Notification{
		Type:      "system",
		MangaID:   "",
		Message:   message,
		Timestamp: time.Now().Unix(),
	}
}
