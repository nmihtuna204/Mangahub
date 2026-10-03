package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	ggrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pb "mangahub/internal/grpc/pb"
	"mangahub/pkg/models"
)

var testSecret = []byte("grpc-test-secret")

func token(t *testing.T, issuer string, secret []byte, expires time.Time) string {
	t.Helper()
	claims := jwt.MapClaims{"user_id": "u1", "username": "reader1", "role": "user", "iss": issuer, "exp": expires.Unix()}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// call runs the interceptor for a method with the given authorization metadata.
func call(t *testing.T, method, authorization string) (*models.UserProfile, error) {
	t.Helper()
	ctx := context.Background()
	if authorization != "" {
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", authorization))
	}
	var caller *models.UserProfile
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		caller = CallerFromContext(ctx)
		return "ok", nil
	}
	_, err := AuthInterceptor(testSecret, "mangahub")(ctx, nil, &ggrpc.UnaryServerInfo{FullMethod: method}, handler)
	return caller, err
}

func TestAuthInterceptor(t *testing.T) {
	update := pb.MangaService_UpdateProgress_FullMethodName
	valid := token(t, "mangahub", testSecret, time.Now().Add(time.Hour))

	caller, err := call(t, update, "Bearer "+valid)
	if err != nil || caller == nil || caller.ID != "u1" || caller.Role != "user" {
		t.Errorf("valid token: caller %+v, err %v", caller, err)
	}

	for name, authz := range map[string]string{
		"missing":       "",
		"not bearer":    "Basic " + valid,
		"wrong secret":  "Bearer " + token(t, "mangahub", []byte("other"), time.Now().Add(time.Hour)),
		"wrong issuer":  "Bearer " + token(t, "someone-else", testSecret, time.Now().Add(time.Hour)),
		"expired":       "Bearer " + token(t, "mangahub", testSecret, time.Now().Add(-time.Minute)),
		"garbage token": "Bearer not.a.jwt",
	} {
		if _, err := call(t, update, authz); codeOf(err) != codes.Unauthenticated {
			t.Errorf("%s: got %v, want Unauthenticated", name, err)
		}
	}

	// Reads stay public
	for _, m := range []string{pb.MangaService_GetManga_FullMethodName, pb.MangaService_SearchManga_FullMethodName} {
		if _, err := call(t, m, ""); err != nil {
			t.Errorf("%s without a token: %v", m, err)
		}
	}
}

func codeOf(err error) codes.Code { return status.Code(err) }
