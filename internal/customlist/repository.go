// Package customlist - Custom Lists Repository
// Handles database operations for user-created manga lists
package customlist

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"mangahub/internal/manga"
	"mangahub/pkg/models"

	"github.com/google/uuid"
)

// ErrNotFound is returned when a list, or a manga in a list, does not exist.
var ErrNotFound = errors.New("not found")

// Repository handles custom list database operations
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new custom list repository
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

const listColumns = `l.id, l.user_id, l.name, COALESCE(l.description, ''), l.is_public, l.sort_order,
	l.created_at, l.updated_at,
	(SELECT COUNT(*) FROM custom_list_items i WHERE i.list_id = l.id)`

func scanList(row interface{ Scan(...interface{}) error }) (*models.CustomList, error) {
	var list models.CustomList
	err := row.Scan(&list.ID, &list.UserID, &list.Name, &list.Description, &list.IsPublic,
		&list.SortOrder, &list.CreatedAt, &list.UpdatedAt, &list.ItemCount)
	return &list, err
}

// Create inserts a new list
func (r *Repository) Create(ctx context.Context, list *models.CustomList) error {
	list.ID = uuid.New().String()
	list.CreatedAt = time.Now()
	list.UpdatedAt = list.CreatedAt

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO custom_lists (id, user_id, name, description, is_public, sort_order, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		list.ID, list.UserID, list.Name, list.Description, list.IsPublic, list.SortOrder,
		list.CreatedAt, list.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create list: %w", err)
	}
	return nil
}

// Get retrieves a list by ID (ErrNotFound if it doesn't exist)
func (r *Repository) Get(ctx context.Context, id string) (*models.CustomList, error) {
	list, err := scanList(r.db.QueryRowContext(ctx, `SELECT `+listColumns+` FROM custom_lists l WHERE l.id = ?`, id))
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get list: %w", err)
	}
	return list, nil
}

// ListByUser returns a user's lists, optionally only the public ones
func (r *Repository) ListByUser(ctx context.Context, userID string, publicOnly bool) ([]models.CustomList, error) {
	query := `SELECT ` + listColumns + ` FROM custom_lists l WHERE l.user_id = ?`
	if publicOnly {
		query += ` AND l.is_public = 1`
	}
	rows, err := r.db.QueryContext(ctx, query+` ORDER BY l.sort_order ASC, l.name ASC`, userID)
	if err != nil {
		return nil, fmt.Errorf("query lists: %w", err)
	}
	defer rows.Close()

	lists := []models.CustomList{}
	for rows.Next() {
		list, err := scanList(rows)
		if err != nil {
			return nil, fmt.Errorf("scan list: %w", err)
		}
		lists = append(lists, *list)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate lists: %w", err)
	}
	return lists, nil
}

