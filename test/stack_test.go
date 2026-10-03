package test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"mangahub/internal/auth"
	grpcpkg "mangahub/internal/grpc"
	pb "mangahub/internal/grpc/pb"
	"mangahub/internal/server"
	"mangahub/internal/tcp"
	"mangahub/internal/testutil"
	"mangahub/internal/udp"
	"mangahub/pkg/config"
	"mangahub/pkg/database"
	"mangahub/pkg/logger"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	logger.Init(logger.Config{Level: "error", Format: "text", Output: "stdout"})
	os.Exit(m.Run())
}

const jwtSecret = "e2e-secret"

// stack is the whole system running in-process: a seeded SQLite database,
// the TCP, UDP and gRPC servers, and the HTTP API (with the WebSocket hub and
// protocol bridge) built exactly as cmd/api-server builds it.
type stack struct {
	t        *testing.T
	db       *database.DB
	base     string // http://127.0.0.1:port
	tcpAddr  string
	udpAddr  string
	grpcAddr string
	tcp      *tcp.ProgressSyncServer
	udp      *udp.NotificationServer
	audits   *int32 // gRPC UpdateProgress calls carrying the bridge's audit metadata
}

// startStack starts everything; options can adjust the API config (rate
// limits are off unless an option turns them on).
func startStack(t *testing.T, options ...func(*server.Config)) *stack {
	t.Helper()
	s := &stack{t: t, db: testutil.NewSeededDB(t), audits: new(int32)}

	tcpLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s.tcp = tcp.NewProgressSyncServer("127.0.0.1", 0)
	go s.tcp.Serve(tcpLn)
	s.tcpAddr = tcpLn.Addr().String()

	udpConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	s.udp = udp.NewNotificationServer("127.0.0.1", 0)
	s.udp.SetTokenVerifier(func(token string) (string, error) { // as cmd/udp-server does
		user, err := auth.VerifyToken(token, []byte(jwtSecret), "mangahub")
		if err != nil {
			return "", err
		}
		return user.ID, nil
	})
	go s.udp.Serve(udpConn)
	s.udpAddr = udpConn.LocalAddr().String()

	grpcLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	countAudits := func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, h grpc.UnaryHandler) (interface{}, error) {
		if md, ok := metadata.FromIncomingContext(ctx); ok && len(md.Get(grpcpkg.AuditMetadataKey)) > 0 {
			atomic.AddInt32(s.audits, 1)
		}
		return h(ctx, req)
	}
	grpcSrv := grpc.NewServer(grpc.ChainUnaryInterceptor(
		countAudits,
		grpcpkg.AuthInterceptor([]byte(jwtSecret), "mangahub"), // as cmd/grpc-server does
	))
	pb.RegisterMangaServiceServer(grpcSrv, grpcpkg.NewMangaServiceServer(s.db.DB))
	go grpcSrv.Serve(grpcLn)
	s.grpcAddr = grpcLn.Addr().String()

	apiCfg := server.Config{
		JWT:      config.JWTConfig{Secret: jwtSecret, Issuer: "mangahub", Expiration: time.Hour},
		TCPAddr:  s.tcpAddr,
		UDPAddr:  s.udpAddr,
		GRPCAddr: s.grpcAddr,
	}
	for _, opt := range options {
		opt(&apiCfg)
	}
	api := server.New(s.db, apiCfg)
	httpSrv := httptest.NewServer(api.Router)
	s.base = httpSrv.URL

	t.Cleanup(func() {
		httpSrv.Close()
		api.Close()
		grpcSrv.Stop()
		s.tcp.Stop()
		s.udp.Stop()
	})
	return s
}

func (s *stack) wsURL(path string) string {
	return "ws" + strings.TrimPrefix(s.base, "http") + path
}

// call sends a JSON request and decodes the JSON response.
func (s *stack) call(method, path string, body interface{}, token string) (int, map[string]interface{}) {
	s.t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, s.base+path, r)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		s.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	var out map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

// mustCall is call that fails the test unless the status matches.
func (s *stack) mustCall(want int, method, path string, body interface{}, token string) map[string]interface{} {
	s.t.Helper()
	code, out := s.call(method, path, body, token)
	if code != want {
		s.t.Fatalf("%s %s: status %d, want %d (%v)", method, path, code, want, out)
	}
	return out
}

func (s *stack) login(user, password string) string {
	s.t.Helper()
	out := s.mustCall(200, "POST", "/auth/login", map[string]string{"username": user, "password": password}, "")
	return data(out)["token"].(string)
}

// manga returns the seeded catalog in title order.
func (s *stack) manga() []map[string]interface{} {
	s.t.Helper()
	out := s.mustCall(200, "GET", "/manga?limit=100", nil, "")
	return list(data(out)["data"])
}

// entry returns the user's library entry for a manga (nil if absent).
func (s *stack) entry(token, mangaID string) map[string]interface{} {
	s.t.Helper()
	out := s.mustCall(200, "GET", "/users/library", nil, token)
	for _, e := range list(out["data"]) {
		if e["manga_id"] == mangaID {
			return e
		}
	}
	return nil
}

func data(out map[string]interface{}) map[string]interface{} {
	d, _ := out["data"].(map[string]interface{})
	return d
}

func list(v interface{}) []map[string]interface{} {
	raw, _ := v.([]interface{})
	out := make([]map[string]interface{}, 0, len(raw))
	for _, item := range raw {
		if m, ok := item.(map[string]interface{}); ok {
			out = append(out, m)
		}
	}
	return out
}

// eventually polls cond until it holds or the timeout passes.
func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
