// Package chapters - New Chapter Releases
// Ghi nhận chapter mới và báo cho đúng những người đang đọc manga đó
// Chức năng:
//   - Release: record that a new chapter is out and notify the manga's readers
//   - Syncer: check an external source (MangaDex) for new chapters
//   - Admin HTTP handler to release a chapter by hand
//
// Readers are the users with the manga in their library, except those who
// dropped it. Notifications go through a Notifier: the protocol bridge in the
// API server (targeted UDP + chat room), or a UDP broadcaster elsewhere.
package chapters

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"mangahub/pkg/models"
)

// Notifier delivers chapter release notifications to readers.
type Notifier interface {
	NotifyChapterRelease(ctx context.Context, mangaID, mangaTitle string, chapter int, userIDs []string) error
}

// Release describes a chapter that was just released.
type Release struct {
	MangaID         string `json:"manga_id"`
	Title           string `json:"title"`
	PreviousLatest  int    `json:"previous_latest"`
	Chapter         int    `json:"chapter"`
	NotifiedReaders int    `json:"notified_readers"`
}

// Service records chapter releases.
type Service struct {
	db       *sql.DB
	notifier Notifier
}

// NewService creates a release service; notifier may be nil (no notifications).
func NewService(db *sql.DB, notifier Notifier) *Service {
	return &Service{db: db, notifier: notifier}
}

// Release records that chapter is out for the manga (raising total_chapters)
// and notifies its readers. It fails with 404 for an unknown manga and 409 if
// the chapter isn't newer than what is already known.
func (s *Service) Release(ctx context.Context, mangaID string, chapter int) (*Release, error) {
	if chapter <= 0 {
		return nil, models.NewAppError(models.ErrCodeValidation, "chapter must be positive", 400, nil)
	}

	rel := &Release{MangaID: mangaID, Chapter: chapter}
	err := s.db.QueryRowContext(ctx, `SELECT title, total_chapters FROM manga WHERE id = ?`, mangaID).
		Scan(&rel.Title, &rel.PreviousLatest)
	if err == sql.ErrNoRows {
		return nil, models.NewAppError(models.ErrCodeNotFound, "manga not found", 404, models.ErrMangaNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("load manga: %w", err)
	}

	// Conditional, so two concurrent releases of the same chapter notify once
	res, err := s.db.ExecContext(ctx, `
		UPDATE manga SET total_chapters = ?, updated_at = ?
		WHERE id = ? AND total_chapters < ?`, chapter, time.Now(), mangaID, chapter)
	if err != nil {
		return nil, fmt.Errorf("update total_chapters: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, models.NewAppError(models.ErrCodeConflict,
			fmt.Sprintf("chapter %d is not newer than the latest known chapter", chapter), 409, nil)
	}

	readers, err := s.Readers(ctx, mangaID)
	if err != nil {
		return nil, err
	}
	rel.NotifiedReaders = len(readers)
	if s.notifier != nil && len(readers) > 0 {
		if err := s.notifier.NotifyChapterRelease(ctx, mangaID, rel.Title, chapter, readers); err != nil {
			// The release is recorded either way; a lost notification isn't fatal
			return rel, fmt.Errorf("chapter %d recorded, but notifying readers failed: %w", chapter, err)
		}
	}
	return rel, nil
}

// Readers returns the IDs of users with the manga in their library who
// haven't dropped it.
func (s *Service) Readers(ctx context.Context, mangaID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT user_id FROM reading_progress WHERE manga_id = ? AND status != 'dropped'`, mangaID)
	if err != nil {
		return nil, fmt.Errorf("load readers: %w", err)
	}
	defer rows.Close()

	readers := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan reader: %w", err)
		}
		readers = append(readers, id)
	}
	return readers, rows.Err()
}
