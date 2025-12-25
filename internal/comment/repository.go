// Package comment - Comment Repository
// Data access layer cho comment system
// Chức năng:
//   - CRUD operations for comments
//   - Threaded replies support
//   - Like/unlike comments
//   - Pagination for comment lists
package comment

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"mangahub/pkg/models"
)

// Repository defines data access operations for comments
type Repository interface {
	// Create creates a new comment
	Create(ctx context.Context, userID, mangaID string, req models.CreateCommentRequest) (*models.Comment, error)

	// GetByID retrieves a comment by ID
	GetByID(ctx context.Context, id string) (*models.Comment, error)

	// GetByManga retrieves comments for a manga with optional chapter filter
	GetByManga(ctx context.Context, mangaID string, chapterNumber *int, limit, offset int) ([]models.CommentWithUser, error)

	// GetReplies retrieves replies for a comment
	GetReplies(ctx context.Context, parentID string) ([]models.CommentWithUser, error)

	// CountByManga counts total comments for a manga/chapter
	CountByManga(ctx context.Context, mangaID string, chapterNumber *int) (int, error)

	// Update updates a comment's content
	Update(ctx context.Context, id, userID string, req models.UpdateCommentRequest) (*models.Comment, error)

	// Delete soft-deletes a comment (sets is_deleted = true)
	Delete(ctx context.Context, id, userID string) error

	// Like adds a like to a comment
	Like(ctx context.Context, commentID, userID string) error

	// Unlike removes a like from a comment
	Unlike(ctx context.Context, commentID, userID string) error

	// HasLiked checks if a user has liked a comment
	HasLiked(ctx context.Context, commentID, userID string) (bool, error)
}

type repository struct {
	db *sql.DB
}

// NewRepository creates a new comment repository
func NewRepository(db *sql.DB) Repository {
	return &repository{db: db}
}

