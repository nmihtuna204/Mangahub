package protocols

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	pb "mangahub/internal/grpc/pb"
)

// ---- fakes for the four legs ----

type tcpSink struct {
	ln    net.Listener
	lines chan map[string]interface{}

	mu    sync.Mutex
	conns []net.Conn
}

// stop simulates the server process exiting: listener and connections close.
func (s *tcpSink) stop() {
	s.ln.Close()
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.conns {
		c.Close()
	}
}

func listenTCP(t *testing.T, addr string) *tcpSink {
	t.Helper()
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("listen tcp %s: %v", addr, err)
	}
	s := &tcpSink{ln: ln, lines: make(chan map[string]interface{}, 64)}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			s.mu.Lock()
			s.conns = append(s.conns, conn)
			s.mu.Unlock()
			go func(c net.Conn) {
				sc := bufio.NewScanner(c)
				for sc.Scan() {
					var m map[string]interface{}
					if json.Unmarshal(sc.Bytes(), &m) == nil {
						s.lines <- m
					}
				}
			}(conn)
		}
	}()
	t.Cleanup(s.stop)
	return s
}

type udpSink struct {
	conn  *net.UDPConn
	grams chan string
}

func listenUDP(t *testing.T) *udpSink {
	t.Helper()
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	s := &udpSink{conn: conn, grams: make(chan string, 64)}
	go func() {
		buf := make([]byte, 4096)
		for {
			n, _, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			s.grams <- string(buf[:n])
		}
	}()
	t.Cleanup(func() { conn.Close() })
	return s
}

type grpcCall struct {
	req           *pb.ProgressRequest
	audit         string
	authorization string
}

type fakeMangaService struct {
	pb.UnimplementedMangaServiceServer
	calls chan grpcCall
}

func (f *fakeMangaService) UpdateProgress(ctx context.Context, req *pb.ProgressRequest) (*pb.ProgressResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	audit := ""
	if v := md.Get(grpcAuditKey); len(v) > 0 {
		audit = v[0]
	}
	authz := ""
	if v := md.Get("authorization"); len(v) > 0 {
		authz = v[0]
	}
	f.calls <- grpcCall{req: req, audit: audit, authorization: authz}
	return &pb.ProgressResponse{}, nil
}

func listenGRPC(t *testing.T) (string, *fakeMangaService) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	svc := &fakeMangaService{calls: make(chan grpcCall, 128)}
	srv := grpc.NewServer()
	pb.RegisterMangaServiceServer(srv, svc)
	go srv.Serve(ln)
	t.Cleanup(srv.Stop)
	return ln.Addr().String(), svc
}

type roomCall struct{ roomID, userID, username, content, msgType string }

type fakeRooms struct{ calls chan roomCall }

func (f *fakeRooms) NotifyRoom(roomID, userID, username, content, msgType string) {
	f.calls <- roomCall{roomID, userID, username, content, msgType}
}

// freeAddr returns a loopback TCP address nothing is listening on.
func freeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	return addr
}

func recv[T any](t *testing.T, ch <-chan T, what string) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
	}
	var zero T
	return zero
}

var event = ProgressEvent{
	UserID: "u1", Username: "reader1", MangaID: "m1", MangaTitle: "One Piece", Chapter: 7, Status: "reading",
	Token: "user-jwt",
}

