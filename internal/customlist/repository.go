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
		INSERT INTO custom_lists (id, user_id, name, description, is_public, sort_order, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := r.db.Exec(query,
		list.ID, list.UserID, list.Name, list.Description,
		list.IsPublic, list.SortOrder,
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
		SELECT id, user_id, name, description, is_public, sort_order, created_at, updated_at
		FROM custom_lists WHERE id = ?`

	var list models.CustomList
	var description sql.NullString
	err := r.db.QueryRow(query, id).Scan(
		&list.ID, &list.UserID, &list.Name, &description,
		&list.IsPublic, &list.SortOrder,
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

	return &list, nil
}

// GetUserLists retrieves all lists for a user
func (r *Repository) GetUserLists(userID string) ([]models.CustomList, error) {
	query := `
		SELECT id, user_id, name, description, is_public, sort_order, created_at, updated_at
		FROM custom_lists 
		WHERE user_id = ?
		ORDER BY sort_order ASC, name ASC`

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query lists: %w", err)
	}
	defer rows.Close()

	var lists []models.CustomList
	for rows.Next() {
		var list models.CustomList
		var description sql.NullString
		err := rows.Scan(
			&list.ID, &list.UserID, &list.Name, &description,
			&list.IsPublic, &list.SortOrder,
			&list.CreatedAt, &list.UpdatedAt,
		)
		if err != nil {
			continue
		}
		if description.Valid {
			list.Description = description.String
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
		SET name = ?, description = ?, is_public = ?, sort_order = ?, updated_at = ?
		WHERE id = ? AND user_id = ?`

	result, err := r.db.Exec(query,
		list.Name, list.Description, list.IsPublic, list.SortOrder,
		list.UpdatedAt, list.ID, list.UserID,
	)
	if err != nil {
		return fmt.Errorf("failed to update list: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("list not found")
	}

	return nil
}

	// DeleteList deletes a custom list
func (r *Repository) DeleteList(id, userID string) error {
	result, err := r.db.Exec(`
		DELETE FROM custom_lists 
		WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return fmt.Errorf("failed to delete list: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("list not found")
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

	return nil
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

	return nil
}

// GetListItems retrieves all manga in a list with details
func (r *Repository) GetListItems(listID string) ([]models.CustomListWithManga, error) {
	query := `
		SELECT 
			cli.id, cli.list_id, cli.manga_id, cli.sort_order, cli.notes, cli.added_at, cli.created_at,
			m.id, m.title, m.author, m.artist, m.description, m.cover_url, m.status, m.type,
			m.total_chapters, m.average_rating, m.rating_count, m.year, m.created_at, m.updated_at
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

		err := rows.Scan(
			&item.ID, &item.ListID, &item.MangaID, &item.SortOrder, &notes, &item.AddedAt,
			&item.Manga.ID, &item.Manga.Title, &item.Manga.Author, &item.Manga.Artist,
			&item.Manga.Description, &item.Manga.CoverURL, &item.Manga.Status, &item.Manga.Type,
			&item.Manga.TotalChapters, &item.Manga.AverageRating, &item.Manga.RatingCount, &item.Manga.Year,
			&item.Manga.CreatedAt, &item.Manga.UpdatedAt,
		)
		if err != nil {
			continue
		}

		if notes.Valid {
			item.Notes = notes.String
		}
		// Genres are loaded separately via JOIN in service layer
		item.Manga.Genres = []models.Genre{}

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

	for i, dl := range defaultLists {
		list := &models.CustomList{
			ID:          uuid.New().String(),
			UserID:      userID,
			Name:        dl.Name,
			Description: "",
			IsPublic:    false,
			SortOrder:   i,
		}
		if err := r.CreateList(list); err != nil {
			return err
		}
	}

	return nil
}
