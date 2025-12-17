// Package activity - Activity Feed HTTP Handlers
// REST API endpoints for activity feed
package activity

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handler handles HTTP requests for activities
type Handler struct {
	service *Service
}

// NewHandler creates a new activity handler
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// GetRecentActivities handles GET /activities
// Returns recent activities across all users
func (h *Handler) GetRecentActivities(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	activities, total, err := h.service.GetRecentActivities(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"activities": activities,
		"total":      total,
		"limit":      limit,
		"offset":     offset,
	})
}

// GetUserActivities handles GET /activities/user/:userID
// Returns activities for a specific user
func (h *Handler) GetUserActivities(c *gin.Context) {
	userID := c.Param("userID")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	activities, total, err := h.service.GetUserActivities(c.Request.Context(), userID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"activities": activities,
		"total":      total,
		"limit":      limit,
		"offset":     offset,
	})
}
