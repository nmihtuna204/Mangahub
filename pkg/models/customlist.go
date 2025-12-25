package models

import (
	"time"
)

// CustomList represents a user-created manga list
type CustomList struct {
	ID          string    `json:"id" db:"id"`
	UserID      string    `json:"user_id" db:"user_id"`
	Name        string    `json:"name" db:"name"`
	Description string    `json:"description" db:"description"`
	IsPublic    bool      `json:"is_public" db:"is_public"`
	SortOrder   int       `json:"sort_order" db:"sort_order"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// CustomListItem represents a manga in a custom list
type CustomListItem struct {
	ID        string    `json:"id" db:"id"`
	ListID    string    `json:"list_id" db:"list_id"`
	MangaID   string    `json:"manga_id" db:"manga_id"`
	Notes     string    `json:"notes" db:"notes"`
	SortOrder int       `json:"sort_order" db:"sort_order"`
	AddedAt   time.Time `json:"added_at" db:"added_at"`
}

// CustomListWithManga combines list item with manga details
type CustomListWithManga struct {
	CustomListItem
	Manga Manga `json:"manga"`
}

// CustomListWithItems is a list with all its items
type CustomListWithItems struct {
	CustomList
	Items []CustomListWithManga `json:"items"`
}

// CreateListRequest is used to create a new custom list
type CreateListRequest struct {
	Name        string `json:"name" validate:"required,min=1,max=100"`
	Description string `json:"description,omitempty" validate:"max=500"`
	IsPublic    bool   `json:"is_public"`
}

// UpdateListRequest is used to update a custom list
type UpdateListRequest struct {
	Name        *string `json:"name,omitempty" validate:"omitempty,min=1,max=100"`
	Description *string `json:"description,omitempty" validate:"omitempty,max=500"`
	IsPublic    *bool   `json:"is_public,omitempty"`
}

// AddToListRequest is used to add manga to a custom list
type AddToListRequest struct {
	MangaID   string `json:"manga_id" validate:"required"`
	Notes     string `json:"notes,omitempty"`
	SortOrder int    `json:"sort_order,omitempty"`
}

// ReorderListRequest is used to reorder items in a list
type ReorderListRequest struct {
	ItemIDs []string `json:"item_ids" validate:"required"`
}

// CustomListsResponse is a list of user's custom lists
type CustomListsResponse struct {
	Lists []CustomList `json:"lists"`
	Total int          `json:"total"`
}
