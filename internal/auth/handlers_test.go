package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"mangahub/pkg/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRegister(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create a mock service
	svc := &mockAuthService{
		registerFunc: func(ctx context.Context, req models.RegisterRequest) (*models.UserProfile, error) {
			return &models.UserProfile{
				ID:       "user-123",
				Username: req.Username,
			}, nil
		},
	}

	handler := NewHandler(svc)
	router := gin.Default()
	router.POST("/auth/register", handler.Register)

	body := map[string]string{
		"username": "testuser",
		"email":    "test@example.com",
		"password": "password123",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/auth/register", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, true, resp["success"])

	data := resp["data"].(map[string]interface{})
	assert.Equal(t, "user-123", data["id"])
	assert.Equal(t, "testuser", data["username"])
}

func TestRegisterMissingFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Mock service that returns validation error
	svc := &mockAuthService{
		registerFunc: func(ctx context.Context, req models.RegisterRequest) (*models.UserProfile, error) {
			// Simulate service-layer validation failure
			return nil, &models.AppError{
				StatusCode: http.StatusBadRequest,
				Code:       models.ErrCodeBadRequest,
				Message:    "validation failed",
			}
		},
	}

	handler := NewHandler(svc)
	router := gin.Default()
	router.POST("/auth/register", handler.Register)

	body := map[string]string{
		"username": "testuser",
		"email":    "", // Empty email
		"password": "", // Empty password
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/auth/register", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, false, resp["success"])
}

func TestLoginSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := &mockAuthService{
		loginFunc: func(ctx context.Context, req models.LoginRequest) (*models.LoginResponse, error) {
			return &models.LoginResponse{
				User: models.UserProfile{
					ID:       "user-123",
					Username: req.Username,
				},
				Token: "mock-jwt-token",
			}, nil
		},
	}

	handler := NewHandler(svc)
	router := gin.Default()
	router.POST("/auth/login", handler.Login)

	body := map[string]string{
		"username": "testuser",
		"password": "password123",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/auth/login", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, true, resp["success"])

	data := resp["data"].(map[string]interface{})
	user := data["user"].(map[string]interface{})
	assert.Equal(t, "user-123", user["id"])
	assert.Equal(t, "testuser", user["username"])
	assert.Equal(t, "mock-jwt-token", data["token"])
}

func TestLoginFail(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Mock service that returns authentication error
	svc := &mockAuthService{
		loginFunc: func(ctx context.Context, req models.LoginRequest) (*models.LoginResponse, error) {
			// Return proper AppError for invalid credentials
			return nil, &models.AppError{
				StatusCode: http.StatusUnauthorized,
				Code:       models.ErrCodeUnauthorized,
				Message:    "invalid credentials",
			}
		},
	}

	handler := NewHandler(svc)
	router := gin.Default()
	router.POST("/auth/login", handler.Login)

	body := map[string]string{
		"username": "nonexistent",
		"password": "wrong",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/auth/login", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, false, resp["success"])
}

// Mock service for testing
type mockAuthService struct {
	registerFunc     func(ctx context.Context, req models.RegisterRequest) (*models.UserProfile, error)
	loginFunc        func(ctx context.Context, req models.LoginRequest) (*models.LoginResponse, error)
	refreshTokenFunc func(ctx context.Context, userID string) (string, error)
	getUserByIDFunc  func(ctx context.Context, userID string) (*models.UserProfile, error)
}

func (m *mockAuthService) Register(ctx context.Context, req models.RegisterRequest) (*models.UserProfile, error) {
	if m.registerFunc != nil {
		return m.registerFunc(ctx, req)
	}
	return nil, nil
}

func (m *mockAuthService) Login(ctx context.Context, req models.LoginRequest) (*models.LoginResponse, error) {
	if m.loginFunc != nil {
		return m.loginFunc(ctx, req)
	}
	return nil, nil
}

func (m *mockAuthService) ParseToken(token string) (*models.UserProfile, error) {
	return nil, nil
}

func (m *mockAuthService) RefreshToken(ctx context.Context, userID string) (string, error) {
	if m.refreshTokenFunc != nil {
		return m.refreshTokenFunc(ctx, userID)
	}
	return "new-mock-token", nil
}

func (m *mockAuthService) GetUserByID(ctx context.Context, userID string) (*models.UserProfile, error) {
	if m.getUserByIDFunc != nil {
		return m.getUserByIDFunc(ctx, userID)
	}
	return &models.UserProfile{ID: userID, Username: "testuser"}, nil
}

// Helper to set up authenticated context
func setupAuthenticatedRouter(handler *Handler) *gin.Engine {
	router := gin.Default()
	// Simulate middleware setting user in context
	router.Use(func(c *gin.Context) {
		user := &models.UserProfile{
			ID:       "user-123",
			Username: "testuser",
		}
		c.Set(string(ContextUserKey), user)
		c.Next()
	})
	return router
}

func TestGetMe(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := &mockAuthService{}
	handler := NewHandler(svc)
	router := setupAuthenticatedRouter(handler)
	router.GET("/auth/me", handler.GetMe)

	req := httptest.NewRequest("GET", "/auth/me", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, true, resp["success"])

	data := resp["data"].(map[string]interface{})
	assert.Equal(t, "user-123", data["id"])
	assert.Equal(t, "testuser", data["username"])
}

func TestGetMeUnauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := &mockAuthService{}
	handler := NewHandler(svc)
	router := gin.Default()
	router.GET("/auth/me", handler.GetMe)

	req := httptest.NewRequest("GET", "/auth/me", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestLogout(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := &mockAuthService{}
	handler := NewHandler(svc)
	router := setupAuthenticatedRouter(handler)
	router.POST("/auth/logout", handler.Logout)

	req := httptest.NewRequest("POST", "/auth/logout", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, true, resp["success"])
	assert.Contains(t, resp["message"], "logout")
}

func TestRefreshToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := &mockAuthService{
		refreshTokenFunc: func(ctx context.Context, userID string) (string, error) {
			return "refreshed-token-abc", nil
		},
	}
	handler := NewHandler(svc)
	router := setupAuthenticatedRouter(handler)
	router.POST("/auth/refresh", handler.RefreshToken)

	req := httptest.NewRequest("POST", "/auth/refresh", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, true, resp["success"])

	data := resp["data"].(map[string]interface{})
	assert.Equal(t, "refreshed-token-abc", data["token"])
	assert.Equal(t, "user-123", data["user_id"])
}

func TestRefreshTokenUnauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := &mockAuthService{}
	handler := NewHandler(svc)
	router := gin.Default()
	router.POST("/auth/refresh", handler.RefreshToken)

	req := httptest.NewRequest("POST", "/auth/refresh", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
