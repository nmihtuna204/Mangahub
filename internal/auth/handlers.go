package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"mangahub/pkg/models"
)

type Handler struct {
	svc Service
}

func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(c *gin.Context) {
	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest,
			models.NewErrorResponse(models.ErrCodeBadRequest, "invalid JSON body", map[string]interface{}{"error": err.Error()}))
		return
	}

	user, err := h.svc.Register(c.Request.Context(), req)
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
		models.NewSuccessResponse(user, "user registered successfully"))
}

func (h *Handler) Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest,
			models.NewErrorResponse(models.ErrCodeBadRequest, "invalid JSON body", map[string]interface{}{"error": err.Error()}))
		return
	}

	resp, err := h.svc.Login(c.Request.Context(), req)
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
		models.NewSuccessResponse(resp, "login successful"))
}

// GetMe returns the currently authenticated user's profile
func (h *Handler) GetMe(c *gin.Context) {
	user := GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized,
			models.NewErrorResponse(models.ErrCodeUnauthorized, "not authenticated", nil))
		return
	}

	c.JSON(http.StatusOK,
		models.NewSuccessResponse(user, "user profile retrieved"))
}

// Logout handles user logout
// Note: With stateless JWT, we just return success. 
// Token blacklisting will be implemented with Redis in Phase 2.
func (h *Handler) Logout(c *gin.Context) {
	user := GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized,
			models.NewErrorResponse(models.ErrCodeUnauthorized, "not authenticated", nil))
		return
	}

	// TODO Phase 2: Add token to blacklist in Redis
	// For now, client should discard the token

	c.JSON(http.StatusOK,
		models.NewSuccessResponse(map[string]interface{}{
			"message": "logged out successfully",
			"user_id": user.ID,
		}, "logout successful"))
}

// RefreshToken generates a new token for the current user
// Useful for extending session without re-login
func (h *Handler) RefreshToken(c *gin.Context) {
	user := GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized,
			models.NewErrorResponse(models.ErrCodeUnauthorized, "not authenticated", nil))
		return
	}

	// Get new token from service
	token, err := h.svc.RefreshToken(c.Request.Context(), user.ID)
	if err != nil {
		if appErr, ok := err.(*models.AppError); ok {
			c.JSON(appErr.StatusCode,
				models.NewErrorResponse(appErr.Code, appErr.Message, appErr.Details))
			return
		}
		c.JSON(http.StatusInternalServerError,
			models.NewErrorResponse(models.ErrCodeInternal, "failed to refresh token", nil))
		return
	}

	c.JSON(http.StatusOK,
		models.NewSuccessResponse(map[string]interface{}{
			"token":   token,
			"user_id": user.ID,
		}, "token refreshed"))
}
