package chapters

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"mangahub/pkg/logger"
	"mangahub/pkg/models"
)

// Handler exposes chapter releases over HTTP (admin only; see internal/server).
type Handler struct {
	svc *Service
}

// NewHandler creates the chapter release handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// ReleaseChapter handles POST /admin/manga/:id/chapters {"chapter": N}:
// record that chapter N is out and notify the manga's readers.
func (h *Handler) ReleaseChapter(c *gin.Context) {
	var req struct {
		Chapter int `json:"chapter"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest,
			models.NewErrorResponse(models.ErrCodeBadRequest, "invalid JSON body", map[string]interface{}{"error": err.Error()}))
		return
	}

	rel, err := h.svc.Release(c.Request.Context(), c.Param("id"), req.Chapter)
	if err != nil {
		if appErr, ok := err.(*models.AppError); ok {
			c.JSON(appErr.StatusCode, models.NewErrorResponse(appErr.Code, appErr.Message, appErr.Details))
			return
		}
		if rel == nil {
			logger.Errorf("release chapter: %v", err)
			c.JSON(http.StatusInternalServerError,
				models.NewErrorResponse(models.ErrCodeInternal, "failed to release chapter", nil))
			return
		}
		// Recorded, but the notification didn't go out
		logger.Warnf("release chapter: %v", err)
	}
	c.JSON(http.StatusCreated, models.NewSuccessResponse(rel, "chapter released"))
}