// Update saves a list's name, description and visibility
func (r *Repository) Update(ctx context.Context, list *models.CustomList) error {
	list.UpdatedAt = time.Now()
	res, err := r.db.ExecContext(ctx, `
		UPDATE custom_lists SET name = ?, description = ?, is_public = ?, updated_at = ?
		WHERE id = ?`,
		list.Name, list.Description, list.IsPublic, list.UpdatedAt, list.ID,
	)
	if err != nil {
		return fmt.Errorf("update list: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete removes a list and (by cascade) its items
func (r *Repository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM custom_lists WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete list: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// AddItem adds a manga to the end of a list. Adding a manga that is already
// in the list only updates its notes.
func (r *Repository) AddItem(ctx context.Context, listID, mangaID, notes string) (*models.CustomListItem, error) {
	now := time.Now()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO custom_list_items (id, list_id, manga_id, notes, sort_order, added_at)
		VALUES (?, ?, ?, ?, (SELECT COALESCE(MAX(sort_order), -1) + 1 FROM custom_list_items WHERE list_id = ?), ?)
		ON CONFLICT(list_id, manga_id) DO UPDATE SET notes = excluded.notes`,
		uuid.New().String(), listID, mangaID, notes, listID, now,
	)
	if err != nil {
		return nil, fmt.Errorf("add list item: %w", err)
	}
	r.touch(ctx, listID, now)

	var item models.CustomListItem
	var itemNotes sql.NullString
	err = r.db.QueryRowContext(ctx, `
		SELECT id, list_id, manga_id, notes, sort_order, added_at
		FROM custom_list_items WHERE list_id = ? AND manga_id = ?`, listID, mangaID,
	).Scan(&item.ID, &item.ListID, &item.MangaID, &itemNotes, &item.SortOrder, &item.AddedAt)
	if err != nil {
		return nil, fmt.Errorf("load list item: %w", err)
	}
	item.Notes = itemNotes.String
	return &item, nil
}

// RemoveItem removes a manga from a list (ErrNotFound if it isn't in it)
func (r *Repository) RemoveItem(ctx context.Context, listID, mangaID string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM custom_list_items WHERE list_id = ? AND manga_id = ?`, listID, mangaID)
	if err != nil {
		return fmt.Errorf("remove list item: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	r.touch(ctx, listID, time.Now())
	return nil
}

// Items returns a list's manga in list order, with genres
func (r *Repository) Items(ctx context.Context, listID string) ([]models.CustomListWithManga, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			cli.id, cli.list_id, cli.manga_id, COALESCE(cli.notes, ''), cli.sort_order, cli.added_at,
			m.id, m.title, COALESCE(m.author, ''), COALESCE(m.artist, ''), COALESCE(m.description, ''),
			COALESCE(m.cover_url, ''), m.status, m.type,
			m.total_chapters, m.average_rating, m.rating_count, COALESCE(m.year, 0), m.created_at, m.updated_at
		FROM custom_list_items cli
		JOIN manga m ON cli.manga_id = m.id
		WHERE cli.list_id = ?
		ORDER BY cli.sort_order ASC, cli.rowid ASC`, listID)
	if err != nil {
		return nil, fmt.Errorf("query list items: %w", err)
	}
	defer rows.Close()

	items := []models.CustomListWithManga{}
	for rows.Next() {
		var item models.CustomListWithManga
		if err := rows.Scan(
			&item.ID, &item.ListID, &item.MangaID, &item.Notes, &item.SortOrder, &item.AddedAt,
			&item.Manga.ID, &item.Manga.Title, &item.Manga.Author, &item.Manga.Artist,
			&item.Manga.Description, &item.Manga.CoverURL, &item.Manga.Status, &item.Manga.Type,
			&item.Manga.TotalChapters, &item.Manga.AverageRating, &item.Manga.RatingCount, &item.Manga.Year,
			&item.Manga.CreatedAt, &item.Manga.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan list item: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate list items: %w", err)
	}
	rows.Close() // release the connection before the genre query

	mangaList := make([]models.Manga, len(items))
	for i := range items {
		mangaList[i] = items[i].Manga
	}
	if err := manga.AttachGenres(ctx, r.db, mangaList); err != nil {
		return nil, err
	}
	for i := range items {
		items[i].Manga.Genres = mangaList[i].Genres
	}
	return items, nil
}

// Reorder sets the order of a list's items. itemIDs must contain exactly the
// list's item IDs; otherwise nothing changes and false is returned.
func (r *Repository) Reorder(ctx context.Context, listID string, itemIDs []string) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin reorder: %w", err)
	}
	defer tx.Rollback()

	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM custom_list_items WHERE list_id = ?`, listID).Scan(&count); err != nil {
		return false, fmt.Errorf("count list items: %w", err)
	}
	seen := make(map[string]bool, len(itemIDs))
	for _, id := range itemIDs {
		seen[id] = true
	}
	if len(itemIDs) != count || len(seen) != count {
		return false, nil
	}

	for i, id := range itemIDs {
		res, err := tx.ExecContext(ctx, `UPDATE custom_list_items SET sort_order = ? WHERE id = ? AND list_id = ?`, i, id, listID)
		if err != nil {
			return false, fmt.Errorf("reorder: %w", err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return false, nil // an ID from another list
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE custom_lists SET updated_at = ? WHERE id = ?`, time.Now(), listID); err != nil {
		return false, fmt.Errorf("reorder: %w", err)
	}
	return true, tx.Commit()
}

// MangaExists reports whether the manga exists
func (r *Repository) MangaExists(ctx context.Context, mangaID string) (bool, error) {
	var one int
	err := r.db.QueryRowContext(ctx, "SELECT 1 FROM manga WHERE id = ?", mangaID).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

// touch bumps a list's updated_at after its items change
func (r *Repository) touch(ctx context.Context, listID string, at time.Time) {
	_, _ = r.db.ExecContext(ctx, `UPDATE custom_lists SET updated_at = ? WHERE id = ?`, at, listID)
}