// Create creates a new comment
func (r *repository) Create(ctx context.Context, userID, mangaID string, req models.CreateCommentRequest) (*models.Comment, error) {
	now := time.Now()
	id := uuid.New().String()

	// Handle optional parent_id for replies
	var parentID interface{}
	if req.ParentID != "" {
		parentID = req.ParentID
	} else {
		parentID = nil
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO comments 
		(id, manga_id, chapter_number, user_id, content, is_spoiler, parent_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, mangaID, req.ChapterNumber, userID, req.Content, req.IsSpoiler, parentID, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert comment: %w", err)
	}

	return r.GetByID(ctx, id)
}

// GetByID retrieves a comment by ID
func (r *repository) GetByID(ctx context.Context, id string) (*models.Comment, error) {
	var c models.Comment
	var chapterNum sql.NullInt64
	var parentIDStr sql.NullString

	err := r.db.QueryRowContext(ctx, `
		SELECT id, manga_id, chapter_number, user_id, content, is_spoiler, 
		       parent_id, likes_count, is_edited, is_deleted, created_at, updated_at
		FROM comments WHERE id = ?`, id,
	).Scan(
		&c.ID, &c.MangaID, &chapterNum, &c.UserID, &c.Content, &c.IsSpoiler,
		&parentIDStr, &c.LikesCount, &c.IsEdited, &c.IsDeleted, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get comment: %w", err)
	}

	if chapterNum.Valid {
		ch := int(chapterNum.Int64)
		c.ChapterNumber = &ch
	}
	if parentIDStr.Valid {
		c.ParentID = &parentIDStr.String
	}

	return &c, nil
}

// GetByManga retrieves top-level comments for a manga (optionally filtered by chapter)
func (r *repository) GetByManga(ctx context.Context, mangaID string, chapterNumber *int, limit, offset int) ([]models.CommentWithUser, error) {
	var query string
	var args []interface{}

	// Build query based on whether chapter filter is provided
	if chapterNumber != nil {
		query = `
			SELECT c.id, c.manga_id, c.chapter_number, c.user_id, c.content, c.is_spoiler,
			       c.parent_id, c.likes_count, c.is_edited, c.is_deleted, c.created_at, c.updated_at,
			       u.username, u.display_name
			FROM comments c
			JOIN users u ON c.user_id = u.id
			WHERE c.manga_id = ? AND c.chapter_number = ? AND c.parent_id IS NULL AND c.is_deleted = 0
			ORDER BY c.created_at DESC
			LIMIT ? OFFSET ?`
		args = []interface{}{mangaID, *chapterNumber, limit, offset}
	} else {
		// Get manga-level comments (where chapter_number is NULL)
		query = `
			SELECT c.id, c.manga_id, c.chapter_number, c.user_id, c.content, c.is_spoiler,
			       c.parent_id, c.likes_count, c.is_edited, c.is_deleted, c.created_at, c.updated_at,
			       u.username, u.display_name
			FROM comments c
			JOIN users u ON c.user_id = u.id
			WHERE c.manga_id = ? AND c.chapter_number IS NULL AND c.parent_id IS NULL AND c.is_deleted = 0
			ORDER BY c.created_at DESC
			LIMIT ? OFFSET ?`
		args = []interface{}{mangaID, limit, offset}
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get comments: %w", err)
	}
	defer rows.Close()

	return r.scanComments(rows)
}

// GetReplies retrieves replies for a parent comment
func (r *repository) GetReplies(ctx context.Context, parentID string) ([]models.CommentWithUser, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT c.id, c.manga_id, c.chapter_number, c.user_id, c.content, c.is_spoiler,
		       c.parent_id, c.likes_count, c.is_edited, c.is_deleted, c.created_at, c.updated_at,
		       u.username, u.display_name
		FROM comments c
		JOIN users u ON c.user_id = u.id
		WHERE c.parent_id = ? AND c.is_deleted = 0
		ORDER BY c.created_at ASC`, parentID,
	)
	if err != nil {
		return nil, fmt.Errorf("get replies: %w", err)
	}
	defer rows.Close()

	return r.scanComments(rows)
}

// scanComments is a helper to scan comment rows
func (r *repository) scanComments(rows *sql.Rows) ([]models.CommentWithUser, error) {
	var comments []models.CommentWithUser
	for rows.Next() {
		var c models.CommentWithUser
		var chapterNum sql.NullInt64
		var parentIDStr sql.NullString

		err := rows.Scan(
			&c.ID, &c.MangaID, &chapterNum, &c.UserID, &c.Content, &c.IsSpoiler,
			&parentIDStr, &c.LikesCount, &c.IsEdited, &c.IsDeleted, &c.CreatedAt, &c.UpdatedAt,
			&c.Username, &c.DisplayName,
		)
		if err != nil {
			return nil, fmt.Errorf("scan comment: %w", err)
		}

		if chapterNum.Valid {
			ch := int(chapterNum.Int64)
			c.ChapterNumber = &ch
		}
		if parentIDStr.Valid {
			c.ParentID = &parentIDStr.String
		}
		// Avatar can be generated from external service (Gravatar, etc.)
		c.AvatarURL = ""

		comments = append(comments, c)
	}
	return comments, nil
}

// CountByManga counts total comments for a manga/chapter
func (r *repository) CountByManga(ctx context.Context, mangaID string, chapterNumber *int) (int, error) {
	var query string
	var args []interface{}

	if chapterNumber != nil {
		query = "SELECT COUNT(*) FROM comments WHERE manga_id = ? AND chapter_number = ? AND is_deleted = 0"
		args = []interface{}{mangaID, *chapterNumber}
	} else {
		query = "SELECT COUNT(*) FROM comments WHERE manga_id = ? AND chapter_number IS NULL AND is_deleted = 0"
		args = []interface{}{mangaID}
	}

	var count int
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count comments: %w", err)
	}
	return count, nil
}

// Update updates a comment's content (only owner can update)
func (r *repository) Update(ctx context.Context, id, userID string, req models.UpdateCommentRequest) (*models.Comment, error) {
	now := time.Now()

	result, err := r.db.ExecContext(ctx, `
		UPDATE comments 
		SET content = ?, is_spoiler = ?, is_edited = 1, updated_at = ?
		WHERE id = ? AND user_id = ? AND is_deleted = 0`,
		req.Content, req.IsSpoiler, now, id, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("update comment: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, fmt.Errorf("comment not found or not owned by user")
	}

	return r.GetByID(ctx, id)
}

// Delete soft-deletes a comment (only owner can delete)
func (r *repository) Delete(ctx context.Context, id, userID string) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE comments 
		SET is_deleted = 1, content = '[deleted]', updated_at = ?
		WHERE id = ? AND user_id = ?`,
		time.Now(), id, userID,
	)
	if err != nil {
		return fmt.Errorf("delete comment: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("comment not found or not owned by user")
	}
	return nil
}

// Like adds a like to a comment
func (r *repository) Like(ctx context.Context, commentID, userID string) error {
	now := time.Now()
	id := uuid.New().String()

	// Use INSERT OR IGNORE to handle duplicate likes gracefully
	_, err := r.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO comment_likes (id, comment_id, user_id, created_at)
		VALUES (?, ?, ?, ?)`, id, commentID, userID, now,
	)
	if err != nil {
		return fmt.Errorf("insert like: %w", err)
	}

	// Update likes_count
	_, err = r.db.ExecContext(ctx, `
		UPDATE comments SET likes_count = (
			SELECT COUNT(*) FROM comment_likes WHERE comment_id = ?
		) WHERE id = ?`, commentID, commentID,
	)
	if err != nil {
		return fmt.Errorf("update likes count: %w", err)
	}

	return nil
}

// Unlike removes a like from a comment
func (r *repository) Unlike(ctx context.Context, commentID, userID string) error {
	result, err := r.db.ExecContext(ctx, `
		DELETE FROM comment_likes WHERE comment_id = ? AND user_id = ?`,
		commentID, userID,
	)
	if err != nil {
		return fmt.Errorf("delete like: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("like not found")
	}

	// Update likes_count
	_, err = r.db.ExecContext(ctx, `
		UPDATE comments SET likes_count = (
			SELECT COUNT(*) FROM comment_likes WHERE comment_id = ?
		) WHERE id = ?`, commentID, commentID,
	)
	if err != nil {
		return fmt.Errorf("update likes count: %w", err)
	}

	return nil
}

// HasLiked checks if a user has liked a comment
func (r *repository) HasLiked(ctx context.Context, commentID, userID string) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM comment_likes WHERE comment_id = ? AND user_id = ?`,
		commentID, userID,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check like: %w", err)
	}
	return count > 0, nil
}
