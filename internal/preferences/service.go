// Package preferences - User Preferences Service
// Handles user settings and preferences management
package preferences

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"mangahub/pkg/database"
	"mangahub/pkg/models"
)

// Service provides preferences business logic
type Service struct {
	db *database.DB
}

// NewService creates a new preferences service
func NewService(db *database.DB) *Service {
	return &Service{db: db}
}

// GetPreferences returns user preferences, creating defaults if needed
func (s *Service) GetPreferences(ctx context.Context, userID string) (*models.UserPreferences, error) {
	query := `
		SELECT user_id, theme, language, chapters_per_page, reading_direction,
			default_status, show_spoilers, auto_sync, notifications_enabled,
			email_notifications, activity_public, library_public, keybindings,
			created_at, updated_at
		FROM user_preferences WHERE user_id = ?`

	var prefs models.UserPreferences
	var keybindingsJSON sql.NullString

	err := s.db.QueryRow(query, userID).Scan(
		&prefs.UserID, &prefs.Theme, &prefs.Language, &prefs.ChaptersPerPage,
		&prefs.ReadingDirection, &prefs.DefaultStatus, &prefs.ShowSpoilers,
		&prefs.AutoSync, &prefs.NotificationsEnabled, &prefs.EmailNotifications,
		&prefs.ActivityPublic, &prefs.LibraryPublic, &keybindingsJSON,
		&prefs.CreatedAt, &prefs.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		// Create default preferences
		defaults := models.DefaultPreferences(userID)
		if err := s.createPreferences(&defaults); err != nil {
			return nil, err
		}
		return &defaults, nil
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get preferences: %w", err)
	}

	// Parse keybindings
	if keybindingsJSON.Valid && keybindingsJSON.String != "" {
		if err := json.Unmarshal([]byte(keybindingsJSON.String), &prefs.Keybindings); err != nil {
			prefs.Keybindings = models.DefaultKeybindings()
		}
	} else {
		prefs.Keybindings = models.DefaultKeybindings()
	}

	return &prefs, nil
}

