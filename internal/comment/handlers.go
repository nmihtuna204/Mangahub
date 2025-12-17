// Package comment - Comment HTTP Handlers
// HTTP handlers cho comment API endpoints
// Endpoints:
//   - POST /manga/:id/comments - Create comment
//   - GET /manga/:id/comments - Get comments (with optional ?chapter=N)
//   - PUT /comments/:id - Update comment
//   - DELETE /comments/:id - Delete comment
//   - POST /comments/:id/like - Like comment
//   - DELETE /comments/:id/like - Unlike comment
package comment

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"mangahub/internal/auth"
	"mangahub/pkg/models"
)

// Handler handles HTTP requests for comments
type Handler struct {
	svc Service
}

// NewHandler creates a new comment handler
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// CreateComment handles POST /manga/:id/comments
// Creates a new comment on a manga or chapter
// Request body: { content, chapter_number?, is_spoiler, parent_id? }
func (h *Handler) CreateComment(c *gin.Context) {
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
	var req models.CreateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest,
			models.NewErrorResponse(models.ErrCodeBadRequest, "invalid JSON body", map[string]interface{}{"error": err.Error()}))
		return
	}

	// Create comment
	comment, err := h.svc.Create(c.Request.Context(), user.ID, mangaID, req)
	if err != nil {
		if appErr, ok := err.(*models.AppError); ok {
			c.JSON(appErr.StatusCode,
				models.NewErrorResponse(appErr.Code, appErr.Message, appErr.Details))
			return
		}
		c.JSON(http.StatusInternalServerError,
			models.NewErrorResponse(models.ErrCodeInternal, "failed to create comment", nil))
		return
	}

	c.JSON(http.StatusCreated,
		models.NewSuccessResponse(comment, "comment created successfully"))
}

// GetComments handles GET /manga/:id/comments
// Retrieves comments for a manga with optional chapter filter
// Query params: ?chapter=N&page=1&page_size=20
func (h *Handler) GetComments(c *gin.Context) {
	// Get manga ID from URL
	mangaID := c.Param("id")
	if mangaID == "" {
		c.JSON(http.StatusBadRequest,
			models.NewErrorResponse(models.ErrCodeBadRequest, "manga_id is required", nil))
		return
	}

	// Parse optional chapter number from query
	var chapterNumber *int
	if chapterStr := c.Query("chapter"); chapterStr != "" {
		ch, err := strconv.Atoi(chapterStr)
		if err == nil {
			chapterNumber = &ch
		}
	}

	// Parse pagination
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	// Get current user ID if authenticated (optional)
	var currentUserID string
	if user := auth.GetCurrentUser(c); user != nil {
		currentUserID = user.ID
	}

	// Get comments
	response, err := h.svc.GetComments(c.Request.Context(), mangaID, chapterNumber, currentUserID, page, pageSize)
	if err != nil {
		if appErr, ok := err.(*models.AppError); ok {
			c.JSON(appErr.StatusCode,
				models.NewErrorResponse(appErr.Code, appErr.Message, appErr.Details))
			return
		}
		c.JSON(http.StatusInternalServerError,
			models.NewErrorResponse(models.ErrCodeInternal, "failed to get comments", nil))
		return
	}

	c.JSON(http.StatusOK,
		models.NewSuccessResponse(response, "comments retrieved"))
}

// UpdateComment handles PUT /comments/:id
// Updates a comment's content (only owner can update)
// Request body: { content, is_spoiler }
func (h *Handler) UpdateComment(c *gin.Context) {
	// Get authenticated user
	user := auth.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized,
			models.NewErrorResponse(models.ErrCodeUnauthorized, "authentication required", nil))
		return
	}

	// Get comment ID from URL
	commentID := c.Param("id")
	if commentID == "" {
		c.JSON(http.StatusBadRequest,
			models.NewErrorResponse(models.ErrCodeBadRequest, "comment_id is required", nil))
		return
	}

	// Parse request body
	var req models.UpdateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest,
			models.NewErrorResponse(models.ErrCodeBadRequest, "invalid JSON body", map[string]interface{}{"error": err.Error()}))
		return
	}

	// Update comment
	comment, err := h.svc.Update(c.Request.Context(), commentID, user.ID, req)
	if err != nil {
		if appErr, ok := err.(*models.AppError); ok {
			c.JSON(appErr.StatusCode,
				models.NewErrorResponse(appErr.Code, appErr.Message, appErr.Details))
			return
		}
		c.JSON(http.StatusInternalServerError,
			models.NewErrorResponse(models.ErrCodeInternal, "failed to update comment", nil))
		return
	}

	c.JSON(http.StatusOK,
		models.NewSuccessResponse(comment, "comment updated successfully"))
}

