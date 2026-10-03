// Package rating - Rating HTTP Handlers
// HTTP handlers cho rating API endpoints
// Endpoints:
//   - POST /manga/:id/ratings - Submit/update rating
//   - GET /manga/:id/ratings - Get ratings summary
//   - DELETE /manga/:id/ratings - Remove user's rating
//
// Rating activity is recorded by the activity_on_rating database triggers,
// not here, so each rating shows up exactly once in the feed.
package rating

import (
	"net/http"
	"strconv"

	"mangahub/internal/auth"
	"mangahub/pkg/models"

	"github.com/gin-gonic/gin"
)

// Handler handles HTTP requests for ratings
type Handler struct {
	svc Service
}

// NewHandler creates a new rating handler
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// SubmitRating handles POST /manga/:id/ratings
// Creates or updates a user's rating for a manga
// Request body: { rating, review_text, is_spoiler }
func (h *Handler) SubmitRating(c *gin.Context) {
	user := auth.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized,
			models.NewErrorResponse(models.ErrCodeUnauthorized, "authentication required", nil))
		return
	}

	var req models.CreateRatingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest,
			models.NewErrorResponse(models.ErrCodeBadRequest, "invalid JSON body", map[string]interface{}{"error": err.Error()}))
		return
	}

	rating, err := h.svc.Rate(c.Request.Context(), user.ID, c.Param("id"), req)
	if err != nil {
		writeError(c, err, "failed to submit rating")
		return
	}

	c.JSON(http.StatusOK, models.NewSuccessResponse(rating, "rating submitted successfully"))
}

// GetRatings handles GET /manga/:id/ratings
// Returns aggregated rating stats + recent reviews for a manga
// Query params: ?page=1&limit=20
func (h *Handler) GetRatings(c *gin.Context) {
	page := 1
	limit := 20
	if val, err := strconv.Atoi(c.Query("page")); err == nil && val > 0 {
		page = val
	}
	if val, err := strconv.Atoi(c.Query("limit")); err == nil && val > 0 && val <= 100 {
		limit = val
	}

	response, err := h.svc.GetMangaRatings(c.Request.Context(), c.Param("id"), limit, (page-1)*limit)
	if err != nil {
		writeError(c, err, "failed to get ratings")
		return
	}

	c.JSON(http.StatusOK, models.NewSuccessResponse(response, "ratings retrieved"))
}

// DeleteRating handles DELETE /manga/:id/ratings
// Removes the current user's rating for a manga
func (h *Handler) DeleteRating(c *gin.Context) {
	user := auth.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized,
			models.NewErrorResponse(models.ErrCodeUnauthorized, "authentication required", nil))
		return
	}

	mangaID := c.Param("id")
	if err := h.svc.DeleteRating(c.Request.Context(), user.ID, mangaID); err != nil {
		writeError(c, err, "failed to delete rating")
		return
	}

	c.JSON(http.StatusOK, models.NewSuccessResponse(map[string]interface{}{
		"manga_id": mangaID,
		"removed":  true,
	}, "rating removed successfully"))
}

// writeError maps service errors to their HTTP status (400/404/...) instead
// of reporting everything as a 500.
func writeError(c *gin.Context, err error, fallback string) {
	if appErr, ok := err.(*models.AppError); ok {
		c.JSON(appErr.StatusCode, models.NewErrorResponse(appErr.Code, appErr.Message, appErr.Details))
		return
	}
	c.JSON(http.StatusInternalServerError, models.NewErrorResponse(models.ErrCodeInternal, fallback, nil))
}