func TestBroadcastReachesAllFourLegs(t *testing.T) {
	tcp := listenTCP(t, "127.0.0.1:0")
	udp := listenUDP(t)
	grpcAddr, grpcSvc := listenGRPC(t)
	rooms := &fakeRooms{calls: make(chan roomCall, 8)}

	b, err := NewProtocolBridge(tcp.ln.Addr().String(), udp.conn.LocalAddr().String(), grpcAddr, rooms)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	b.BroadcastProgressUpdate(event)

	line := recv(t, tcp.lines, "TCP update")
	if line["manga_id"] != "m1" || line["chapter"] != float64(7) || line["user_id"] != "u1" {
		t.Errorf("TCP line = %v", line)
	}

	gram := recv(t, udp.grams, "UDP datagram")
	payload, ok := strings.CutPrefix(gram, "BROADCAST ")
	var n map[string]interface{}
	if !ok || json.Unmarshal([]byte(payload), &n) != nil {
		t.Fatalf("UDP datagram = %q, want BROADCAST <json>", gram)
	}
	if n["type"] != "progress_update" || n["manga_title"] != "One Piece" || n["chapter"] != float64(7) ||
		!strings.Contains(n["message"].(string), "reader1 is now on chapter 7 of One Piece") {
		t.Errorf("UDP notification = %v", n)
	}

	call := recv(t, grpcSvc.calls, "gRPC audit")
	if call.audit != "true" || call.req.CurrentChapter != 7 || call.req.Status != "reading" || call.req.MangaId != "m1" {
		t.Errorf("gRPC call = %+v (audit %q), want an audit-only UpdateProgress for chapter 7", call.req, call.audit)
	}
	if call.authorization != "Bearer user-jwt" {
		t.Errorf("gRPC authorization = %q, want the user's token forwarded", call.authorization)
	}

	room := recv(t, rooms.calls, "room notice")
	if room.roomID != "manga_m1" || room.msgType != "system" || !strings.Contains(room.content, "chapter 7") {
		t.Errorf("room notice = %+v", room)
	}
}

func TestDescribeCompleted(t *testing.T) {
	ev := event
	ev.Status = "completed"
	if got := describe(ev); got != "reader1 finished One Piece" {
		t.Errorf("describe = %q", got)
	}
	ev.MangaTitle = ""
	if got := describe(ev); !strings.Contains(got, "m1") {
		t.Errorf("describe without title = %q, want the manga ID", got)
	}
}

// Regression: the TCP leg was skipped forever when the TCP server wasn't up
// when the API started.
func TestTCPLegConnectsWhenServerStartsLate(t *testing.T) {
	addr := freeAddr(t)
	udp := listenUDP(t)
	b, _ := NewProtocolBridge(addr, udp.conn.LocalAddr().String(), freeAddr(t), nil)
	defer b.Close()

	if err := b.sendTCP([]byte("{}\n")); err == nil {
		t.Fatal("send to a closed port unexpectedly succeeded")
	}

	tcp := listenTCP(t, addr)
	b.BroadcastProgressUpdate(event)
	if line := recv(t, tcp.lines, "TCP update after late start"); line["manga_id"] != "m1" {
		t.Errorf("got %v", line)
	}
}

// Regression: after the TCP server restarted, every write failed.
func TestTCPLegRecoversAfterServerRestart(t *testing.T) {
	tcp := listenTCP(t, "127.0.0.1:0")
	addr := tcp.ln.Addr().String()
	udp := listenUDP(t)
	b, _ := NewProtocolBridge(addr, udp.conn.LocalAddr().String(), freeAddr(t), nil)
	defer b.Close()

	b.BroadcastProgressUpdate(event)
	recv(t, tcp.lines, "first update")

	// Restart: the old server process goes away, a new one listens on the same port
	tcp.stop()
	restarted := listenTCP(t, addr)

	deadline := time.After(5 * time.Second)
	for {
		b.BroadcastProgressUpdate(event)
		select {
		case <-restarted.lines:
			return
		case <-time.After(200 * time.Millisecond):
		case <-deadline:
			t.Fatal("no update reached the restarted TCP server")
		}
	}
}

// The gRPC leg must deliver audits in the order the updates happened.
func TestGRPCAuditsStayInOrder(t *testing.T) {
	grpcAddr, grpcSvc := listenGRPC(t)
	udp := listenUDP(t)
	b, _ := NewProtocolBridge(freeAddr(t), udp.conn.LocalAddr().String(), grpcAddr, nil)
	defer b.Close()

	for ch := int32(1); ch <= 30; ch++ {
		ev := event
		ev.Chapter = ch
		b.enqueueGRPC(ev)
	}
	for want := int32(1); want <= 30; want++ {
		call := recv(t, grpcSvc.calls, "gRPC audit")
		if call.req.CurrentChapter != want {
			t.Fatalf("audit %d arrived as chapter %d (out of order)", want, call.req.CurrentChapter)
		}
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	udp := listenUDP(t)
	b, _ := NewProtocolBridge(freeAddr(t), udp.conn.LocalAddr().String(), freeAddr(t), nil)
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); b.Close() }()
	}
	wg.Wait()
}