// createPreferences creates default preferences for a user
func (s *Service) createPreferences(prefs *models.UserPreferences) error {
	prefs.CreatedAt = time.Now()
	prefs.UpdatedAt = time.Now()

	keybindingsJSON, _ := json.Marshal(prefs.Keybindings)

	query := `
		INSERT INTO user_preferences (
			user_id, theme, language, chapters_per_page, reading_direction,
			default_status, show_spoilers, auto_sync, notifications_enabled,
			email_notifications, activity_public, library_public, keybindings,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := s.db.Exec(query,
		prefs.UserID, prefs.Theme, prefs.Language, prefs.ChaptersPerPage,
		prefs.ReadingDirection, prefs.DefaultStatus, prefs.ShowSpoilers,
		prefs.AutoSync, prefs.NotificationsEnabled, prefs.EmailNotifications,
		prefs.ActivityPublic, prefs.LibraryPublic, string(keybindingsJSON),
		prefs.CreatedAt, prefs.UpdatedAt,
	)

	return err
}

// UpdatePreferences updates user preferences
func (s *Service) UpdatePreferences(ctx context.Context, userID string, req *models.UpdatePreferencesRequest) (*models.UserPreferences, error) {
	// Get current preferences
	prefs, err := s.GetPreferences(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Apply updates
	if req.Theme != nil {
		prefs.Theme = *req.Theme
	}
	if req.Language != nil {
		prefs.Language = *req.Language
	}
	if req.ChaptersPerPage != nil {
		prefs.ChaptersPerPage = *req.ChaptersPerPage
	}
	if req.ReadingDirection != nil {
		prefs.ReadingDirection = *req.ReadingDirection
	}
	if req.DefaultStatus != nil {
		prefs.DefaultStatus = *req.DefaultStatus
	}
	if req.ShowSpoilers != nil {
		prefs.ShowSpoilers = *req.ShowSpoilers
	}
	if req.AutoSync != nil {
		prefs.AutoSync = *req.AutoSync
	}
	if req.NotificationsEnabled != nil {
		prefs.NotificationsEnabled = *req.NotificationsEnabled
	}
	if req.EmailNotifications != nil {
		prefs.EmailNotifications = *req.EmailNotifications
	}
	if req.ActivityPublic != nil {
		prefs.ActivityPublic = *req.ActivityPublic
	}
	if req.LibraryPublic != nil {
		prefs.LibraryPublic = *req.LibraryPublic
	}
	if req.Keybindings != nil {
		prefs.Keybindings = *req.Keybindings
	}

	prefs.UpdatedAt = time.Now()
	keybindingsJSON, _ := json.Marshal(prefs.Keybindings)

	query := `
		UPDATE user_preferences SET
			theme = ?, language = ?, chapters_per_page = ?, reading_direction = ?,
			default_status = ?, show_spoilers = ?, auto_sync = ?, notifications_enabled = ?,
			email_notifications = ?, activity_public = ?, library_public = ?, keybindings = ?,
			updated_at = ?
		WHERE user_id = ?`

	_, err = s.db.Exec(query,
		prefs.Theme, prefs.Language, prefs.ChaptersPerPage, prefs.ReadingDirection,
		prefs.DefaultStatus, prefs.ShowSpoilers, prefs.AutoSync, prefs.NotificationsEnabled,
		prefs.EmailNotifications, prefs.ActivityPublic, prefs.LibraryPublic, string(keybindingsJSON),
		prefs.UpdatedAt, userID,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to update preferences: %w", err)
	}

	return prefs, nil
}

// ResetPreferences resets preferences to defaults
func (s *Service) ResetPreferences(ctx context.Context, userID string) (*models.UserPreferences, error) {
	defaults := models.DefaultPreferences(userID)
	defaults.UpdatedAt = time.Now()

	keybindingsJSON, _ := json.Marshal(defaults.Keybindings)

	query := `
		UPDATE user_preferences SET
			theme = ?, language = ?, chapters_per_page = ?, reading_direction = ?,
			default_status = ?, show_spoilers = ?, auto_sync = ?, notifications_enabled = ?,
			email_notifications = ?, activity_public = ?, library_public = ?, keybindings = ?,
			updated_at = ?
		WHERE user_id = ?`

	_, err := s.db.Exec(query,
		defaults.Theme, defaults.Language, defaults.ChaptersPerPage, defaults.ReadingDirection,
		defaults.DefaultStatus, defaults.ShowSpoilers, defaults.AutoSync, defaults.NotificationsEnabled,
		defaults.EmailNotifications, defaults.ActivityPublic, defaults.LibraryPublic, string(keybindingsJSON),
		defaults.UpdatedAt, userID,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to reset preferences: %w", err)
	}

	return &defaults, nil
}

// ExportData exports user data
func (s *Service) ExportData(ctx context.Context, userID string, req *models.ExportDataRequest) (*models.ExportDataResponse, error) {
	exportData := make(map[string]interface{})
	exportData["exported_at"] = time.Now()
	exportData["user_id"] = userID

	// Export library (reading progress)
	if req.IncludeLibrary || req.IncludeProgress {
		rows, err := s.db.Query(`
			SELECT rp.id, rp.manga_id, rp.current_chapter, rp.total_chapters, rp.status,
				rp.rating, rp.notes, rp.is_favorite, rp.started_at, rp.completed_at, rp.last_read_at,
				m.title, m.author
			FROM reading_progress rp
			JOIN manga m ON rp.manga_id = m.id
			WHERE rp.user_id = ?`, userID)
		if err == nil {
			defer rows.Close()
			var library []map[string]interface{}
			for rows.Next() {
				var item struct {
					ID, MangaID, Status, Notes, Title, Author string
					CurrentChapter, TotalChapters             int
					Rating                                    sql.NullInt64
					IsFavorite                                bool
					StartedAt, CompletedAt, LastReadAt        sql.NullTime
				}
				rows.Scan(&item.ID, &item.MangaID, &item.CurrentChapter, &item.TotalChapters,
					&item.Status, &item.Rating, &item.Notes, &item.IsFavorite,
					&item.StartedAt, &item.CompletedAt, &item.LastReadAt, &item.Title, &item.Author)

				entry := map[string]interface{}{
					"manga_id":        item.MangaID,
					"title":           item.Title,
					"author":          item.Author,
					"current_chapter": item.CurrentChapter,
					"total_chapters":  item.TotalChapters,
					"status":          item.Status,
					"is_favorite":     item.IsFavorite,
					"notes":           item.Notes,
				}
				if item.Rating.Valid {
					entry["rating"] = item.Rating.Int64
				}
				if item.StartedAt.Valid {
					entry["started_at"] = item.StartedAt.Time
				}
				if item.CompletedAt.Valid {
					entry["completed_at"] = item.CompletedAt.Time
				}
				library = append(library, entry)
			}
			exportData["library"] = library
		}
	}

	// Export stats
	if req.IncludeStats {
		var stats struct {
			TotalChapters int
			TotalManga    int
			AvgRating     float64
		}
		s.db.QueryRow(`SELECT COUNT(*) FROM chapter_history WHERE user_id = ?`, userID).Scan(&stats.TotalChapters)
		s.db.QueryRow(`SELECT COUNT(*) FROM reading_progress WHERE user_id = ?`, userID).Scan(&stats.TotalManga)
		s.db.QueryRow(`SELECT COALESCE(AVG(rating), 0) FROM reading_progress WHERE user_id = ? AND rating IS NOT NULL`, userID).Scan(&stats.AvgRating)

		exportData["statistics"] = map[string]interface{}{
			"total_chapters_read": stats.TotalChapters,
			"total_manga":         stats.TotalManga,
			"average_rating":      stats.AvgRating,
		}
	}

	// Export custom lists
	if req.IncludeLists {
		rows, err := s.db.Query(`
			SELECT id, name, description, icon_emoji, is_public, manga_count
			FROM custom_lists WHERE user_id = ?`, userID)
		if err == nil {
			defer rows.Close()
			var lists []map[string]interface{}
			for rows.Next() {
				var list struct {
					ID, Name, Description, IconEmoji string
					IsPublic                         bool
					MangaCount                       int
				}
				rows.Scan(&list.ID, &list.Name, &list.Description, &list.IconEmoji, &list.IsPublic, &list.MangaCount)
				lists = append(lists, map[string]interface{}{
					"name":        list.Name,
					"description": list.Description,
					"icon_emoji":  list.IconEmoji,
					"is_public":   list.IsPublic,
					"manga_count": list.MangaCount,
				})
			}
			exportData["custom_lists"] = lists
		}
	}

	// Format response
	var dataBytes []byte
	var filename string

	switch req.Format {
	case "csv":
		// For simplicity, we'll export as JSON with .csv extension
		// A proper implementation would format as CSV
		dataBytes, _ = json.MarshalIndent(exportData, "", "  ")
		filename = fmt.Sprintf("mangahub_export_%s.json", time.Now().Format("20060102"))
	default:
		dataBytes, _ = json.MarshalIndent(exportData, "", "  ")
		filename = fmt.Sprintf("mangahub_export_%s.json", time.Now().Format("20060102"))
	}

	return &models.ExportDataResponse{
		UserID:     userID,
		ExportedAt: time.Now(),
		Format:     req.Format,
		Data:       string(dataBytes),
		Filename:   filename,
	}, nil
}

// Handler handles HTTP requests for preferences
type Handler struct {
	service *Service
}

// NewHandler creates a new preferences handler
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// GetPreferences returns user preferences
// GET /api/v1/preferences
func (h *Handler) GetPreferences(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := getUserIDFromContext(ctx)
	if userID == "" {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	prefs, err := h.service.GetPreferences(ctx, userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get preferences")
		return
	}

	respondJSON(w, http.StatusOK, prefs)
}

// UpdatePreferences updates user preferences
// PUT /api/v1/preferences
func (h *Handler) UpdatePreferences(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := getUserIDFromContext(ctx)
	if userID == "" {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req models.UpdatePreferencesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	prefs, err := h.service.UpdatePreferences(ctx, userID, &req)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update preferences")
		return
	}

	respondJSON(w, http.StatusOK, prefs)
}

// ResetPreferences resets preferences to defaults
// POST /api/v1/preferences/reset
func (h *Handler) ResetPreferences(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := getUserIDFromContext(ctx)
	if userID == "" {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	prefs, err := h.service.ResetPreferences(ctx, userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to reset preferences")
		return
	}

	respondJSON(w, http.StatusOK, prefs)
}

// ExportData exports user data
// POST /api/v1/preferences/export
func (h *Handler) ExportData(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := getUserIDFromContext(ctx)
	if userID == "" {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req models.ExportDataRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Default to JSON format
	if req.Format == "" {
		req.Format = "json"
	}

	export, err := h.service.ExportData(ctx, userID, &req)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to export data")
		return
	}

	respondJSON(w, http.StatusOK, export)
}

func getUserIDFromContext(ctx interface{}) string {
	if c, ok := ctx.(interface{ Value(interface{}) interface{} }); ok {
		if userID, ok := c.Value("user_id").(string); ok {
			return userID
		}
	}
	return ""
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, models.APIResponse{
		Success: false,
		Message: message,
	})
}
