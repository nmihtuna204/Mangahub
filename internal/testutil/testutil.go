// Package testutil provides shared helpers for tests.
package testutil

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"mangahub/pkg/database"
)

// dbConfig mirrors the production pool limits (configs/development.yaml) so
// connection-pool behavior in tests matches the real servers.
func dbConfig(t testing.TB) database.Config {
	return database.Config{
		Path:            filepath.Join(t.TempDir(), "test.db"),
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
	}
}

// NewDB returns an empty database with the production schema (database.Migrate).
// It is file-backed rather than ":memory:" so every pooled connection sees the
// same data, which concurrency tests rely on.
func NewDB(t testing.TB) *database.DB {
	t.Helper()
	db, err := database.Open(dbConfig(t))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// NewSeededDB returns a database with the production schema and seed data
// (admin/admin123, reader1/reader2/mangafan with password123, 15 genres, 101 manga).
func NewSeededDB(t testing.TB) *database.DB {
	t.Helper()
	db, err := database.NewDB(dbConfig(t))
	if err != nil {
		t.Fatalf("open seeded test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// MustExec runs a statement or fails the test.
func MustExec(t testing.TB, db *sql.DB, query string, args ...interface{}) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

// AddUser inserts a user with the given ID (username = id).
func AddUser(t testing.TB, db *sql.DB, id string) {
	t.Helper()
	MustExec(t, db, `INSERT INTO users (id, username, email, password_hash, display_name) VALUES (?, ?, ?, 'x', ?)`,
		id, id, id+"@example.com", id)
}

// AddManga inserts a manga with the given ID and title.
func AddManga(t testing.TB, db *sql.DB, id, title string) {
	t.Helper()
	MustExec(t, db, `INSERT INTO manga (id, title, author, total_chapters) VALUES (?, ?, 'Author', 100)`, id, title)
}

// AddGenre inserts a genre and links it to the given manga.
func AddGenre(t testing.TB, db *sql.DB, mangaID, name, slug string) {
	t.Helper()
	MustExec(t, db, `INSERT OR IGNORE INTO genres (id, name, slug) VALUES (?, ?, ?)`, "g-"+slug, name, slug)
	MustExec(t, db, `INSERT INTO manga_genres (id, manga_id, genre_id) VALUES (?, ?, ?)`, mangaID+"-"+slug, mangaID, "g-"+slug)
}
