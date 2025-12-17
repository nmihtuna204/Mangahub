package logger

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// GinLogger returns a gin.HandlerFunc for logging
func GinLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()

		// Process request
		c.Next()

		// Calculate latency
		latency := time.Since(startTime)
		latencyMs := latency.Milliseconds()

		// Get status code
		statusCode := c.Writer.Status()

		// Protocol-aware logging with clear [HTTP] prefix
		HTTP(c.Request.Method, c.Request.URL.Path, statusCode, latencyMs)

		// Additional structured logging for errors
		if len(c.Errors) > 0 {
			Get().WithFields(logrus.Fields{
				"protocol": ProtocolHTTP,
				"method":   c.Request.Method,
				"path":     c.Request.URL.Path,
				"status":   statusCode,
				"errors":   c.Errors.String(),
			}).Error("[HTTP] Request failed with errors")
		}
	}
}

// Recovery returns a gin.HandlerFunc for recovering from panics
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				Get().WithFields(logrus.Fields{
					"error": err,
					"path":  c.Request.URL.Path,
				}).Error("Panic recovered")
				c.AbortWithStatus(500)
			}
		}()
		c.Next()
	}
}
