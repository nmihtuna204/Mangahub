// Package database - Database Connection and Schema Management
// Xử lý SQLite database connections và migrations
// Chức năng:
//   - Initialize SQLite database connection
//   - Run schema migrations (CREATE TABLE statements)
//   - Connection pooling configuration
//   - Health check queries
//   - Seed initial data
//   - Pure Go SQLite driver (glebarez/go-sqlite)
package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
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
	// Ensure directory exists
	dir := filepath.Dir(config.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

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

	// Seed initial data if empty
	if err := db.Seed(); err != nil {
		return nil, fmt.Errorf("failed to seed database: %w", err)
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
		// ===== Core Tables =====
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			username TEXT UNIQUE NOT NULL,
			email TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			display_name TEXT NOT NULL,
			role TEXT DEFAULT 'user' CHECK (role IN ('user', 'admin', 'moderator')),
			is_active BOOLEAN DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_login_at DATETIME
		)`,

		`CREATE TABLE IF NOT EXISTS manga (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			author TEXT,
			artist TEXT,
			description TEXT,
			cover_url TEXT,
			status TEXT DEFAULT 'ongoing' CHECK (status IN ('ongoing', 'completed', 'hiatus', 'cancelled')),
			type TEXT DEFAULT 'manga' CHECK (type IN ('manga', 'manhwa', 'manhua', 'novel')),
			total_chapters INTEGER DEFAULT 0,
			average_rating REAL DEFAULT 0.0 CHECK (average_rating BETWEEN 0 AND 10),
			rating_count INTEGER DEFAULT 0,
			year INTEGER,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS genres (
			id TEXT PRIMARY KEY,
			name TEXT UNIQUE NOT NULL,
			slug TEXT UNIQUE NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS manga_genres (
			id TEXT PRIMARY KEY,
			manga_id TEXT NOT NULL,
			genre_id TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (manga_id) REFERENCES manga(id) ON DELETE CASCADE,
			FOREIGN KEY (genre_id) REFERENCES genres(id) ON DELETE CASCADE,
			UNIQUE(manga_id, genre_id)
		)`,

		// ===== Full-text Search =====
		`CREATE VIRTUAL TABLE IF NOT EXISTS manga_fts USING fts5(
			id UNINDEXED,
			title,
			author,
			description,
			content='manga'
		)`,

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

		// ===== External IDs =====
		`CREATE TABLE IF NOT EXISTS manga_external_ids (
			manga_id TEXT PRIMARY KEY,
			mangadex_id TEXT,
			anilist_id INTEGER,
			mal_id INTEGER,
			kitsu_id TEXT,
			primary_source TEXT DEFAULT 'mangadex',
			last_synced_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (manga_id) REFERENCES manga(id) ON DELETE CASCADE
		)`,

		// ===== User Reading Progress =====
		`CREATE TABLE IF NOT EXISTS reading_progress (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			manga_id TEXT NOT NULL,
			current_chapter INTEGER DEFAULT 0,
			status TEXT DEFAULT 'plan_to_read' CHECK (status IN ('plan_to_read', 'reading', 'completed', 'on_hold', 'dropped')),
			is_favorite BOOLEAN DEFAULT 0,
			started_at DATETIME,
			completed_at DATETIME,
			last_read_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY (manga_id) REFERENCES manga(id) ON DELETE CASCADE,
			UNIQUE(user_id, manga_id)
		)`,

		// ===== Ratings =====
		`CREATE TABLE IF NOT EXISTS manga_ratings (
			id TEXT PRIMARY KEY,
			manga_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			rating INTEGER NOT NULL CHECK (rating BETWEEN 1 AND 10),
			review_text TEXT,
			is_spoiler BOOLEAN DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (manga_id) REFERENCES manga(id) ON DELETE CASCADE,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			UNIQUE(manga_id, user_id)
		)`,

		`CREATE TRIGGER IF NOT EXISTS update_manga_rating_insert AFTER INSERT ON manga_ratings BEGIN
			UPDATE manga 
			SET average_rating = (SELECT AVG(rating) FROM manga_ratings WHERE manga_id = new.manga_id),
				rating_count = (SELECT COUNT(*) FROM manga_ratings WHERE manga_id = new.manga_id)
			WHERE id = new.manga_id;
		END`,

		`CREATE TRIGGER IF NOT EXISTS update_manga_rating_update AFTER UPDATE ON manga_ratings BEGIN
			UPDATE manga 
			SET average_rating = (SELECT AVG(rating) FROM manga_ratings WHERE manga_id = new.manga_id)
			WHERE id = new.manga_id;
		END`,

		`CREATE TRIGGER IF NOT EXISTS update_manga_rating_delete AFTER DELETE ON manga_ratings BEGIN
			UPDATE manga 
			SET average_rating = (SELECT COALESCE(AVG(rating), 0) FROM manga_ratings WHERE manga_id = old.manga_id),
				rating_count = (SELECT COUNT(*) FROM manga_ratings WHERE manga_id = old.manga_id)
			WHERE id = old.manga_id;
		END`,

		// ===== Comments =====
		`CREATE TABLE IF NOT EXISTS comments (
			id TEXT PRIMARY KEY,
			manga_id TEXT NOT NULL,
			chapter_number INTEGER,
			user_id TEXT NOT NULL,
			content TEXT NOT NULL,
			parent_id TEXT,
			likes_count INTEGER DEFAULT 0,
			is_spoiler BOOLEAN DEFAULT 0,
			is_edited BOOLEAN DEFAULT 0,
			is_deleted BOOLEAN DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (manga_id) REFERENCES manga(id) ON DELETE CASCADE,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY (parent_id) REFERENCES comments(id) ON DELETE SET NULL
		)`,

		`CREATE TABLE IF NOT EXISTS comment_likes (
			id TEXT PRIMARY KEY,
			comment_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (comment_id) REFERENCES comments(id) ON DELETE CASCADE,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			UNIQUE(comment_id, user_id)
		)`,

		`CREATE TRIGGER IF NOT EXISTS increment_comment_likes AFTER INSERT ON comment_likes BEGIN
			UPDATE comments SET likes_count = likes_count + 1 WHERE id = new.comment_id;
		END`,

		`CREATE TRIGGER IF NOT EXISTS decrement_comment_likes AFTER DELETE ON comment_likes BEGIN
			UPDATE comments SET likes_count = likes_count - 1 WHERE id = old.comment_id;
		END`,

		// ===== Chat =====
		`CREATE TABLE IF NOT EXISTS chat_rooms (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			room_type TEXT DEFAULT 'manga' CHECK (room_type IN ('general', 'manga')),
			manga_id TEXT,
			owner_id TEXT NOT NULL,
			description TEXT,
			is_active BOOLEAN DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (manga_id) REFERENCES manga(id) ON DELETE SET NULL,
			FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE CASCADE
		)`,

		`CREATE TABLE IF NOT EXISTS chat_room_members (
			id TEXT PRIMARY KEY,
			room_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			role TEXT DEFAULT 'member' CHECK (role IN ('owner', 'moderator', 'member')),
			joined_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_read_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (room_id) REFERENCES chat_rooms(id) ON DELETE CASCADE,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			UNIQUE(room_id, user_id)
		)`,

		`CREATE TABLE IF NOT EXISTS chat_messages (
			id TEXT PRIMARY KEY,
			room_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			content TEXT NOT NULL,
			reply_to_id TEXT,
			is_edited BOOLEAN DEFAULT 0,
			is_deleted BOOLEAN DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (room_id) REFERENCES chat_rooms(id) ON DELETE CASCADE,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,

		// ===== Custom Lists =====
		`CREATE TABLE IF NOT EXISTS custom_lists (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			is_public BOOLEAN DEFAULT 0,
			sort_order INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,

		`CREATE TABLE IF NOT EXISTS custom_list_items (
			id TEXT PRIMARY KEY,
			list_id TEXT NOT NULL,
			manga_id TEXT NOT NULL,
			notes TEXT,
			sort_order INTEGER DEFAULT 0,
			added_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (list_id) REFERENCES custom_lists(id) ON DELETE CASCADE,
			FOREIGN KEY (manga_id) REFERENCES manga(id) ON DELETE CASCADE,
			UNIQUE(list_id, manga_id)
		)`,

		// ===== Activity Feed =====
		`CREATE TABLE IF NOT EXISTS activity_feed (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			username TEXT NOT NULL,
			activity_type TEXT NOT NULL CHECK (activity_type IN ('comment', 'rating', 'progress', 'list_add')),
			manga_id TEXT NOT NULL,
			manga_title TEXT NOT NULL,
			chapter_number INTEGER,
			rating REAL,
			comment_text TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY (manga_id) REFERENCES manga(id) ON DELETE CASCADE
		)`,

		`CREATE TRIGGER IF NOT EXISTS activity_on_comment AFTER INSERT ON comments BEGIN
			INSERT INTO activity_feed (id, user_id, username, activity_type, manga_id, manga_title, chapter_number, comment_text, created_at)
			SELECT
				'act-' || new.id,
				new.user_id,
				u.username,
				'comment',
				new.manga_id,
				m.title,
				new.chapter_number,
				new.content,
				new.created_at
			FROM users u, manga m
			WHERE u.id = new.user_id AND m.id = new.manga_id;
		END`,

		`CREATE TRIGGER IF NOT EXISTS activity_on_rating AFTER INSERT ON manga_ratings BEGIN
			INSERT INTO activity_feed (id, user_id, username, activity_type, manga_id, manga_title, rating, created_at)
			SELECT
				'act-' || new.id,
				new.user_id,
				u.username,
				'rating',
				new.manga_id,
				m.title,
				new.rating,
				new.created_at
			FROM users u, manga m
			WHERE u.id = new.user_id AND m.id = new.manga_id;
		END`,

		// ===== Indexes =====
		`CREATE INDEX IF NOT EXISTS idx_users_username ON users(username)`,
		`CREATE INDEX IF NOT EXISTS idx_users_email ON users(email)`,
		`CREATE INDEX IF NOT EXISTS idx_manga_title ON manga(title)`,
		`CREATE INDEX IF NOT EXISTS idx_manga_status ON manga(status)`,
		`CREATE INDEX IF NOT EXISTS idx_manga_type ON manga(type)`,
		`CREATE INDEX IF NOT EXISTS idx_manga_rating ON manga(average_rating DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_manga_genres_manga ON manga_genres(manga_id)`,
		`CREATE INDEX IF NOT EXISTS idx_manga_genres_genre ON manga_genres(genre_id)`,
		`CREATE INDEX IF NOT EXISTS idx_external_mangadex ON manga_external_ids(mangadex_id)`,
		`CREATE INDEX IF NOT EXISTS idx_external_mal ON manga_external_ids(mal_id)`,
		`CREATE INDEX IF NOT EXISTS idx_external_anilist ON manga_external_ids(anilist_id)`,
		`CREATE INDEX IF NOT EXISTS idx_progress_user ON reading_progress(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_progress_manga ON reading_progress(manga_id)`,
		`CREATE INDEX IF NOT EXISTS idx_progress_status ON reading_progress(status)`,
		`CREATE INDEX IF NOT EXISTS idx_progress_favorite ON reading_progress(is_favorite) WHERE is_favorite = 1`,
		`CREATE INDEX IF NOT EXISTS idx_progress_last_read ON reading_progress(last_read_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_ratings_manga ON manga_ratings(manga_id)`,
		`CREATE INDEX IF NOT EXISTS idx_ratings_user ON manga_ratings(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_ratings_created ON manga_ratings(created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_comments_manga ON comments(manga_id)`,
		`CREATE INDEX IF NOT EXISTS idx_comments_chapter ON comments(manga_id, chapter_number)`,
		`CREATE INDEX IF NOT EXISTS idx_comments_user ON comments(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_comments_parent ON comments(parent_id)`,
		`CREATE INDEX IF NOT EXISTS idx_comments_created ON comments(created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_comment_likes_comment ON comment_likes(comment_id)`,
		`CREATE INDEX IF NOT EXISTS idx_comment_likes_user ON comment_likes(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_chat_rooms_type ON chat_rooms(room_type)`,
		`CREATE INDEX IF NOT EXISTS idx_chat_rooms_manga ON chat_rooms(manga_id)`,
		`CREATE INDEX IF NOT EXISTS idx_room_members_room ON chat_room_members(room_id)`,
		`CREATE INDEX IF NOT EXISTS idx_room_members_user ON chat_room_members(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_chat_messages_room ON chat_messages(room_id)`,
		`CREATE INDEX IF NOT EXISTS idx_chat_messages_created ON chat_messages(created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_custom_lists_user ON custom_lists(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_custom_list_items_list ON custom_list_items(list_id)`,
		`CREATE INDEX IF NOT EXISTS idx_custom_list_items_manga ON custom_list_items(manga_id)`,
		`CREATE INDEX IF NOT EXISTS idx_activity_created ON activity_feed(created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_activity_user ON activity_feed(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_activity_manga ON activity_feed(manga_id)`,
		`CREATE INDEX IF NOT EXISTS idx_activity_type ON activity_feed(activity_type)`,
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

// HealthCheck verifies database connectivity and returns status info
func (db *DB) HealthCheck() (map[string]interface{}, error) {
	result := make(map[string]interface{})

	// Test connection with ping
	start := time.Now()
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("database ping failed: %w", err)
	}
	result["ping_latency_ms"] = time.Since(start).Milliseconds()
	result["connected"] = true

	// Get some basic stats
	stats := db.Stats()
	result["open_connections"] = stats.OpenConnections
	result["in_use"] = stats.InUse
	result["idle"] = stats.Idle
	result["wait_count"] = stats.WaitCount
	result["max_open_connections"] = stats.MaxOpenConnections

	// Quick query test
	var tableCount int
	err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table'").Scan(&tableCount)
	if err != nil {
		return nil, fmt.Errorf("failed to query tables: %w", err)
	}
	result["table_count"] = tableCount

	// Get database file size (if possible)
	var pageCount, pageSize int64
	if err := db.QueryRow("PRAGMA page_count").Scan(&pageCount); err == nil {
		if err := db.QueryRow("PRAGMA page_size").Scan(&pageSize); err == nil {
			result["database_size_bytes"] = pageCount * pageSize
		}
	}

	result["status"] = "healthy"
	return result, nil
}
