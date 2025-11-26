package models

import (
	"errors"
	"fmt"
)

// Common error codes
const (
	ErrCodeValidation      = "VALIDATION_ERROR"
	ErrCodeNotFound        = "NOT_FOUND"
	ErrCodeUnauthorized    = "UNAUTHORIZED"
	ErrCodeForbidden       = "FORBIDDEN"
	ErrCodeConflict        = "CONFLICT"
	ErrCodeInternal        = "INTERNAL_ERROR"
	ErrCodeBadRequest      = "BAD_REQUEST"
	ErrCodeServiceUnavailable = "SERVICE_UNAVAILABLE"
)

// Common errors
var (
	ErrUserNotFound        = errors.New("user not found")
	ErrMangaNotFound       = errors.New("manga not found")
	ErrProgressNotFound    = errors.New("reading progress not found")
	ErrInvalidCredentials  = errors.New("invalid username or password")
	ErrUsernameExists      = errors.New("username already exists")
	ErrEmailExists         = errors.New("email already exists")
	ErrInvalidToken        = errors.New("invalid or expired token")
	ErrUnauthorized        = errors.New("unauthorized access")
	ErrForbidden           = errors.New("forbidden access")
	ErrInvalidInput        = errors.New("invalid input")
)

// AppError is a custom application error
type AppError struct {
	Code       string
	Message    string
	Err        error
	StatusCode int
	Details    map[string]interface{}
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Err
}

// NewAppError creates a new application error
func NewAppError(code, message string, statusCode int, err error) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		Err:        err,
		StatusCode: statusCode,
		Details:    make(map[string]interface{}),
	}
}
