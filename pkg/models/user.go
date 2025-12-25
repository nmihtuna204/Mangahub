package models

import (
	"time"
)

// User represents a system user
type User struct {
	ID           string    `json:"id" db:"id"`
	Username     string    `json:"username" db:"username" validate:"required,min=3,max=50"`
	Email        string    `json:"email" db:"email" validate:"required,email"`
	PasswordHash string    `json:"-" db:"password_hash"`
	DisplayName  string    `json:"display_name" db:"display_name"`
	Role         string    `json:"role" db:"role"` // user, admin
	IsActive     bool      `json:"is_active" db:"is_active"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty" db:"last_login_at"`
}

// UserProfile is the public-facing user profile
type UserProfile struct {
	ID          string     `json:"id"`
	Username    string     `json:"username"`
	DisplayName string     `json:"display_name"`
	AvatarURL   string     `json:"avatar_url"`
	CreatedAt   time.Time  `json:"created_at"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
}

// RegisterRequest represents a user registration request
type RegisterRequest struct {
	Username string `json:"username" validate:"required,min=3,max=50"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8,max=100"`
}

// LoginRequest represents a login request
type LoginRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

// LoginResponse represents a successful login response
type LoginResponse struct {
	Token     string      `json:"token"`
	ExpiresAt time.Time   `json:"expires_at"`
	User      UserProfile `json:"user"`
}
