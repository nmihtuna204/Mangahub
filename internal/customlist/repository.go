// Package customlist - Custom Lists Repository
// Handles database operations for user-created manga lists
package customlist

import (
	"database/sql"
	"fmt"
	"time"

	"mangahub/pkg/database"
	"mangahub/pkg/models"

	"github.com/google/uuid"
)

// Repository handles custom list database operations
type Repository struct {
	db *database.DB
}

// NewRepository creates a new custom list repository
func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

// CreateList creates a new custom list
func (r *Repository) CreateList(list *models.CustomList) error {
	if list.ID == "" {
		list.ID = uuid.New().String()
	}
	list.CreatedAt = time.Now()
	list.UpdatedAt = time.Now()

	query := `
		INSERT INTO custom_lists (id, user_id, name, description, icon_emoji, is_public, is_default, sort_order, manga_count, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?)`

	_, err := r.db.Exec(query,
		list.ID, list.UserID, list.Name, list.Description,
		list.IconEmoji, list.IsPublic, list.IsDefault, list.SortOrder,
		list.CreatedAt, list.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create list: %w", err)
	}
	return nil
}

// GetList retrieves a list by ID
func (r *Repository) GetList(id string) (*models.CustomList, error) {
	query := `
		SELECT id, user_id, name, description, icon_emoji, is_public, is_default, sort_order, manga_count, created_at, updated_at
		FROM custom_lists WHERE id = ?`

	var list models.CustomList
	var description, iconEmoji sql.NullString
	err := r.db.QueryRow(query, id).Scan(
		&list.ID, &list.UserID, &list.Name, &description, &iconEmoji,
		&list.IsPublic, &list.IsDefault, &list.SortOrder, &list.MangaCount,
		&list.CreatedAt, &list.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get list: %w", err)
	}

	if description.Valid {
		list.Description = description.String
	}
	if iconEmoji.Valid {
		list.IconEmoji = iconEmoji.String
	}

	return &list, nil
}

// GetUserLists retrieves all lists for a user
func (r *Repository) GetUserLists(userID string) ([]models.CustomList, error) {
	query := `
		SELECT id, user_id, name, description, icon_emoji, is_public, is_default, sort_order, manga_count, created_at, updated_at
		FROM custom_lists 
		WHERE user_id = ?
		ORDER BY is_default DESC, sort_order ASC, name ASC`

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query lists: %w", err)
	}
	defer rows.Close()

	var lists []models.CustomList
	for rows.Next() {
		var list models.CustomList
		var description, iconEmoji sql.NullString
		err := rows.Scan(
			&list.ID, &list.UserID, &list.Name, &description, &iconEmoji,
			&list.IsPublic, &list.IsDefault, &list.SortOrder, &list.MangaCount,
			&list.CreatedAt, &list.UpdatedAt,
		)
		if err != nil {
			continue
		}
		if description.Valid {
			list.Description = description.String
		}
		if iconEmoji.Valid {
			list.IconEmoji = iconEmoji.String
		}
		lists = append(lists, list)
	}

	return lists, nil
}

