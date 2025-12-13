package progress

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"mangahub/internal/auth"
	"mangahub/pkg/models"
)

type ProtocolBridge interface {
	BroadcastProgressUpdate(userID, username, mangaID string, chapter int32, status string) error
}

type Handler struct {
	svc    Service
	bridge ProtocolBridge
}

func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

func NewHandlerWithBridge(svc Service, bridge ProtocolBridge) *Handler {
	return &Handler{
		svc:    svc,
		bridge: bridge,
	}
}

// POST /users/library  (add manga to library with initial status/progress)
func (h *Handler) AddToLibrary(c *gin.Context) {
	user := auth.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized,
			models.NewErrorResponse(models.ErrCodeUnauthorized, "unauthorized", nil))
		return
	}

	var req models.UpdateProgressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest,
			models.NewErrorResponse(models.ErrCodeBadRequest, "invalid JSON body", map[string]interface{}{"error": err.Error()}))
		return
	}

	progress, err := h.svc.Update(c.Request.Context(), user.ID, req)
	if err != nil {
		if appErr, ok := err.(*models.AppError); ok {
			c.JSON(appErr.StatusCode,
				models.NewErrorResponse(appErr.Code, appErr.Message, appErr.Details))
			return
		}
		c.JSON(http.StatusInternalServerError,
			models.NewErrorResponse(models.ErrCodeInternal, "unexpected error", nil))
		return
	}

	c.JSON(http.StatusCreated,
		models.NewSuccessResponse(progress, "manga added to library"))
}

// GET /users/library
func (h *Handler) GetLibrary(c *gin.Context) {
	user := auth.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized,
			models.NewErrorResponse(models.ErrCodeUnauthorized, "unauthorized", nil))
		return
	}

	list, err := h.svc.List(c.Request.Context(), user.ID)
	if err != nil {
		if appErr, ok := err.(*models.AppError); ok {
			c.JSON(appErr.StatusCode,
				models.NewErrorResponse(appErr.Code, appErr.Message, appErr.Details))
			return
		}
		c.JSON(http.StatusInternalServerError,
			models.NewErrorResponse(models.ErrCodeInternal, "unexpected error", nil))
		return
	}

	c.JSON(http.StatusOK,
		models.NewSuccessResponse(list, "user library"))
}

// DELETE /users/library/:manga_id
func (h *Handler) RemoveFromLibrary(c *gin.Context) {
	user := auth.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized,
			models.NewErrorResponse(models.ErrCodeUnauthorized, "unauthorized", nil))
		return
	}

	mangaID := c.Param("manga_id")
	if mangaID == "" {
		c.JSON(http.StatusBadRequest,
			models.NewErrorResponse(models.ErrCodeBadRequest, "manga_id is required", nil))
		return
	}

	err := h.svc.Delete(c.Request.Context(), user.ID, mangaID)
	if err != nil {
		if appErr, ok := err.(*models.AppError); ok {
			c.JSON(appErr.StatusCode,
				models.NewErrorResponse(appErr.Code, appErr.Message, appErr.Details))
			return
		}
		c.JSON(http.StatusInternalServerError,
			models.NewErrorResponse(models.ErrCodeInternal, "unexpected error", nil))
		return
	}

	c.JSON(http.StatusOK,
		models.NewSuccessResponse(map[string]interface{}{
			"manga_id": mangaID,
			"removed":  true,
		}, "manga removed from library"))
}

// PUT /users/progress
func (h *Handler) UpdateProgress(c *gin.Context) {
	user := auth.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized,
			models.NewErrorResponse(models.ErrCodeUnauthorized, "unauthorized", nil))
		return
	}

	var req models.UpdateProgressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest,
			models.NewErrorResponse(models.ErrCodeBadRequest, "invalid JSON body", map[string]interface{}{"error": err.Error()}))
		return
	}

	progress, err := h.svc.Update(c.Request.Context(), user.ID, req)
	if err != nil {
		if appErr, ok := err.(*models.AppError); ok {
			c.JSON(appErr.StatusCode,
				models.NewErrorResponse(appErr.Code, appErr.Message, appErr.Details))
			return
		}
		c.JSON(http.StatusInternalServerError,
			models.NewErrorResponse(models.ErrCodeInternal, "unexpected error", nil))
		return
	}

	// 🔄 BRIDGE: Broadcast update through all protocols
	if h.bridge != nil {
		go func() {
			_ = h.bridge.BroadcastProgressUpdate(
				user.ID,
				user.Username,
				req.MangaID,
				int32(req.CurrentChapter),
				req.Status,
			)
		}()
	}

	c.JSON(http.StatusOK,
		models.NewSuccessResponse(progress, "reading progress updated"))
}
