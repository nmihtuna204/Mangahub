package models

import (
	"time"
)

// UserPreferences represents user settings and preferences
type UserPreferences struct {
	UserID               string      `json:"user_id" db:"user_id"`
	Theme                string      `json:"theme" db:"theme"`       // dark, light, dracula, nord
	Language             string      `json:"language" db:"language"` // en, vi, jp, etc.
	ChaptersPerPage      int         `json:"chapters_per_page" db:"chapters_per_page"`
	ReadingDirection     string      `json:"reading_direction" db:"reading_direction"` // ltr, rtl
	DefaultStatus        string      `json:"default_status" db:"default_status"`       // reading, plan_to_read
	ShowSpoilers         bool        `json:"show_spoilers" db:"show_spoilers"`
	AutoSync             bool        `json:"auto_sync" db:"auto_sync"`
	NotificationsEnabled bool        `json:"notifications_enabled" db:"notifications_enabled"`
	EmailNotifications   bool        `json:"email_notifications" db:"email_notifications"`
	ActivityPublic       bool        `json:"activity_public" db:"activity_public"`
	LibraryPublic        bool        `json:"library_public" db:"library_public"`
	KeybindingsJSON      string      `json:"-" db:"keybindings"`
	Keybindings          Keybindings `json:"keybindings" db:"-"`
	CreatedAt            time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt            time.Time   `json:"updated_at" db:"updated_at"`
}

// Keybindings represents customizable keyboard shortcuts
type Keybindings struct {
	NextChapter    string `json:"next_chapter"`
	PrevChapter    string `json:"prev_chapter"`
	ToggleFavorite string `json:"toggle_favorite"`
	OpenSearch     string `json:"open_search"`
	GoToLibrary    string `json:"go_to_library"`
	GoToDashboard  string `json:"go_to_dashboard"`
	ShowHelp       string `json:"show_help"`
	Quit           string `json:"quit"`
}

// DefaultKeybindings returns the default keybindings
func DefaultKeybindings() Keybindings {
	return Keybindings{
		NextChapter:    "n",
		PrevChapter:    "p",
		ToggleFavorite: "f",
		OpenSearch:     "/",
		GoToLibrary:    "l",
		GoToDashboard:  "d",
		ShowHelp:       "?",
		Quit:           "q",
	}
}

// DefaultPreferences returns default user preferences
func DefaultPreferences(userID string) UserPreferences {
	return UserPreferences{
		UserID:               userID,
		Theme:                "dracula",
		Language:             "en",
		ChaptersPerPage:      20,
		ReadingDirection:     "ltr",
		DefaultStatus:        "reading",
		ShowSpoilers:         false,
		AutoSync:             true,
		NotificationsEnabled: true,
		EmailNotifications:   false,
		ActivityPublic:       true,
		LibraryPublic:        true,
		Keybindings:          DefaultKeybindings(),
	}
}

// UpdatePreferencesRequest is used to update user preferences
type UpdatePreferencesRequest struct {
	Theme                *string      `json:"theme,omitempty" validate:"omitempty,oneof=dark light dracula nord"`
	Language             *string      `json:"language,omitempty" validate:"omitempty,len=2"`
	ChaptersPerPage      *int         `json:"chapters_per_page,omitempty" validate:"omitempty,min=5,max=100"`
	ReadingDirection     *string      `json:"reading_direction,omitempty" validate:"omitempty,oneof=ltr rtl"`
	DefaultStatus        *string      `json:"default_status,omitempty" validate:"omitempty,oneof=reading plan_to_read"`
	ShowSpoilers         *bool        `json:"show_spoilers,omitempty"`
	AutoSync             *bool        `json:"auto_sync,omitempty"`
	NotificationsEnabled *bool        `json:"notifications_enabled,omitempty"`
	EmailNotifications   *bool        `json:"email_notifications,omitempty"`
	ActivityPublic       *bool        `json:"activity_public,omitempty"`
	LibraryPublic        *bool        `json:"library_public,omitempty"`
	Keybindings          *Keybindings `json:"keybindings,omitempty"`
}

// ExportDataRequest is used to request data export
type ExportDataRequest struct {
	IncludeLibrary  bool   `json:"include_library"`
	IncludeProgress bool   `json:"include_progress"`
	IncludeStats    bool   `json:"include_stats"`
	IncludeLists    bool   `json:"include_lists"`
	Format          string `json:"format" validate:"oneof=json csv"` // json, csv
}

// ExportDataResponse contains the exported data
type ExportDataResponse struct {
	UserID     string    `json:"user_id"`
	ExportedAt time.Time `json:"exported_at"`
	Format     string    `json:"format"`
	Data       string    `json:"data"` // base64 encoded data
	Filename   string    `json:"filename"`
}