// DeleteComment handles DELETE /comments/:id
// Soft-deletes a comment (only owner can delete)
func (h *Handler) DeleteComment(c *gin.Context) {
	// Get authenticated user
	user := auth.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized,
			models.NewErrorResponse(models.ErrCodeUnauthorized, "authentication required", nil))
		return
	}

	// Get comment ID from URL
	commentID := c.Param("id")
	if commentID == "" {
		c.JSON(http.StatusBadRequest,
			models.NewErrorResponse(models.ErrCodeBadRequest, "comment_id is required", nil))
		return
	}

	// Delete comment
	err := h.svc.Delete(c.Request.Context(), commentID, user.ID)
	if err != nil {
		if appErr, ok := err.(*models.AppError); ok {
			c.JSON(appErr.StatusCode,
				models.NewErrorResponse(appErr.Code, appErr.Message, appErr.Details))
			return
		}
		c.JSON(http.StatusInternalServerError,
			models.NewErrorResponse(models.ErrCodeInternal, "failed to delete comment", nil))
		return
	}

	c.JSON(http.StatusOK,
		models.NewSuccessResponse(map[string]interface{}{
			"comment_id": commentID,
			"deleted":    true,
		}, "comment deleted successfully"))
}

// LikeComment handles POST /comments/:id/like
// Adds a like to a comment
func (h *Handler) LikeComment(c *gin.Context) {
	// Get authenticated user
	user := auth.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized,
			models.NewErrorResponse(models.ErrCodeUnauthorized, "authentication required", nil))
		return
	}

	// Get comment ID from URL
	commentID := c.Param("id")
	if commentID == "" {
		c.JSON(http.StatusBadRequest,
			models.NewErrorResponse(models.ErrCodeBadRequest, "comment_id is required", nil))
		return
	}

	// Like comment
	err := h.svc.Like(c.Request.Context(), commentID, user.ID)
	if err != nil {
		if appErr, ok := err.(*models.AppError); ok {
			c.JSON(appErr.StatusCode,
				models.NewErrorResponse(appErr.Code, appErr.Message, appErr.Details))
			return
		}
		c.JSON(http.StatusInternalServerError,
			models.NewErrorResponse(models.ErrCodeInternal, "failed to like comment", nil))
		return
	}

	c.JSON(http.StatusOK,
		models.NewSuccessResponse(map[string]interface{}{
			"comment_id": commentID,
			"liked":      true,
		}, "comment liked"))
}

// UnlikeComment handles DELETE /comments/:id/like
// Removes a like from a comment
func (h *Handler) UnlikeComment(c *gin.Context) {
	// Get authenticated user
	user := auth.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized,
			models.NewErrorResponse(models.ErrCodeUnauthorized, "authentication required", nil))
		return
	}

	// Get comment ID from URL
	commentID := c.Param("id")
	if commentID == "" {
		c.JSON(http.StatusBadRequest,
			models.NewErrorResponse(models.ErrCodeBadRequest, "comment_id is required", nil))
		return
	}

	// Unlike comment
	err := h.svc.Unlike(c.Request.Context(), commentID, user.ID)
	if err != nil {
		if appErr, ok := err.(*models.AppError); ok {
			c.JSON(appErr.StatusCode,
				models.NewErrorResponse(appErr.Code, appErr.Message, appErr.Details))
			return
		}
		c.JSON(http.StatusInternalServerError,
			models.NewErrorResponse(models.ErrCodeInternal, "failed to unlike comment", nil))
		return
	}

	c.JSON(http.StatusOK,
		models.NewSuccessResponse(map[string]interface{}{
			"comment_id": commentID,
			"liked":      false,
		}, "comment unliked"))
}
