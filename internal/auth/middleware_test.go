package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"

	"mangahub/internal/testutil"
	"mangahub/pkg/models"
	"mangahub/pkg/utils"
)

const testSecret = "test-secret"

func newService(t *testing.T) Service {
	t.Helper()
	db := testutil.NewDB(t)
	hash, _ := utils.HashPassword("password123")
	testutil.MustExec(t, db.DB, `INSERT INTO users (id, username, email, password_hash, display_name, role)
		VALUES ('a1', 'admin', 'admin@example.com', ?, 'Admin', 'admin'),
		       ('u1', 'reader', 'reader@example.com', ?, 'Reader', 'user'),
		       ('d1', 'disabled', 'disabled@example.com', ?, 'Disabled', 'user')`, hash, hash, hash)
	testutil.MustExec(t, db.DB, `UPDATE users SET is_active = 0 WHERE id = 'd1'`)
	return NewService(db.DB, testSecret, "mangahub", time.Hour)
}

func login(t *testing.T, svc Service, user string) string {
	t.Helper()
	resp, err := svc.Login(context.Background(), models.LoginRequest{Username: user, Password: "password123"})
	if err != nil {
		t.Fatalf("login %s: %v", user, err)
	}
	return resp.Token
}

func claims(t *testing.T, token string) map[string]interface{} {
	t.Helper()
	part := strings.Split(token, ".")[1]
	raw, err := base64.RawURLEncoding.DecodeString(part)
	if err != nil {
		t.Fatal(err)
	}
	var c map[string]interface{}
	json.Unmarshal(raw, &c)
	return c
}

func TestLoginRejectsDisabledAccount(t *testing.T) {
	svc := newService(t)
	_, err := svc.Login(context.Background(), models.LoginRequest{Username: "disabled", Password: "password123"})
	var appErr *models.AppError
	if !errors.As(err, &appErr) || appErr.StatusCode != 403 {
		t.Errorf("disabled login: got %v, want 403", err)
	}
	// Wrong password on a disabled account still says "invalid credentials"
	_, err = svc.Login(context.Background(), models.LoginRequest{Username: "disabled", Password: "nope"})
	if !errors.As(err, &appErr) || appErr.StatusCode != 401 {
		t.Errorf("disabled + wrong password: got %v, want 401", err)
	}
}

func TestRefreshKeepsRole(t *testing.T) {
	svc := newService(t)
	token, err := svc.RefreshToken(context.Background(), "a1")
	if err != nil {
		t.Fatal(err)
	}
	if role := claims(t, token)["role"]; role != "admin" {
		t.Errorf("refreshed role = %v, want admin (was downgraded to user)", role)
	}
}

func TestParseTokenRejectsForgedTokens(t *testing.T) {
	svc := newService(t)
	valid := login(t, svc, "reader")
	if _, err := svc.ParseToken(valid); err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}

	// alg=none with the same claims and no signature
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	none := header + "." + strings.Split(valid, ".")[1] + "."
	if _, err := svc.ParseToken(none); err == nil {
		t.Error("alg=none token accepted")
	}

	// Correct secret, wrong issuer
	wrongIssuer := jwt.NewWithClaims(jwt.SigningMethodHS256, jwtClaims{
		UserID:           "u1",
		RegisteredClaims: jwt.RegisteredClaims{Issuer: "someone-else", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
	})
	signed, _ := wrongIssuer.SignedString([]byte(testSecret))
	if _, err := svc.ParseToken(signed); err == nil {
		t.Error("token from another issuer accepted")
	}
}

func serve(handler gin.HandlerFunc, method, target, authHeader string) (*httptest.ResponseRecorder, *models.UserProfile) {
	gin.SetMode(gin.TestMode)
	var seen *models.UserProfile
	r := gin.New()
	r.Handle(method, "/x", handler, func(c *gin.Context) {
		seen = GetCurrentUser(c)
		c.Status(http.StatusOK)
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, target, nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	r.ServeHTTP(w, req)
	return w, seen
}

func TestMiddlewares(t *testing.T) {
	svc := newService(t)
	token := login(t, svc, "reader")

	cases := []struct {
		name     string
		mw       gin.HandlerFunc
		target   string
		header   string
		wantCode int
		wantUser bool
	}{
		{"JWT header", JWTMiddleware(svc), "/x", "Bearer " + token, 200, true},
		{"JWT missing", JWTMiddleware(svc), "/x", "", 401, false},
		{"JWT malformed", JWTMiddleware(svc), "/x", "Token " + token, 401, false},
		{"JWT ignores ?token=", JWTMiddleware(svc), "/x?token=" + token, "", 401, false},
		{"WS ?token=", WSAuthMiddleware(svc), "/x?token=" + token, "", 200, true},
		{"WS header", WSAuthMiddleware(svc), "/x", "Bearer " + token, 200, true},
		{"WS bad ?token=", WSAuthMiddleware(svc), "/x?token=garbage", "", 401, false},
		{"Optional anonymous", OptionalAuthMiddleware(svc), "/x", "", 200, false},
		{"Optional valid", OptionalAuthMiddleware(svc), "/x", "Bearer " + token, 200, true},
		{"Optional invalid", OptionalAuthMiddleware(svc), "/x", "Bearer garbage", 200, false},
	}
	for _, c := range cases {
		w, user := serve(c.mw, http.MethodGet, c.target, c.header)
		if w.Code != c.wantCode || (user != nil) != c.wantUser {
			t.Errorf("%s: code %d user %v; want %d, user set = %v", c.name, w.Code, user, c.wantCode, c.wantUser)
		}
		if c.wantUser && user != nil && user.ID != "u1" {
			t.Errorf("%s: user = %+v", c.name, user)
		}
	}
}
