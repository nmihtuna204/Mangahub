package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/yourusername/mangahub/pkg/models"
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

	svc := &mockAuthService{}
	handler := NewHandler(svc)
	router := gin.Default()
	router.POST("/auth/register", handler.Register)

	body := map[string]string{
		"username": "testuser",
		// Missing email and password
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

	svc := &mockAuthService{
		loginFunc: func(ctx context.Context, req models.LoginRequest) (*models.LoginResponse, error) {
			return nil, errors.New("invalid credentials")
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
	registerFunc func(ctx context.Context, req models.RegisterRequest) (*models.UserProfile, error)
	loginFunc    func(ctx context.Context, req models.LoginRequest) (*models.LoginResponse, error)
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
