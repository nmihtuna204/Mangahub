package progress

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"mangahub/internal/auth"
	"mangahub/internal/protocols"
	"mangahub/pkg/logger"
	"mangahub/pkg/models"
)

type ProtocolBridge interface {
	BroadcastProgressUpdate(ev protocols.ProgressEvent) error
}

type ActivityRecorder interface {
	RecordChapterRead(ctx context.Context, userID, username, mangaID, mangaTitle string, chapterNum int) error
	RecordMangaCompleted(ctx context.Context, userID, username, mangaID, mangaTitle string) error
}

type Handler struct {
	svc              Service
	bridge           ProtocolBridge
	activityRecorder ActivityRecorder
	mangaSvc         MangaService
}

type MangaService interface {
	GetByID(ctx context.Context, id string) (*models.Manga, error)
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

func NewHandlerWithActivity(svc Service, bridge ProtocolBridge, activityRecorder ActivityRecorder, mangaSvc MangaService) *Handler {
	return &Handler{
		svc:              svc,
		bridge:           bridge,
		activityRecorder: activityRecorder,
		mangaSvc:         mangaSvc,
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

	// Everything below runs after the response is sent. The gin.Context and its
	// request context are recycled once this handler returns, so the goroutine
	// only captures plain values and uses its own context.
	userID, username, token := user.ID, user.Username, auth.GetToken(c)
	chapterSent := req.CurrentChapter != nil && *req.CurrentChapter > 0
	completedSent := req.Status != nil && *req.Status == "completed"
	final := *progress // merged state after the update, not the raw request

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		mangaTitle := ""
		if h.mangaSvc != nil {
			if m, err := h.mangaSvc.GetByID(ctx, final.MangaID); err == nil {
				mangaTitle = m.Title
			}
		}

		// 🔄 BRIDGE: Broadcast update through all protocols
		if h.bridge != nil {
			_ = h.bridge.BroadcastProgressUpdate(protocols.ProgressEvent{
				UserID:     userID,
				Username:   username,
				MangaID:    final.MangaID,
				MangaTitle: mangaTitle,
				Chapter:    int32(final.CurrentChapter),
				Status:     final.Status,
				Token:      token,
			})
		}

		if h.activityRecorder == nil || mangaTitle == "" {
			return
		}
		// 📝 ACTIVITY: Record chapter read activity
		if chapterSent {
			if err := h.activityRecorder.RecordChapterRead(ctx, userID, username, final.MangaID, mangaTitle, final.CurrentChapter); err != nil {
				logger.Warnf("record chapter activity: %v", err)
			}
		}
		// 🎉 ACTIVITY: Record completion if manga is completed
		if completedSent {
			if err := h.activityRecorder.RecordMangaCompleted(ctx, userID, username, final.MangaID, mangaTitle); err != nil {
				logger.Warnf("record completion activity: %v", err)
			}
		}
	}()

	c.JSON(http.StatusOK,
		models.NewSuccessResponse(progress, "reading progress updated"))
}
