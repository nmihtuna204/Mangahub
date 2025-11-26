package database

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/glebarez/go-sqlite"
)

// DB wraps the sql.DB connection
type DB struct {
	*sql.DB
}

// Config holds database configuration
type Config struct {
	Path            string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// NewDB creates a new database connection
func NewDB(config Config) (*DB, error) {
	// Open database connection
	sqlDB, err := sql.Open("sqlite", config.Path+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Set connection pool settings
	sqlDB.SetMaxOpenConns(config.MaxOpenConns)
	sqlDB.SetMaxIdleConns(config.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(config.ConnMaxLifetime)

	// Verify connection
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db := &DB{sqlDB}

	// Run migrations
	if err := db.Migrate(); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return db, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.DB.Close()
}

// Migrate runs database migrations
func (db *DB) Migrate() error {
	migrations := []string{
		// Users table
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			username TEXT UNIQUE NOT NULL,
			email TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			display_name TEXT NOT NULL,
			bio TEXT,
			avatar_url TEXT,
			role TEXT DEFAULT 'user',
			is_active BOOLEAN DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_login_at DATETIME
		)`,

		// Manga table
		`CREATE TABLE IF NOT EXISTS manga (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			author TEXT,
			artist TEXT,
			description TEXT,
			cover_url TEXT,
			status TEXT DEFAULT 'ongoing',
			type TEXT DEFAULT 'manga',
			genres TEXT,
			total_chapters INTEGER DEFAULT 0,
			rating REAL DEFAULT 0,
			year INTEGER,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Reading progress table
		`CREATE TABLE IF NOT EXISTS reading_progress (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			manga_id TEXT NOT NULL,
			current_chapter INTEGER DEFAULT 0,
			total_chapters INTEGER DEFAULT 0,
			status TEXT DEFAULT 'plan_to_read',
			rating INTEGER,
			notes TEXT,
			is_favorite BOOLEAN DEFAULT 0,
			started_at DATETIME,
			completed_at DATETIME,
			last_read_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			sync_version INTEGER DEFAULT 1,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY (manga_id) REFERENCES manga(id) ON DELETE CASCADE,
			UNIQUE(user_id, manga_id)
		)`,

		// Indexes for performance
		`CREATE INDEX IF NOT EXISTS idx_users_username ON users(username)`,
		`CREATE INDEX IF NOT EXISTS idx_users_email ON users(email)`,
		`CREATE INDEX IF NOT EXISTS idx_manga_title ON manga(title)`,
		`CREATE INDEX IF NOT EXISTS idx_manga_status ON manga(status)`,
		`CREATE INDEX IF NOT EXISTS idx_manga_rating ON manga(rating DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_progress_user_id ON reading_progress(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_progress_manga_id ON reading_progress(manga_id)`,
		`CREATE INDEX IF NOT EXISTS idx_progress_status ON reading_progress(status)`,
		`CREATE INDEX IF NOT EXISTS idx_progress_last_read ON reading_progress(last_read_at DESC)`,

		// Full-text search for manga
		`CREATE VIRTUAL TABLE IF NOT EXISTS manga_fts USING fts5(
			id UNINDEXED,
			title,
			author,
			description,
			content='manga',
			content_rowid='rowid'
		)`,

		// Triggers to keep FTS in sync
		`CREATE TRIGGER IF NOT EXISTS manga_fts_insert AFTER INSERT ON manga BEGIN
			INSERT INTO manga_fts(id, title, author, description)
			VALUES (new.id, new.title, new.author, new.description);
		END`,

		`CREATE TRIGGER IF NOT EXISTS manga_fts_update AFTER UPDATE ON manga BEGIN
			UPDATE manga_fts SET title = new.title, author = new.author, description = new.description
			WHERE id = new.id;
		END`,

		`CREATE TRIGGER IF NOT EXISTS manga_fts_delete AFTER DELETE ON manga BEGIN
			DELETE FROM manga_fts WHERE id = old.id;
		END`,
	}

	for _, migration := range migrations {
		if _, err := db.Exec(migration); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}

	return nil
}

// BeginTx starts a new transaction
func (db *DB) BeginTx() (*sql.Tx, error) {
	return db.Begin()
}
