// Package comment - Comment Service Tests
// Unit tests cho comment service
package comment

import (
	"context"
	"database/sql"
	"testing"

	"mangahub/internal/testutil"
	"mangahub/pkg/models"
)

// setupTestDB returns a database with the production schema and test data.
// It used ":memory:", where every pooled connection gets its own empty
// database, so a second connection would have seen no tables;
// testutil.NewDB is file-backed.
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	sqlDB := testutil.NewDB(t).DB

	// Insert test data
	mustExec(t, sqlDB, `INSERT INTO users (id, username, email, password_hash, display_name) VALUES ('user1', 'testuser', 'test@test.com', 'hash123', 'Test User')`)
	mustExec(t, sqlDB, `INSERT INTO users (id, username, email, password_hash, display_name) VALUES ('user2', 'testuser2', 'test2@test.com', 'hash456', 'Test User 2')`)
	mustExec(t, sqlDB, `INSERT INTO manga (id, title, author) VALUES ('manga1', 'Test Manga', 'Test Author')`)

	return sqlDB
}

func mustExec(t *testing.T, db *sql.DB, query string, args ...interface{}) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatalf("failed to exec %q: %v", query, err)
	}
}

func TestCommentRepository_Create(t *testing.T) {
	db := setupTestDB(t)

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

	repo := NewRepository(db)
	ctx := context.Background()

	comment, err := repo.Create(ctx, "user1", "manga1", models.CreateCommentRequest{Content: "Original content"})
	if err != nil {
		t.Fatal(err)
	}

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

	repo := NewRepository(db)
	ctx := context.Background()

	comment, err := repo.Create(ctx, "user1", "manga1", models.CreateCommentRequest{Content: "To be deleted"})
	if err != nil {
		t.Fatal(err)
	}

	if err := repo.Delete(ctx, comment.ID, "user1"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Soft deletion: the row stays, flagged (this check used to pass even if
	// the row was gone or never flagged, as long as GetByID returned nil)
	var isDeleted bool
	if err := db.QueryRow(`SELECT is_deleted FROM comments WHERE id = ?`, comment.ID).Scan(&isDeleted); err != nil {
		t.Fatalf("comment row after delete: %v (want it kept, soft-deleted)", err)
	}
	if !isDeleted {
		t.Error("expected is_deleted = 1 after Delete")
	}
	if got, _ := repo.GetByManga(ctx, "manga1", nil, 10, 0); len(got) != 0 {
		t.Errorf("deleted comment still listed: %+v", got)
	}
}

func TestCommentRepository_Like(t *testing.T) {
	db := setupTestDB(t)

	repo := NewRepository(db)
	ctx := context.Background()

	comment, err := repo.Create(ctx, "user1", "manga1", models.CreateCommentRequest{Content: "Likeable comment"})
	if err != nil {
		t.Fatal(err)
	}
	likes := func() int {
		t.Helper()
		c, err := repo.GetByID(ctx, comment.ID)
		if err != nil || c == nil {
			t.Fatalf("GetByID: %v, %v", c, err)
		}
		return c.LikesCount
	}

	if err := repo.Like(ctx, comment.ID, "user2"); err != nil {
		t.Fatalf("Like failed: %v", err)
	}
	if n := likes(); n != 1 {
		t.Errorf("expected likes_count 1, got %d", n)
	}

	if err := repo.Unlike(ctx, comment.ID, "user2"); err != nil {
		t.Fatalf("Unlike failed: %v", err)
	}
	if n := likes(); n != 0 {
		t.Errorf("expected likes_count 0, got %d", n)
	}
}

func TestCommentService_Create(t *testing.T) {
	db := setupTestDB(t)

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
