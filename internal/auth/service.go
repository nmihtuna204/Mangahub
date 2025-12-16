// Package auth - Authentication Service
// Xử lý tất cả logic liên quan đến authentication và authorization
// Chức năng:
//   - User registration với password hashing (bcrypt)
//   - User login với JWT token generation
//   - Token validation và parsing
//   - Session management
package auth

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"mangahub/pkg/models"
	"mangahub/pkg/utils"
)

type Service interface {
	Register(ctx context.Context, req models.RegisterRequest) (*models.UserProfile, error)
	Login(ctx context.Context, req models.LoginRequest) (*models.LoginResponse, error)
	ParseToken(tokenStr string) (*models.UserProfile, error)
	RefreshToken(ctx context.Context, userID string) (string, error)
	GetUserByID(ctx context.Context, userID string) (*models.UserProfile, error)
}

type service struct {
	db        *sql.DB
	jwtSecret []byte
	issuer    string
	exp       time.Duration
}

type jwtClaims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

func NewService(db *sql.DB, secret, issuer string, exp time.Duration) Service {
	return &service{
		db:        db,
		jwtSecret: []byte(secret),
		issuer:    issuer,
		exp:       exp,
	}
}

func (s *service) Register(ctx context.Context, req models.RegisterRequest) (*models.UserProfile, error) {
	if err := utils.ValidateStruct(req); err != nil {
		return nil, models.NewAppError(models.ErrCodeValidation, "invalid registration data", 400, err)
	}

	var exists int
	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM users WHERE username = ? OR email = ?",
		req.Username, req.Email,
	).Scan(&exists)
	if err != nil {
		return nil, models.NewAppError(models.ErrCodeInternal, "failed checking user uniqueness", 500, err)
	}
	if exists > 0 {
		return nil, models.NewAppError(models.ErrCodeConflict, "username or email already exists", 409, models.ErrUsernameExists)
	}

	hash, err := utils.HashPassword(req.Password)
	if err != nil {
		return nil, models.NewAppError(models.ErrCodeInternal, "failed to hash password", 500, err)
	}

	now := time.Now()
	userID := uuid.New().String()

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO users (id, username, email, password_hash, display_name, role, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 'user', 1, ?, ?)`,
		userID, req.Username, req.Email, hash, req.Username, now, now,
	)
	if err != nil {
		return nil, models.NewAppError(models.ErrCodeInternal, "failed to create user", 500, err)
	}

	profile := &models.UserProfile{
		ID:          userID,
		Username:    req.Username,
		DisplayName: req.Username,
		Bio:         "",
		AvatarURL:   "",
		CreatedAt:   now,
	}

	return profile, nil
}

func (s *service) Login(ctx context.Context, req models.LoginRequest) (*models.LoginResponse, error) {
	if err := utils.ValidateStruct(req); err != nil {
		return nil, models.NewAppError(models.ErrCodeValidation, "invalid login data", 400, err)
	}

	var (
		id           string
		username     string
		email        string
		hash         string
		displayName  string
		role         string
		createdAt    time.Time
		lastLoginPtr *time.Time
	)

	err := s.db.QueryRowContext(ctx, `
		SELECT id, username, email, password_hash, display_name, role, created_at, last_login_at
		FROM users
		WHERE username = ? OR email = ?`,
		req.Username, req.Username,
	).Scan(&id, &username, &email, &hash, &displayName, &role, &createdAt, &lastLoginPtr)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, models.NewAppError(models.ErrCodeUnauthorized, "invalid credentials", 401, models.ErrInvalidCredentials)
		}
		return nil, models.NewAppError(models.ErrCodeInternal, "failed to query user", 500, err)
	}

	if !utils.CheckPassword(req.Password, hash) {
		return nil, models.NewAppError(models.ErrCodeUnauthorized, "invalid credentials", 401, models.ErrInvalidCredentials)
	}

	now := time.Now()
	expiresAt := now.Add(s.exp)

	claims := jwtClaims{
		UserID:   id,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   id,
			Issuer:    s.issuer,
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return nil, models.NewAppError(models.ErrCodeInternal, "failed to sign token", 500, err)
	}

	_, _ = s.db.ExecContext(ctx, "UPDATE users SET last_login_at = ?, updated_at = ? WHERE id = ?", now, now, id)

	profile := models.UserProfile{
		ID:          id,
		Username:    username,
		DisplayName: displayName,
		Bio:         "",
		AvatarURL:   "",
		CreatedAt:   createdAt,
		LastLoginAt: lastLoginPtr,
	}

	return &models.LoginResponse{
		Token:     tokenStr,
		ExpiresAt: expiresAt,
		User:      profile,
	}, nil
}

func (s *service) ParseToken(tokenStr string) (*models.UserProfile, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &jwtClaims{}, func(t *jwt.Token) (interface{}, error) {
		return s.jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return nil, models.NewAppError(models.ErrCodeUnauthorized, "invalid token", 401, models.ErrInvalidToken)
	}

	claims, ok := token.Claims.(*jwtClaims)
	if !ok {
		return nil, models.NewAppError(models.ErrCodeUnauthorized, "invalid token claims", 401, models.ErrInvalidToken)
	}

	return &models.UserProfile{
		ID:       claims.UserID,
		Username: claims.Username,
		// role can be added if you include it in UserProfile later
	}, nil
}

// RefreshToken generates a new JWT token for an existing user
func (s *service) RefreshToken(ctx context.Context, userID string) (string, error) {
	// Get user from DB to ensure they still exist and are active
	user, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return "", err
	}

	now := time.Now()
	expiresAt := now.Add(s.exp)

	claims := jwtClaims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     "user", // Default role
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID,
			Issuer:    s.issuer,
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", models.NewAppError(models.ErrCodeInternal, "failed to sign token", 500, err)
	}

	return tokenStr, nil
}

// GetUserByID retrieves a user profile by their ID
func (s *service) GetUserByID(ctx context.Context, userID string) (*models.UserProfile, error) {
	var (
		id          string
		username    string
		displayName string
		bio         sql.NullString
		avatarURL   sql.NullString
		createdAt   time.Time
		lastLogin   *time.Time
	)

	err := s.db.QueryRowContext(ctx, `
		SELECT id, username, display_name, bio, avatar_url, created_at, last_login_at
		FROM users
		WHERE id = ? AND is_active = 1`,
		userID,
	).Scan(&id, &username, &displayName, &bio, &avatarURL, &createdAt, &lastLogin)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, models.NewAppError(models.ErrCodeNotFound, "user not found", 404, nil)
		}
		return nil, models.NewAppError(models.ErrCodeInternal, "failed to query user", 500, err)
	}

	return &models.UserProfile{
		ID:          id,
		Username:    username,
		DisplayName: displayName,
		Bio:         bio.String,
		AvatarURL:   avatarURL.String,
		CreatedAt:   createdAt,
		LastLoginAt: lastLogin,
	}, nil
}
