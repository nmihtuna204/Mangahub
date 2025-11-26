package models

import (
	"time"
)

// Manga represents a manga/comic
type Manga struct {
	ID            string    `json:"id" db:"id"`
	Title         string    `json:"title" db:"title" validate:"required"`
	Author        string    `json:"author" db:"author"`
	Artist        string    `json:"artist" db:"artist"`
	Description   string    `json:"description" db:"description"`
	CoverURL      string    `json:"cover_url" db:"cover_url"`
	Status        string    `json:"status" db:"status"` // ongoing, completed, hiatus
	Type          string    `json:"type" db:"type"`     // manga, manhwa, manhua
	Genres        []string  `json:"genres" db:"-"`      // stored as JSON in DB
	GenresJSON    string    `json:"-" db:"genres"`
	TotalChapters int       `json:"total_chapters" db:"total_chapters"`
	Rating        float64   `json:"rating" db:"rating"`
	Year          int       `json:"year" db:"year"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}

// MangaSearchRequest represents search parameters
type MangaSearchRequest struct {
	Query  string   `json:"query" form:"query"`
	Genres []string `json:"genres" form:"genres"`
	Status string   `json:"status" form:"status"`
	Type   string   `json:"type" form:"type"`
	Limit  int      `json:"limit" form:"limit" validate:"min=1,max=100"`
	Offset int      `json:"offset" form:"offset" validate:"min=0"`
	SortBy string   `json:"sort_by" form:"sort_by"` // title, rating, year
	Order  string   `json:"order" form:"order"`     // asc, desc
}

// MangaListResponse represents paginated manga results
type MangaListResponse struct {
	Data    []Manga `json:"data"`
	Total   int     `json:"total"`
	Limit   int     `json:"limit"`
	Offset  int     `json:"offset"`
	HasMore bool    `json:"has_more"`
}

// ValidateMangaSearch validates manga search request
func ValidateMangaSearch(req *MangaSearchRequest) error {
	if req.Limit <= 0 {
		req.Limit = 20
	}
	if req.Limit > 100 {
		req.Limit = 100
	}
	if req.Offset < 0 {
		req.Offset = 0
	}
	return nil
}