// UpdateList updates a custom list
func (r *Repository) UpdateList(list *models.CustomList) error {
	list.UpdatedAt = time.Now()

	query := `
		UPDATE custom_lists 
		SET name = ?, description = ?, icon_emoji = ?, is_public = ?, sort_order = ?, updated_at = ?
		WHERE id = ? AND user_id = ? AND is_default = 0`

	result, err := r.db.Exec(query,
		list.Name, list.Description, list.IconEmoji, list.IsPublic, list.SortOrder,
		list.UpdatedAt, list.ID, list.UserID,
	)
	if err != nil {
		return fmt.Errorf("failed to update list: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("list not found or is a default list")
	}

	return nil
}

// DeleteList deletes a custom list (not default lists)
func (r *Repository) DeleteList(id, userID string) error {
	result, err := r.db.Exec(`
		DELETE FROM custom_lists 
		WHERE id = ? AND user_id = ? AND is_default = 0`, id, userID)
	if err != nil {
		return fmt.Errorf("failed to delete list: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("list not found or is a default list")
	}

	return nil
}

// AddMangaToList adds a manga to a list
func (r *Repository) AddMangaToList(listID, mangaID, userID, notes string) error {
	// Verify list ownership
	var ownerID string
	err := r.db.QueryRow(`SELECT user_id FROM custom_lists WHERE id = ?`, listID).Scan(&ownerID)
	if err != nil {
		return fmt.Errorf("list not found")
	}
	if ownerID != userID {
		return fmt.Errorf("unauthorized")
	}

	// Get max sort order
	var maxOrder int
	r.db.QueryRow(`SELECT COALESCE(MAX(sort_order), 0) FROM custom_list_items WHERE list_id = ?`, listID).Scan(&maxOrder)

	id := uuid.New().String()
	now := time.Now()

	_, err = r.db.Exec(`
		INSERT INTO custom_list_items (id, list_id, manga_id, sort_order, notes, added_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(list_id, manga_id) DO UPDATE SET notes = ?, sort_order = ?`,
		id, listID, mangaID, maxOrder+1, notes, now, now, notes, maxOrder+1)
	if err != nil {
		return fmt.Errorf("failed to add manga: %w", err)
	}

	// Update manga count
	_, err = r.db.Exec(`
		UPDATE custom_lists SET manga_count = (SELECT COUNT(*) FROM custom_list_items WHERE list_id = ?), updated_at = ?
		WHERE id = ?`, listID, now, listID)

	return err
}

// RemoveMangaFromList removes a manga from a list
func (r *Repository) RemoveMangaFromList(listID, mangaID, userID string) error {
	// Verify list ownership
	var ownerID string
	err := r.db.QueryRow(`SELECT user_id FROM custom_lists WHERE id = ?`, listID).Scan(&ownerID)
	if err != nil {
		return fmt.Errorf("list not found")
	}
	if ownerID != userID {
		return fmt.Errorf("unauthorized")
	}

	_, err = r.db.Exec(`DELETE FROM custom_list_items WHERE list_id = ? AND manga_id = ?`, listID, mangaID)
	if err != nil {
		return fmt.Errorf("failed to remove manga: %w", err)
	}

	// Update manga count
	now := time.Now()
	_, err = r.db.Exec(`
		UPDATE custom_lists SET manga_count = (SELECT COUNT(*) FROM custom_list_items WHERE list_id = ?), updated_at = ?
		WHERE id = ?`, listID, now, listID)

	return err
}

// GetListItems retrieves all manga in a list with details
func (r *Repository) GetListItems(listID string) ([]models.CustomListWithManga, error) {
	query := `
		SELECT 
			cli.id, cli.list_id, cli.manga_id, cli.sort_order, cli.notes, cli.added_at, cli.created_at,
			m.id, m.title, m.author, m.artist, m.description, m.cover_url, m.status, m.type,
			m.genres, m.total_chapters, m.rating, m.year, m.created_at, m.updated_at
		FROM custom_list_items cli
		JOIN manga m ON cli.manga_id = m.id
		WHERE cli.list_id = ?
		ORDER BY cli.sort_order ASC`

	rows, err := r.db.Query(query, listID)
	if err != nil {
		return nil, fmt.Errorf("failed to query list items: %w", err)
	}
	defer rows.Close()

	var items []models.CustomListWithManga
	for rows.Next() {
		var item models.CustomListWithManga
		var notes sql.NullString
		var genres sql.NullString

		err := rows.Scan(
			&item.ID, &item.ListID, &item.MangaID, &item.SortOrder, &notes, &item.AddedAt, &item.CreatedAt,
			&item.Manga.ID, &item.Manga.Title, &item.Manga.Author, &item.Manga.Artist,
			&item.Manga.Description, &item.Manga.CoverURL, &item.Manga.Status, &item.Manga.Type,
			&genres, &item.Manga.TotalChapters, &item.Manga.Rating, &item.Manga.Year,
			&item.Manga.CreatedAt, &item.Manga.UpdatedAt,
		)
		if err != nil {
			continue
		}

		if notes.Valid {
			item.Notes = notes.String
		}
		if genres.Valid {
			item.Manga.GenresJSON = genres.String
		}

		items = append(items, item)
	}

	return items, nil
}

// GetListWithItems retrieves a list with all its items
func (r *Repository) GetListWithItems(listID string) (*models.CustomListWithItems, error) {
	list, err := r.GetList(listID)
	if err != nil || list == nil {
		return nil, err
	}

	items, err := r.GetListItems(listID)
	if err != nil {
		return nil, err
	}

	return &models.CustomListWithItems{
		CustomList: *list,
		Items:      items,
	}, nil
}

// ReorderListItems reorders items in a list
func (r *Repository) ReorderListItems(listID, userID string, itemIDs []string) error {
	// Verify list ownership
	var ownerID string
	err := r.db.QueryRow(`SELECT user_id FROM custom_lists WHERE id = ?`, listID).Scan(&ownerID)
	if err != nil {
		return fmt.Errorf("list not found")
	}
	if ownerID != userID {
		return fmt.Errorf("unauthorized")
	}

	// Update sort orders
	for i, itemID := range itemIDs {
		_, err := r.db.Exec(`
			UPDATE custom_list_items SET sort_order = ? WHERE id = ? AND list_id = ?`,
			i, itemID, listID)
		if err != nil {
			return fmt.Errorf("failed to update order: %w", err)
		}
	}

	return nil
}

// EnsureDefaultLists creates default lists for a user if they don't exist
func (r *Repository) EnsureDefaultLists(userID string) error {
	// Check if default lists exist
	var count int
	r.db.QueryRow(`SELECT COUNT(*) FROM custom_lists WHERE user_id = ? AND is_default = 1`, userID).Scan(&count)
	if count > 0 {
		return nil // Already have default lists
	}

	defaultLists := []struct {
		Name      string
		Emoji     string
		SortOrder int
	}{
		{"Favorites", "❤️", 0},
		{"Plan to Read", "📋", 1},
		{"Top 10", "🏆", 2},
	}

	for _, dl := range defaultLists {
		list := &models.CustomList{
			UserID:    userID,
			Name:      dl.Name,
			IconEmoji: dl.Emoji,
			IsDefault: true,
			IsPublic:  false,
			SortOrder: dl.SortOrder,
		}
		if err := r.CreateList(list); err != nil {
			return err
		}
	}

	return nil
}
