// Package logger - Structured Logging System
// Cung cấp logging functionality cho toàn bộ application
// Chức năng:
//   - Multiple log levels (debug, info, warn, error, fatal)
//   - JSON và text format support
//   - Output đến stdout hoặc file
//   - Request logging middleware cho Gin
//   - Sử dụng logrus cho structured logging
package logger

import (
	"os"

	"github.com/sirupsen/logrus"
)

var log *logrus.Logger

// Config holds logger configuration
type Config struct {
	Level  string
	Format string
	Output string
}

// Init initializes the logger
func Init(config Config) {
	log = logrus.New()

	// Set log level
	level, err := logrus.ParseLevel(config.Level)
	if err != nil {
		level = logrus.InfoLevel
	}
	log.SetLevel(level)

	// Set formatter
	if config.Format == "json" {
		log.SetFormatter(&logrus.JSONFormatter{})
	} else {
		log.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})
	}

	// Set output
	if config.Output == "stdout" || config.Output == "" {
		log.SetOutput(os.Stdout)
	} else {
		file, err := os.OpenFile(config.Output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			log.Fatal("Failed to open log file:", err)
		}
		log.SetOutput(file)
	}
}

// Get returns the logger instance
func Get() *logrus.Logger {
	if log == nil {
		// Initialize with defaults if not initialized
		Init(Config{
			Level:  "info",
			Format: "text",
			Output: "stdout",
		})
	}
	return log
}

// WithField creates a log entry with a field
func WithField(key string, value interface{}) *logrus.Entry {
	return Get().WithField(key, value)
}

// WithFields creates a log entry with multiple fields
func WithFields(fields logrus.Fields) *logrus.Entry {
	return Get().WithFields(fields)
}

// Info logs an info message
func Info(args ...interface{}) {
	Get().Info(args...)
}

// Infof logs a formatted info message
func Infof(format string, args ...interface{}) {
	Get().Infof(format, args...)
}

// Debug logs a debug message
func Debug(args ...interface{}) {
	Get().Debug(args...)
}

// Debugf logs a formatted debug message
func Debugf(format string, args ...interface{}) {
	Get().Debugf(format, args...)
}

// Warn logs a warning message
func Warn(args ...interface{}) {
	Get().Warn(args...)
}

// Warnf logs a formatted warning message
func Warnf(format string, args ...interface{}) {
	Get().Warnf(format, args...)
}

// Error logs an error message
func Error(args ...interface{}) {
	Get().Error(args...)
}

// Errorf logs a formatted error message
func Errorf(format string, args ...interface{}) {
	Get().Errorf(format, args...)
}

// Fatal logs a fatal message and exits
func Fatal(args ...interface{}) {
	Get().Fatal(args...)
}

// Fatalf logs a formatted fatal message and exits
func Fatalf(format string, args ...interface{}) {
	Get().Fatalf(format, args...)
}
