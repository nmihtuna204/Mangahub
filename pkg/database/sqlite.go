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

		// ===== Phase 0 New Tables =====

		// Chat messages table for real-time chat
		`CREATE TABLE IF NOT EXISTS chat_messages (
			id TEXT PRIMARY KEY,
			room_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			content TEXT NOT NULL,
			message_type TEXT DEFAULT 'text',
			reply_to_id TEXT,
			edited BOOLEAN DEFAULT 0,
			deleted BOOLEAN DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,

		// Chat rooms table
		`CREATE TABLE IF NOT EXISTS chat_rooms (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			room_type TEXT DEFAULT 'public',
			manga_id TEXT,
			owner_id TEXT NOT NULL,
			description TEXT,
			max_members INTEGER DEFAULT 100,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (manga_id) REFERENCES manga(id) ON DELETE SET NULL,
			FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE CASCADE
		)`,

		// Chat room members
		`CREATE TABLE IF NOT EXISTS chat_room_members (
			id TEXT PRIMARY KEY,
			room_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			role TEXT DEFAULT 'member',
			joined_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_read_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			notifications_enabled BOOLEAN DEFAULT 1,
			FOREIGN KEY (room_id) REFERENCES chat_rooms(id) ON DELETE CASCADE,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			UNIQUE(room_id, user_id)
		)`,

		// Manga ratings table
		`CREATE TABLE IF NOT EXISTS manga_ratings (
			id TEXT PRIMARY KEY,
			manga_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			overall_rating REAL NOT NULL,
			story_rating REAL,
			art_rating REAL,
			character_rating REAL,
			enjoyment_rating REAL,
			review_text TEXT,
			is_spoiler BOOLEAN DEFAULT 0,
			helpful_count INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (manga_id) REFERENCES manga(id) ON DELETE CASCADE,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			UNIQUE(manga_id, user_id)
		)`,

		// Achievements table
		`CREATE TABLE IF NOT EXISTS achievements (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			description TEXT NOT NULL,
			category TEXT NOT NULL,
			tier TEXT DEFAULT 'bronze',
			points INTEGER DEFAULT 10,
			icon_url TEXT,
			requirement_type TEXT NOT NULL,
			requirement_value INTEGER NOT NULL,
			is_active BOOLEAN DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// User achievements (earned achievements)
		`CREATE TABLE IF NOT EXISTS user_achievements (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			achievement_id TEXT NOT NULL,
			progress INTEGER DEFAULT 0,
			unlocked BOOLEAN DEFAULT 0,
			unlocked_at DATETIME,
			notified BOOLEAN DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY (achievement_id) REFERENCES achievements(id) ON DELETE CASCADE,
			UNIQUE(user_id, achievement_id)
		)`,

		// External IDs for manga cross-referencing
		`CREATE TABLE IF NOT EXISTS manga_external_ids (
			id TEXT PRIMARY KEY,
			manga_id TEXT NOT NULL,
			mangadex_id TEXT,
			anilist_id INTEGER,
			mal_id INTEGER,
			kitsu_id TEXT,
			primary_source TEXT DEFAULT 'mangadex',
			last_synced_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (manga_id) REFERENCES manga(id) ON DELETE CASCADE,
			UNIQUE(manga_id)
		)`,

		// ===== Phase 2 New Tables =====

		// Comments table for chapter discussions
		// Supports threaded replies via parent_id
		// Spoiler flag for content warnings
		`CREATE TABLE IF NOT EXISTS comments (
			id TEXT PRIMARY KEY,
			manga_id TEXT NOT NULL,
			chapter_number INTEGER,
			user_id TEXT NOT NULL,
			content TEXT NOT NULL,
			is_spoiler BOOLEAN DEFAULT 0,
			parent_id TEXT,
			likes_count INTEGER DEFAULT 0,
			is_edited BOOLEAN DEFAULT 0,
			is_deleted BOOLEAN DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (manga_id) REFERENCES manga(id) ON DELETE CASCADE,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY (parent_id) REFERENCES comments(id) ON DELETE SET NULL
		)`,

		// Comment likes tracking (prevent duplicate likes)
		`CREATE TABLE IF NOT EXISTS comment_likes (
			id TEXT PRIMARY KEY,
			comment_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (comment_id) REFERENCES comments(id) ON DELETE CASCADE,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			UNIQUE(comment_id, user_id)
		)`,

		// User activities for activity feed
		// Tracks all user actions: read, rate, comment, add_library, etc.
		`CREATE TABLE IF NOT EXISTS activities (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			action_type TEXT NOT NULL,
			manga_id TEXT,
			chapter_number INTEGER,
			details TEXT,
			is_public BOOLEAN DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY (manga_id) REFERENCES manga(id) ON DELETE SET NULL
		)`,

		// Activity feed for user actions (chapter reads, ratings, completions)
		`CREATE TABLE IF NOT EXISTS activity_feed (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			username TEXT NOT NULL,
			activity_type TEXT NOT NULL,
			manga_id TEXT NOT NULL,
			manga_title TEXT NOT NULL,
			chapter_number INTEGER,
			rating REAL,
			comment_text TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY (manga_id) REFERENCES manga(id) ON DELETE CASCADE
		)`,

		// Indexes for new tables
		`CREATE INDEX IF NOT EXISTS idx_chat_messages_room ON chat_messages(room_id)`,
		`CREATE INDEX IF NOT EXISTS idx_chat_messages_user ON chat_messages(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_chat_messages_created ON chat_messages(created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_chat_rooms_type ON chat_rooms(room_type)`,
		`CREATE INDEX IF NOT EXISTS idx_chat_rooms_manga ON chat_rooms(manga_id)`,
		`CREATE INDEX IF NOT EXISTS idx_room_members_room ON chat_room_members(room_id)`,
		`CREATE INDEX IF NOT EXISTS idx_room_members_user ON chat_room_members(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_ratings_manga ON manga_ratings(manga_id)`,
		`CREATE INDEX IF NOT EXISTS idx_ratings_user ON manga_ratings(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_ratings_overall ON manga_ratings(overall_rating DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_achievements_category ON achievements(category)`,
		`CREATE INDEX IF NOT EXISTS idx_user_achievements_user ON user_achievements(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_user_achievements_unlocked ON user_achievements(unlocked)`,
		`CREATE INDEX IF NOT EXISTS idx_external_ids_manga ON manga_external_ids(manga_id)`,
		`CREATE INDEX IF NOT EXISTS idx_external_ids_mangadex ON manga_external_ids(mangadex_id)`,
		`CREATE INDEX IF NOT EXISTS idx_external_ids_mal ON manga_external_ids(mal_id)`,

		// Phase 2 indexes
		`CREATE INDEX IF NOT EXISTS idx_comments_manga ON comments(manga_id)`,
		`CREATE INDEX IF NOT EXISTS idx_comments_chapter ON comments(manga_id, chapter_number)`,
		`CREATE INDEX IF NOT EXISTS idx_comments_user ON comments(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_comments_parent ON comments(parent_id)`,
		`CREATE INDEX IF NOT EXISTS idx_comments_created ON comments(created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_comment_likes_comment ON comment_likes(comment_id)`,
		`CREATE INDEX IF NOT EXISTS idx_comment_likes_user ON comment_likes(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_activities_user ON activities(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_activities_manga ON activities(manga_id)`,
		`CREATE INDEX IF NOT EXISTS idx_activities_type ON activities(action_type)`,
		`CREATE INDEX IF NOT EXISTS idx_activities_created ON activities(created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_activity_feed_user ON activity_feed(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_activity_feed_manga ON activity_feed(manga_id)`,
		`CREATE INDEX IF NOT EXISTS idx_activity_feed_created ON activity_feed(created_at DESC)`,

		// ===== Phase 3 New Tables =====

		// Daily reading statistics for streak and heatmap tracking
		`CREATE TABLE IF NOT EXISTS daily_stats (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			date DATE NOT NULL,
			chapters_read INTEGER DEFAULT 0,
			pages_read INTEGER DEFAULT 0,
			time_minutes INTEGER DEFAULT 0,
			manga_count INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			UNIQUE(user_id, date)
		)`,

		// Chapter reading history for detailed analytics
		`CREATE TABLE IF NOT EXISTS chapter_history (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			manga_id TEXT NOT NULL,
			manga_title TEXT,
			chapter_number INTEGER NOT NULL,
			pages_read INTEGER DEFAULT 0,
			time_minutes INTEGER DEFAULT 0,
			read_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY (manga_id) REFERENCES manga(id) ON DELETE CASCADE
		)`,

		// Custom manga lists
		`CREATE TABLE IF NOT EXISTS custom_lists (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			icon_emoji TEXT,
			is_public BOOLEAN DEFAULT 0,
			is_default BOOLEAN DEFAULT 0,
			sort_order INTEGER DEFAULT 0,
			manga_count INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,

		// Custom list items (manga in lists)
		`CREATE TABLE IF NOT EXISTS custom_list_items (
			id TEXT PRIMARY KEY,
			list_id TEXT NOT NULL,
			manga_id TEXT NOT NULL,
			sort_order INTEGER DEFAULT 0,
			notes TEXT,
			added_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (list_id) REFERENCES custom_lists(id) ON DELETE CASCADE,
			FOREIGN KEY (manga_id) REFERENCES manga(id) ON DELETE CASCADE,
			UNIQUE(list_id, manga_id)
		)`,

		// User preferences
		`CREATE TABLE IF NOT EXISTS user_preferences (
			user_id TEXT PRIMARY KEY,
			theme TEXT DEFAULT 'dracula',
			language TEXT DEFAULT 'en',
			chapters_per_page INTEGER DEFAULT 20,
			reading_direction TEXT DEFAULT 'ltr',
			default_status TEXT DEFAULT 'reading',
			show_spoilers BOOLEAN DEFAULT 0,
			auto_sync BOOLEAN DEFAULT 1,
			notifications_enabled BOOLEAN DEFAULT 1,
			email_notifications BOOLEAN DEFAULT 0,
			activity_public BOOLEAN DEFAULT 1,
			library_public BOOLEAN DEFAULT 1,
			keybindings TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,

		// Phase 3 indexes
		`CREATE INDEX IF NOT EXISTS idx_daily_stats_user ON daily_stats(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_daily_stats_date ON daily_stats(date DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_daily_stats_user_date ON daily_stats(user_id, date DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_chapter_history_user ON chapter_history(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_chapter_history_manga ON chapter_history(manga_id)`,
		`CREATE INDEX IF NOT EXISTS idx_chapter_history_read_at ON chapter_history(read_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_custom_lists_user ON custom_lists(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_custom_lists_public ON custom_lists(is_public)`,
		`CREATE INDEX IF NOT EXISTS idx_custom_list_items_list ON custom_list_items(list_id)`,
		`CREATE INDEX IF NOT EXISTS idx_custom_list_items_manga ON custom_list_items(manga_id)`,
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
