// Package customlist - Custom Lists HTTP Handlers
// REST API endpoints for custom manga lists
package customlist

import (
	"encoding/json"
	"net/http"

	"mangahub/pkg/models"
)

// Handler handles HTTP requests for custom lists
type Handler struct {
	service *Service
}

// NewHandler creates a new custom list handler
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// GetUserLists returns all lists for the current user
// GET /api/v1/lists
func (h *Handler) GetUserLists(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := getUserIDFromContext(ctx)
	if userID == "" {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	lists, err := h.service.GetUserLists(ctx, userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get lists")
		return
	}

	respondJSON(w, http.StatusOK, lists)
}

// CreateList creates a new custom list
// POST /api/v1/lists
func (h *Handler) CreateList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := getUserIDFromContext(ctx)
	if userID == "" {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req models.CreateListRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	list, err := h.service.CreateList(ctx, userID, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, list)
}

// GetList returns a single list with its items
// GET /api/v1/lists/:id
func (h *Handler) GetList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := getUserIDFromContext(ctx)
	if userID == "" {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	listID := getListIDFromPath(r)

	list, err := h.service.GetListWithItems(ctx, listID, userID)
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, list)
}

// UpdateList updates a custom list
// PUT /api/v1/lists/:id
func (h *Handler) UpdateList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := getUserIDFromContext(ctx)
	if userID == "" {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	listID := getListIDFromPath(r)

	var req models.UpdateListRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	list, err := h.service.UpdateList(ctx, listID, userID, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, list)
}

// DeleteList deletes a custom list
// DELETE /api/v1/lists/:id
func (h *Handler) DeleteList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := getUserIDFromContext(ctx)
	if userID == "" {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	listID := getListIDFromPath(r)

	if err := h.service.DeleteList(ctx, listID, userID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "List deleted successfully"})
}

// AddToList adds a manga to a list
// POST /api/v1/lists/:id/items
func (h *Handler) AddToList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := getUserIDFromContext(ctx)
	if userID == "" {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	listID := getListIDFromPath(r)

	var req models.AddToListRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.service.AddToList(ctx, listID, userID, &req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, map[string]string{"message": "Manga added to list"})
}

// RemoveFromList removes a manga from a list
// DELETE /api/v1/lists/:id/items/:mangaId
func (h *Handler) RemoveFromList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := getUserIDFromContext(ctx)
	if userID == "" {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	listID := getListIDFromPath(r)
	mangaID := getMangaIDFromPath(r)

	if err := h.service.RemoveFromList(ctx, listID, mangaID, userID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Manga removed from list"})
}

// ReorderList reorders items in a list
// PUT /api/v1/lists/:id/reorder
func (h *Handler) ReorderList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := getUserIDFromContext(ctx)
	if userID == "" {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	listID := getListIDFromPath(r)

	var req models.ReorderListRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.service.ReorderList(ctx, listID, userID, &req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "List reordered successfully"})
}

// Helper functions

func getUserIDFromContext(ctx interface{}) string {
	if c, ok := ctx.(interface{ Value(interface{}) interface{} }); ok {
		if userID, ok := c.Value("user_id").(string); ok {
			return userID
		}
	}
	return ""
}

func getListIDFromPath(r *http.Request) string {
	return r.PathValue("id") // Go 1.22+
}

func getMangaIDFromPath(r *http.Request) string {
	return r.PathValue("mangaId") // Go 1.22+
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
