// Package models - Genre and Manga-Genre Mapping
// Normalized genre system (replaces JSON array)
// Chức năng:
//   - Genre taxonomy for manga categorization
//   - Many-to-many relationship via manga_genres table
//   - Supports genre-based filtering and discovery
package models

import (
	"time"
)

// Genre represents a manga genre/category
type Genre struct {
	ID        string    `json:"id" db:"id"`
	Name      string    `json:"name" db:"name" validate:"required,min=1,max=50"`
	Slug      string    `json:"slug" db:"slug" validate:"required,min=1,max=50"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// MangaGenre represents the many-to-many relationship between manga and genres
type MangaGenre struct {
	ID        string    `json:"id" db:"id"`
	MangaID   string    `json:"manga_id" db:"manga_id"`
	GenreID   string    `json:"genre_id" db:"genre_id"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// MangaWithGenres combines manga with its genres
type MangaWithGenres struct {
	Manga
	Genres []Genre `json:"genres"`
}

// GenreWithCount includes manga count for statistics
type GenreWithCount struct {
	Genre
	MangaCount int `json:"manga_count"`
}

// Common genre slugs (for seeding/reference)
const (
	GenreAction        = "action"
	GenreAdventure     = "adventure"
	GenreComedy        = "comedy"
	GenreDrama         = "drama"
	GenreFantasy       = "fantasy"
	GenreHorror        = "horror"
	GenreIseki         = "isekai"
	GenreMecha         = "mecha"
	GenreMystery       = "mystery"
	GenrePsychological = "psychological"
	GenreRomance       = "romance"
	GenreSciFi         = "sci-fi"
	GenreSliceOfLife   = "slice-of-life"
	GenreSports        = "sports"
	GenreSupernatural  = "supernatural"
	GenreThriller      = "thriller"
)
