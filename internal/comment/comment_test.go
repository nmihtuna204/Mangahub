// Package comment - Comment Service Tests
// Unit tests cho comment service
package comment

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"mangahub/pkg/models"
)

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	// Create required tables
	tables := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			username TEXT NOT NULL UNIQUE,
			email TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			display_name TEXT DEFAULT '',
			avatar_url TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS manga (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			cover_url TEXT,
			author TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
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
		`CREATE TABLE IF NOT EXISTS comment_likes (
			id TEXT PRIMARY KEY,
			comment_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(comment_id, user_id),
			FOREIGN KEY (comment_id) REFERENCES comments(id) ON DELETE CASCADE,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
	}

	for _, table := range tables {
		if _, err := db.Exec(table); err != nil {
			t.Fatalf("failed to create table: %v", err)
		}
	}

	// Insert test data
	db.Exec(`INSERT INTO users (id, username, email, password_hash, display_name) VALUES ('user1', 'testuser', 'test@test.com', 'hash123', 'Test User')`)
	db.Exec(`INSERT INTO users (id, username, email, password_hash, display_name) VALUES ('user2', 'testuser2', 'test2@test.com', 'hash456', 'Test User 2')`)
	db.Exec(`INSERT INTO manga (id, title, author) VALUES ('manga1', 'Test Manga', 'Test Author')`)

	return db
}

func TestCommentRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	// Test creating a comment
	req := models.CreateCommentRequest{
		Content:   "This is a test comment!",
		IsSpoiler: false,
	}

	comment, err := repo.Create(ctx, "user1", "manga1", req)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if comment.Content != "This is a test comment!" {
		t.Errorf("expected content 'This is a test comment!', got '%s'", comment.Content)
	}
	if comment.UserID != "user1" {
		t.Errorf("expected user_id 'user1', got '%s'", comment.UserID)
	}
	if comment.MangaID != "manga1" {
		t.Errorf("expected manga_id 'manga1', got '%s'", comment.MangaID)
	}
}

func TestCommentRepository_CreateReply(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	// Create parent comment
	parentReq := models.CreateCommentRequest{Content: "Parent comment"}
	parent, err := repo.Create(ctx, "user1", "manga1", parentReq)
	if err != nil {
		t.Fatalf("Create parent failed: %v", err)
	}

	// Create reply
	replyReq := models.CreateCommentRequest{
		Content:  "Reply to parent",
		ParentID: parent.ID,
	}
	reply, err := repo.Create(ctx, "user2", "manga1", replyReq)
	if err != nil {
		t.Fatalf("Create reply failed: %v", err)
	}

	if reply.ParentID == nil || *reply.ParentID != parent.ID {
		t.Error("expected reply to have parent_id set")
	}
}

func TestCommentRepository_GetByManga(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	// Create multiple comments
	repo.Create(ctx, "user1", "manga1", models.CreateCommentRequest{Content: "Comment 1"})
	repo.Create(ctx, "user2", "manga1", models.CreateCommentRequest{Content: "Comment 2"})

	// Get comments (no chapter filter = manga-level comments)
	comments, err := repo.GetByManga(ctx, "manga1", nil, 10, 0)
	if err != nil {
		t.Fatalf("GetByManga failed: %v", err)
	}

	if len(comments) != 2 {
		t.Errorf("expected 2 comments, got %d", len(comments))
	}
}

func TestCommentRepository_Update(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	// Create a comment
	comment, _ := repo.Create(ctx, "user1", "manga1", models.CreateCommentRequest{Content: "Original content"})

	// Update the comment
	updated, err := repo.Update(ctx, comment.ID, "user1", models.UpdateCommentRequest{Content: "Updated content"})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	if updated.Content != "Updated content" {
		t.Errorf("expected content 'Updated content', got '%s'", updated.Content)
	}
	if !updated.IsEdited {
		t.Error("expected is_edited to be true")
	}
}

func TestCommentRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	// Create a comment
	comment, _ := repo.Create(ctx, "user1", "manga1", models.CreateCommentRequest{Content: "To be deleted"})

	// Delete the comment
	err := repo.Delete(ctx, comment.ID, "user1")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify soft deletion
	deleted, _ := repo.GetByID(ctx, comment.ID)
	if deleted != nil && !deleted.IsDeleted {
		t.Error("expected comment to be soft-deleted")
	}
}

func TestCommentRepository_Like(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	// Create a comment
	comment, _ := repo.Create(ctx, "user1", "manga1", models.CreateCommentRequest{Content: "Likeable comment"})

	// Like the comment
	err := repo.Like(ctx, comment.ID, "user2")
	if err != nil {
		t.Fatalf("Like failed: %v", err)
	}

	// Verify like count
	updated, _ := repo.GetByID(ctx, comment.ID)
	if updated.LikesCount != 1 {
		t.Errorf("expected likes_count 1, got %d", updated.LikesCount)
	}

	// Unlike the comment
	err = repo.Unlike(ctx, comment.ID, "user2")
	if err != nil {
		t.Fatalf("Unlike failed: %v", err)
	}

	// Verify like count decreased
	updated, _ = repo.GetByID(ctx, comment.ID)
	if updated.LikesCount != 0 {
		t.Errorf("expected likes_count 0, got %d", updated.LikesCount)
	}
}

func TestCommentService_Create(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	svc := NewService(repo)
	ctx := context.Background()

	// Test valid comment
	req := models.CreateCommentRequest{Content: "Valid comment content"}
	comment, err := svc.Create(ctx, "user1", "manga1", req)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if comment.Content != "Valid comment content" {
		t.Errorf("expected content 'Valid comment content', got '%s'", comment.Content)
	}

	// Test empty content
	req.Content = ""
	_, err = svc.Create(ctx, "user1", "manga1", req)
	if err == nil {
		t.Error("expected error for empty content")
	}
}
