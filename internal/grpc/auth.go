package grpc

import (
	"context"
	"strings"

	ggrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"mangahub/internal/auth"
	pb "mangahub/internal/grpc/pb"
	"mangahub/pkg/models"
)

// protectedMethods need a valid JWT in the "authorization: Bearer <token>"
// metadata. Reads stay public, like GET /manga over HTTP.
var protectedMethods = map[string]bool{
	pb.MangaService_UpdateProgress_FullMethodName: true,
}

type callerKey struct{}

// AuthInterceptor verifies the JWT on protected RPCs (same secret and issuer
// as the HTTP API) and makes the caller available via CallerFromContext.
func AuthInterceptor(secret []byte, issuer string) ggrpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *ggrpc.UnaryServerInfo, handler ggrpc.UnaryHandler) (interface{}, error) {
		if !protectedMethods[info.FullMethod] {
			return handler(ctx, req)
		}
		token := bearerFromMetadata(ctx)
		if token == "" {
			return nil, status.Error(codes.Unauthenticated, `missing "authorization: Bearer <token>" metadata`)
		}
		user, err := auth.VerifyToken(token, secret, issuer)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid or expired token")
		}
		return handler(ContextWithCaller(ctx, user), req)
	}
}

// ContextWithCaller attaches the authenticated caller to a context.
func ContextWithCaller(ctx context.Context, user *models.UserProfile) context.Context {
	return context.WithValue(ctx, callerKey{}, user)
}

// CallerFromContext returns the authenticated caller, or nil.
func CallerFromContext(ctx context.Context) *models.UserProfile {
	user, _ := ctx.Value(callerKey{}).(*models.UserProfile)
	return user
}

func bearerFromMetadata(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	for _, v := range md.Get("authorization") {
		if parts := strings.SplitN(v, " ", 2); len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return strings.TrimSpace(parts[1])
		}
	}
	return ""
}
