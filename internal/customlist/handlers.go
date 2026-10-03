// Package customlist - Custom Lists HTTP Handlers
// REST API endpoints for custom manga lists:
//   - GET    /lists                      - my lists (or ?user_id= someone's public lists)
//   - POST   /lists                      - create a list
//   - GET    /lists/:id                  - a list with its manga
//   - PUT    /lists/:id                  - rename / describe / change visibility
//   - DELETE /lists/:id                  - delete a list
//   - POST   /lists/:id/items            - add a manga
//   - DELETE /lists/:id/items/:manga_id  - remove a manga
//   - PUT    /lists/:id/order            - reorder items
package customlist

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"mangahub/internal/auth"
	"mangahub/pkg/models"
)

// Handler handles HTTP requests for custom lists
type Handler struct {
	svc *Service
}

// NewHandler creates a new custom list handler
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// GetLists handles GET /lists[?user_id=…]
// Without user_id it returns the caller's lists (auth required); with user_id
// it returns that user's public lists (all of them if it's the caller).
func (h *Handler) GetLists(c *gin.Context) {
	viewerID := ""
	if user := auth.GetCurrentUser(c); user != nil {
		viewerID = user.ID
	}
	ownerID := c.Query("user_id")
	if ownerID == "" {
		if viewerID == "" {
			c.JSON(http.StatusUnauthorized,
				models.NewErrorResponse(models.ErrCodeUnauthorized, "log in, or pass ?user_id= to see someone's public lists", nil))
			return
		}
		ownerID = viewerID
	}

	resp, err := h.svc.ListsOf(c.Request.Context(), ownerID, viewerID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, models.NewSuccessResponse(resp, "lists retrieved"))
}

// CreateList handles POST /lists
func (h *Handler) CreateList(c *gin.Context) {
	user := auth.GetCurrentUser(c)
	var req models.CreateListRequest
	if !bind(c, &req) {
		return
	}
	list, err := h.svc.CreateList(c.Request.Context(), user.ID, req)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, models.NewSuccessResponse(list, "list created"))
}

// GetList handles GET /lists/:id
func (h *Handler) GetList(c *gin.Context) {
	viewerID := ""
	if user := auth.GetCurrentUser(c); user != nil {
		viewerID = user.ID
	}
	list, err := h.svc.GetList(c.Request.Context(), c.Param("id"), viewerID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, models.NewSuccessResponse(list, "list retrieved"))
}

// UpdateList handles PUT /lists/:id
func (h *Handler) UpdateList(c *gin.Context) {
	user := auth.GetCurrentUser(c)
	var req models.UpdateListRequest
	if !bind(c, &req) {
		return
	}
	list, err := h.svc.UpdateList(c.Request.Context(), c.Param("id"), user.ID, req)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, models.NewSuccessResponse(list, "list updated"))
}

// DeleteList handles DELETE /lists/:id
func (h *Handler) DeleteList(c *gin.Context) {
	user := auth.GetCurrentUser(c)
	if err := h.svc.DeleteList(c.Request.Context(), c.Param("id"), user.ID); err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, models.NewSuccessResponse(gin.H{"list_id": c.Param("id"), "deleted": true}, "list deleted"))
}

// AddItem handles POST /lists/:id/items
func (h *Handler) AddItem(c *gin.Context) {
	user := auth.GetCurrentUser(c)
	var req models.AddToListRequest
	if !bind(c, &req) {
		return
	}
	item, err := h.svc.AddToList(c.Request.Context(), c.Param("id"), user.ID, req)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, models.NewSuccessResponse(item, "manga added to list"))
}

// RemoveItem handles DELETE /lists/:id/items/:manga_id
func (h *Handler) RemoveItem(c *gin.Context) {
	user := auth.GetCurrentUser(c)
	if err := h.svc.RemoveFromList(c.Request.Context(), c.Param("id"), c.Param("manga_id"), user.ID); err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, models.NewSuccessResponse(
		gin.H{"list_id": c.Param("id"), "manga_id": c.Param("manga_id"), "removed": true}, "manga removed from list"))
}

// ReorderItems handles PUT /lists/:id/order
func (h *Handler) ReorderItems(c *gin.Context) {
	user := auth.GetCurrentUser(c)
	var req models.ReorderListRequest
	if !bind(c, &req) {
		return
	}
	if err := h.svc.ReorderList(c.Request.Context(), c.Param("id"), user.ID, req); err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, models.NewSuccessResponse(gin.H{"list_id": c.Param("id"), "reordered": true}, "list reordered"))
}

func bind(c *gin.Context, req interface{}) bool {
	if err := c.ShouldBindJSON(req); err != nil {
		c.JSON(http.StatusBadRequest,
			models.NewErrorResponse(models.ErrCodeBadRequest, "invalid JSON body", map[string]interface{}{"error": err.Error()}))
		return false
	}
	return true
}

func writeError(c *gin.Context, err error) {
	if appErr, ok := err.(*models.AppError); ok {
		c.JSON(appErr.StatusCode, models.NewErrorResponse(appErr.Code, appErr.Message, appErr.Details))
		return
	}
	c.JSON(http.StatusInternalServerError, models.NewErrorResponse(models.ErrCodeInternal, "unexpected error", nil))
}
