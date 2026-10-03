package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"mangahub/pkg/models"
)

const (
	ContextUserKey  = "currentUser"
	ContextTokenKey = "currentToken"
)

// JWTMiddleware requires a valid "Authorization: Bearer <token>" header.
func JWTMiddleware(authService Service) gin.HandlerFunc {
	return requireToken(authService, false)
}

// WSAuthMiddleware is JWTMiddleware that also accepts a ?token= query parameter,
// because browsers and tools like wscat cannot set headers on a WebSocket upgrade.
func WSAuthMiddleware(authService Service) gin.HandlerFunc {
	return requireToken(authService, true)
}

// OptionalAuthMiddleware sets the current user when a valid bearer token is
// present, and otherwise lets the request through anonymously.
func OptionalAuthMiddleware(authService Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		if token, err := bearerToken(c.GetHeader("Authorization")); err == nil && token != "" {
			if userProfile, err := authService.ParseToken(token); err == nil {
				c.Set(ContextUserKey, userProfile)
			}
		}
		c.Next()
	}
}

func requireToken(authService Service, allowQuery bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := bearerToken(c.GetHeader("Authorization"))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized,
				models.NewErrorResponse(models.ErrCodeUnauthorized, "invalid Authorization header format", nil))
			return
		}
		if token == "" && allowQuery {
			token = c.Query("token")
		}
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized,
				models.NewErrorResponse(models.ErrCodeUnauthorized, "missing Authorization header", nil))
			return
		}

		userProfile, err := authService.ParseToken(token)
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
		c.Set(ContextTokenKey, token)
		c.Next()
	}
}

// GetToken returns the JWT the current request authenticated with ("" if none),
// for forwarding to other services (e.g. the gRPC audit call).
func GetToken(c *gin.Context) string {
	return c.GetString(ContextTokenKey)
}

// bearerToken extracts the token from an Authorization header value.
// An empty header yields "", nil; a malformed one yields an error.
func bearerToken(header string) (string, error) {
	if header == "" {
		return "", nil
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", models.ErrInvalidToken
	}
	return parts[1], nil
}

// RequireRole allows the request only if the authenticated user has the role.
// Use it after JWTMiddleware.
func RequireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := GetCurrentUser(c)
		if user == nil || user.Role != role {
			c.AbortWithStatusJSON(http.StatusForbidden,
				models.NewErrorResponse(models.ErrCodeForbidden, role+" role required", nil))
			return
		}
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
