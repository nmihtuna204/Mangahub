package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"mangahub/pkg/models"
)

const (
	ContextUserKey = "currentUser"
)

func JWTMiddleware(authService Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized,
				models.NewErrorResponse(models.ErrCodeUnauthorized, "missing Authorization header", nil))
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized,
				models.NewErrorResponse(models.ErrCodeUnauthorized, "invalid Authorization header format", nil))
			return
		}

		userProfile, err := authService.ParseToken(parts[1])
		if err != nil {
			if appErr, ok := err.(*models.AppError); ok {
				c.AbortWithStatusJSON(appErr.StatusCode,
					models.NewErrorResponse(appErr.Code, appErr.Message, appErr.Details))
				return
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized,
				models.NewErrorResponse(models.ErrCodeUnauthorized, "invalid token", nil))
			return
		}

		c.Set(ContextUserKey, userProfile)
		c.Next()
	}
}

func GetCurrentUser(c *gin.Context) *models.UserProfile {
	val, exists := c.Get(ContextUserKey)
	if !exists {
		return nil
	}
	if user, ok := val.(*models.UserProfile); ok {
		return user
	}
	return nil
}
