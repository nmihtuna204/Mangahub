package manga

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/mangahub/pkg/models"
)

type Handler struct {
	svc Service
}

func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) ListManga(c *gin.Context) {
	var req models.MangaSearchRequest
	req.Query = c.Query("q")
	req.Status = c.Query("status")
	req.SortBy = c.Query("sort_by")
	req.Order = c.Query("order")

	if limitStr := c.Query("limit"); limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil {
			req.Limit = v
		}
	}
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if v, err := strconv.Atoi(offsetStr); err == nil {
			req.Offset = v
		}
	}

	if err := models.ValidateMangaSearch(&req); err != nil {
		c.JSON(http.StatusBadRequest,
			models.NewErrorResponse(models.ErrCodeValidation, "invalid search parameters", map[string]interface{}{"error": err.Error()}))
		return
	}

	resp, err := h.svc.List(c.Request.Context(), req)
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
		models.NewSuccessResponse(resp, "manga list"))
}

func (h *Handler) GetManga(c *gin.Context) {
	id := c.Param("id")
	m, err := h.svc.GetByID(c.Request.Context(), id)
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
		models.NewSuccessResponse(m, "manga details"))
}
