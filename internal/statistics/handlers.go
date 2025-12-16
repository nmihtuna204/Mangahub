// Package statistics - Reading Statistics HTTP Handlers
// REST API endpoints for reading statistics
package statistics

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"mangahub/pkg/models"
)

// Handler handles HTTP requests for statistics
type Handler struct {
	service *Service
}

// NewHandler creates a new statistics handler
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// GetStatistics returns comprehensive reading statistics
// GET /api/v1/statistics
func (h *Handler) GetStatistics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := getUserIDFromContext(ctx)
	if userID == "" {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	stats, err := h.service.GetStatistics(ctx, userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get statistics")
		return
	}

	respondJSON(w, http.StatusOK, stats)
}

// GetStatsOverview returns quick stats for dashboard
// GET /api/v1/statistics/overview
func (h *Handler) GetStatsOverview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := getUserIDFromContext(ctx)
	if userID == "" {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	overview, err := h.service.GetStatsOverview(ctx, userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get stats overview")
		return
	}

	respondJSON(w, http.StatusOK, overview)
}

// RecordChapterRead records a chapter read event
// POST /api/v1/statistics/chapter
func (h *Handler) RecordChapterRead(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := getUserIDFromContext(ctx)
	if userID == "" {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req models.RecordChapterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.MangaID == "" {
		respondError(w, http.StatusBadRequest, "manga_id is required")
		return
	}

	err := h.service.RecordChapterRead(ctx, &req, userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to record chapter read")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Chapter recorded successfully"})
}

// GetChapterHistory returns chapter reading history
// GET /api/v1/statistics/history
func (h *Handler) GetChapterHistory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := getUserIDFromContext(ctx)
	if userID == "" {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	limit := 50
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	history, err := h.service.GetChapterHistory(ctx, userID, limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get chapter history")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"data":   history,
		"limit":  limit,
		"offset": offset,
	})
}

// GetReadingHeatmap returns heatmap data
// GET /api/v1/statistics/heatmap
func (h *Handler) GetReadingHeatmap(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := getUserIDFromContext(ctx)
	if userID == "" {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	days := 365
	if d := r.URL.Query().Get("days"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 {
			days = parsed
		}
	}

	heatmap, err := h.service.GetReadingHeatmap(ctx, userID, days)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get heatmap")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"data": heatmap,
		"days": days,
	})
}

// GetDailyStats returns daily stats within a date range
// GET /api/v1/statistics/daily
func (h *Handler) GetDailyStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := getUserIDFromContext(ctx)
	if userID == "" {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Default to last 30 days
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -30)

	if s := r.URL.Query().Get("start"); s != "" {
		if parsed, err := time.Parse("2006-01-02", s); err == nil {
			startDate = parsed
		}
	}
	if e := r.URL.Query().Get("end"); e != "" {
		if parsed, err := time.Parse("2006-01-02", e); err == nil {
			endDate = parsed
		}
	}

	stats, err := h.service.GetDailyStats(ctx, userID, startDate, endDate)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get daily stats")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"data":       stats,
		"start_date": startDate.Format("2006-01-02"),
		"end_date":   endDate.Format("2006-01-02"),
	})
}

// Helper functions

func getUserIDFromContext(ctx interface{}) string {
	// Extract user ID from context - implementation depends on auth middleware
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
