// Package rating - Rating HTTP Handlers
// HTTP handlers cho rating API endpoints
// Endpoints:
//   - POST /manga/:id/ratings - Submit/update rating
//   - GET /manga/:id/ratings - Get ratings summary
//   - DELETE /manga/:id/ratings - Remove user's rating
package rating

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"mangahub/internal/auth"
	"mangahub/pkg/models"
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
// Request body: { overall_rating, story_rating, art_rating, ... }
func (h *Handler) SubmitRating(c *gin.Context) {
	// Get authenticated user
	user := auth.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized,
			models.NewErrorResponse(models.ErrCodeUnauthorized, "authentication required", nil))
		return
	}

	// Get manga ID from URL
	mangaID := c.Param("id")
	if mangaID == "" {
		c.JSON(http.StatusBadRequest,
			models.NewErrorResponse(models.ErrCodeBadRequest, "manga_id is required", nil))
		return
	}

	// Parse request body
	var req models.CreateRatingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest,
			models.NewErrorResponse(models.ErrCodeBadRequest, "invalid JSON body", map[string]interface{}{"error": err.Error()}))
		return
	}

	// Submit rating
	rating, err := h.svc.Rate(c.Request.Context(), user.ID, mangaID, req)
	if err != nil {
		if appErr, ok := err.(*models.AppError); ok {
			c.JSON(appErr.StatusCode,
				models.NewErrorResponse(appErr.Code, appErr.Message, appErr.Details))
			return
		}
		c.JSON(http.StatusInternalServerError,
			models.NewErrorResponse(models.ErrCodeInternal, "failed to submit rating", nil))
		return
	}

	c.JSON(http.StatusOK,
		models.NewSuccessResponse(rating, "rating submitted successfully"))
}

// GetRatings handles GET /manga/:id/ratings
// Returns aggregated rating stats + recent reviews for a manga
// Optional: includes current user's rating if authenticated
func (h *Handler) GetRatings(c *gin.Context) {
	// Get manga ID from URL
	mangaID := c.Param("id")
	if mangaID == "" {
		c.JSON(http.StatusBadRequest,
			models.NewErrorResponse(models.ErrCodeBadRequest, "manga_id is required", nil))
		return
	}

	// Get current user ID if authenticated (optional)
	var currentUserID string
	if user := auth.GetCurrentUser(c); user != nil {
		currentUserID = user.ID
	}

	// Get rating summary
	summary, err := h.svc.GetRatingSummary(c.Request.Context(), mangaID, currentUserID)
	if err != nil {
		if appErr, ok := err.(*models.AppError); ok {
			c.JSON(appErr.StatusCode,
				models.NewErrorResponse(appErr.Code, appErr.Message, appErr.Details))
			return
		}
		c.JSON(http.StatusInternalServerError,
			models.NewErrorResponse(models.ErrCodeInternal, "failed to get ratings", nil))
		return
	}

	c.JSON(http.StatusOK,
		models.NewSuccessResponse(summary, "ratings retrieved"))
}

// DeleteRating handles DELETE /manga/:id/ratings
// Removes the current user's rating for a manga
func (h *Handler) DeleteRating(c *gin.Context) {
	// Get authenticated user
	user := auth.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized,
			models.NewErrorResponse(models.ErrCodeUnauthorized, "authentication required", nil))
		return
	}

	// Get manga ID from URL
	mangaID := c.Param("id")
	if mangaID == "" {
		c.JSON(http.StatusBadRequest,
			models.NewErrorResponse(models.ErrCodeBadRequest, "manga_id is required", nil))
		return
	}

	// Delete rating
	err := h.svc.DeleteRating(c.Request.Context(), user.ID, mangaID)
	if err != nil {
		if appErr, ok := err.(*models.AppError); ok {
			c.JSON(appErr.StatusCode,
				models.NewErrorResponse(appErr.Code, appErr.Message, appErr.Details))
			return
		}
		c.JSON(http.StatusInternalServerError,
			models.NewErrorResponse(models.ErrCodeInternal, "failed to delete rating", nil))
		return
	}

	c.JSON(http.StatusOK,
		models.NewSuccessResponse(map[string]interface{}{
			"manga_id": mangaID,
			"deleted":  true,
		}, "rating deleted successfully"))
}
