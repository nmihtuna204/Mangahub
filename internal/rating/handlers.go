// Package rating - Rating HTTP Handlers
// HTTP handlers cho rating API endpoints
// Endpoints:
//   - POST /manga/:id/ratings - Submit/update rating
//   - GET /manga/:id/ratings - Get ratings summary
//   - DELETE /manga/:id/ratings - Remove user's rating
package rating

import (
	"context"
	"fmt"
	"net/http"

	"mangahub/internal/auth"
	"mangahub/pkg/models"

	"github.com/gin-gonic/gin"
)

type ActivityRecorder interface {
	RecordMangaRated(ctx context.Context, userID, username, mangaID, mangaTitle string, rating float64) error
}

// Handler handles HTTP requests for ratings
type Handler struct {
	svc              Service
	activityRecorder ActivityRecorder
	mangaSvc         MangaService
}

type MangaService interface {
	GetByID(ctx context.Context, id string) (*models.Manga, error)
}

// NewHandler creates a new rating handler
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// NewHandlerWithActivity creates handler with activity recording
func NewHandlerWithActivity(svc Service, activityRecorder ActivityRecorder, mangaSvc MangaService) *Handler {
	return &Handler{
		svc:              svc,
		activityRecorder: activityRecorder,
		mangaSvc:         mangaSvc,
	}
}

// SubmitRating handles POST /manga/:id/ratings
// Creates or updates a user's rating for a manga
// Request body: { rating, review_text, is_spoiler }
func (h *Handler) SubmitRating(c *gin.Context) {
	// Get authenticated user
	user := auth.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "authentication required",
		})
		return
	}

	// Get manga ID from URL
	mangaID := c.Param("id")
	if mangaID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "manga_id is required",
		})
		return
	}

	// Parse request body
	var req models.CreateRatingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	// Submit rating
	rating, err := h.svc.Rate(c.Request.Context(), user.ID, mangaID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to submit rating",
		})
		return
	}

	// 📝 ACTIVITY: Record rating activity
	if h.activityRecorder != nil && h.mangaSvc != nil {
		go func() {
			manga, err := h.mangaSvc.GetByID(c.Request.Context(), rating.MangaID)
			if err == nil {
				_ = h.activityRecorder.RecordMangaRated(
					c.Request.Context(),
					user.ID,
					user.Username,
					rating.MangaID,
					manga.Title,
					float64(rating.Rating),
				)
			}
		}()
	}

	c.JSON(http.StatusOK, gin.H{
		"data":    rating,
		"message": "rating submitted successfully",
	})
}

// GetRatings handles GET /manga/:id/ratings
// Returns aggregated rating stats + recent reviews for a manga
// Query params: ?page=1&limit=20
func (h *Handler) GetRatings(c *gin.Context) {
	// Get manga ID from URL
	mangaID := c.Param("id")
	if mangaID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "manga_id is required",
		})
		return
	}

	// Parse pagination from query params
	page := 1
	limit := 20
	if p := c.Query("page"); p != "" {
		if val, err := parseInt(p); err == nil && val > 0 {
			page = val
		}
	}
	if l := c.Query("limit"); l != "" {
		if val, err := parseInt(l); err == nil && val > 0 && val <= 100 {
			limit = val
		}
	}

	offset := (page - 1) * limit

	// Get ratings summary and recent ratings
	response, err := h.svc.GetMangaRatings(c.Request.Context(), mangaID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get ratings",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":    response,
		"message": "ratings retrieved",
	})
}

// DeleteRating handles DELETE /manga/:id/ratings
// Removes the current user's rating for a manga
func (h *Handler) DeleteRating(c *gin.Context) {
	// Get authenticated user
	user := auth.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "authentication required",
		})
		return
	}

	// Get manga ID from URL
	mangaID := c.Param("id")
	if mangaID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "manga_id is required",
		})
		return
	}

	// Delete rating
	err := h.svc.DeleteRating(c.Request.Context(), user.ID, mangaID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to delete rating",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": map[string]interface{}{
			"manga_id": mangaID,
			"removed":  true,
		},
		"message": "rating removed successfully",
	})
}

// Helper function to parse integer from string
func parseInt(s string) (int, error) {
	var val int
	_, err := fmt.Sscanf(s, "%d", &val)
	return val, err
}
