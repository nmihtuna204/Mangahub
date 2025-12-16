// Package leaderboard - Leaderboard HTTP Handlers
// HTTP handlers cho leaderboard API endpoints
// Endpoints:
//   - GET /leaderboards/manga - Top rated manga
//   - GET /leaderboards/users - Most active users
//   - GET /leaderboards/trending - Trending manga (with ?days=7 or 30)
package leaderboard

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"mangahub/pkg/models"
)

// Handler handles HTTP requests for leaderboards
type Handler struct {
	svc Service
}

// NewHandler creates a new leaderboard handler
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// GetTopRatedManga handles GET /leaderboards/manga
// Returns manga sorted by rating
// Query params: ?limit=20&offset=0
func (h *Handler) GetTopRatedManga(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	response, err := h.svc.GetTopRatedManga(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError,
			models.NewErrorResponse(models.ErrCodeInternal, "failed to get leaderboard", map[string]interface{}{"error": err.Error()}))
		return
	}

	c.JSON(http.StatusOK,
		models.NewSuccessResponse(response, "top rated manga"))
}

// GetMostActiveUsers handles GET /leaderboards/users
// Returns users sorted by engagement score
// Query params: ?limit=20&offset=0
func (h *Handler) GetMostActiveUsers(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	response, err := h.svc.GetMostActiveUsers(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError,
			models.NewErrorResponse(models.ErrCodeInternal, "failed to get leaderboard", map[string]interface{}{"error": err.Error()}))
		return
	}

	c.JSON(http.StatusOK,
		models.NewSuccessResponse(response, "most active users"))
}

// GetTrendingManga handles GET /leaderboards/trending
// Returns manga with most activity recently
// Query params: ?limit=20&offset=0&days=7 (7=weekly, 30=monthly)
func (h *Handler) GetTrendingManga(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	days, _ := strconv.Atoi(c.DefaultQuery("days", "7"))

	response, err := h.svc.GetTrendingManga(c.Request.Context(), limit, offset, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError,
			models.NewErrorResponse(models.ErrCodeInternal, "failed to get leaderboard", map[string]interface{}{"error": err.Error()}))
		return
	}

	c.JSON(http.StatusOK,
		models.NewSuccessResponse(response, "trending manga"))
}
