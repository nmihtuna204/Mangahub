package network

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gorilla/websocket"

	"mangahub/internal/udp"
)

// run executes a Bubble Tea command with a timeout.
func run(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	if cmd == nil {
		return nil
	}
	ch := make(chan tea.Msg, 1)
	go func() { ch <- cmd() }()
	select {
	case m := <-ch:
		return m
	case <-time.After(5 * time.Second):
		t.Fatal("command timed out")
		return nil
	}
}

// Regression: the server sends unix-second timestamps; decoding them into
// time.Time failed and every chat message was shown as raw JSON.
func TestChatMessageTimestamps(t *testing.T) {
	cases := map[string]time.Time{
		`{"content":"a","timestamp":1790733468}`:             time.Unix(1790733468, 0),
		`{"content":"a","timestamp":"2026-09-30T10:00:00Z"}`: time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC),
	}
	for raw, want := range cases {
		var m ChatMessageMsg
		if err := json.Unmarshal([]byte(raw), &m); err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
		if !m.Timestamp.Equal(want) || m.Content != "a" {
			t.Errorf("%s: got %v %q", raw, m.Timestamp, m.Content)
		}
	}
	var m ChatMessageMsg
	json.Unmarshal([]byte(`{"content":"a","username":"bob","type":"join"}`), &m)
	if m.Timestamp.IsZero() || m.Username != "bob" || m.Type != "join" {
		t.Errorf("missing timestamp: %+v", m)
	}
}

func TestFormatNotification(t *testing.T) {
	cases := []struct {
		msg  UDPNotificationMsg
		want string
	}{
		// chapter numbers >= 10 used to render as a single rune
		{UDPNotificationMsg{Type: "chapter_release", MangaName: "One Piece", Chapter: 1100}, "Chapter 1100 released"},
		{UDPNotificationMsg{Type: "chapter_release", Content: "New chapter!"}, "New chapter!"},
		{UDPNotificationMsg{Type: "progress_update", Content: "reader1 is now on chapter 7"}, "reader1 is now on chapter 7"},
		{UDPNotificationMsg{Type: "system", Title: "Maintenance", Content: "tonight"}, "Maintenance: tonight"},
	}
	for _, c := range cases {
		if got := FormatNotification(c.msg); !strings.Contains(got, c.want) {
			t.Errorf("FormatNotification(%+v) = %q, want it to contain %q", c.msg, got, c.want)
		}
	}
}

// fakeUDPServer answers REGISTER and lets the test push datagrams to the subscriber.
type fakeUDPServer struct {
	conn       *net.UDPConn
	subscriber chan *net.UDPAddr
	registers  chan string // every REGISTER line received
}

func startFakeUDPServer(t *testing.T) *fakeUDPServer {
	t.Helper()
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	s := &fakeUDPServer{conn: conn, subscriber: make(chan *net.UDPAddr, 4), registers: make(chan string, 8)}
	go func() {
		buf := make([]byte, 1024)
		for {
			n, addr, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			msg := string(buf[:n])
			if !strings.HasPrefix(msg, "REGISTER") {
				continue
			}
			s.registers <- msg
			if msg == "REGISTER revoked-token" {
				conn.WriteToUDP([]byte("ERROR invalid token"), addr)
				continue
			}
			conn.WriteToUDP([]byte("REGISTERED"), addr)
			s.subscriber <- addr
		}
	}()
	t.Cleanup(func() { conn.Close() })
	return s
}

// Regression: the listener bound the server's own port and never registered,
// so it could never receive anything.
func TestUDPListener(t *testing.T) {
	server := startFakeUDPServer(t)
	l := NewUDPListener()

	msg := run(t, l.Start(server.conn.LocalAddr().String(), ""))
	if _, ok := msg.(UDPConnectedMsg); !ok {
		t.Fatalf("Start = %#v, want UDPConnectedMsg", msg)
	}
	sub := <-server.subscriber

	wait := make(chan tea.Msg, 1)
	go func() { wait <- run(t, l.WaitForPacket()) }()
	server.conn.WriteToUDP([]byte("REGISTERED"), sub) // confirmation echoes are skipped
	server.conn.WriteToUDP([]byte(`{"type":"progress_update","manga_id":"m1","message":"hi","timestamp":1790733468,"manga_title":"One Piece","chapter":7}`), sub)

	n, ok := (<-wait).(UDPNotificationMsg)
	if !ok || n.Type != "progress_update" || n.Content != "hi" || n.MangaName != "One Piece" || n.Chapter != 7 || !n.Timestamp.Equal(time.Unix(1790733468, 0)) {
		t.Errorf("notification = %+v", n)
	}

	// Stop twice (logout twice) must not panic, and a new Start must work
	run(t, l.Stop())
	run(t, l.Stop())
	if l.IsActive() {
		t.Error("still active after Stop")
	}
	if _, ok := run(t, l.Start(server.conn.LocalAddr().String(), "")).(UDPConnectedMsg); !ok {
		t.Error("restart after Stop failed")
	}
	run(t, l.Stop())
	if _, ok := run(t, l.WaitForPacket()).(UDPDisconnectedMsg); !ok {
		t.Error("WaitForPacket after Stop should report disconnected")
	}
}

func TestUDPListenerUnreachableServer(t *testing.T) {
	conn, _ := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	addr := conn.LocalAddr().String()
	conn.Close() // nothing listens here any more

	if _, ok := run(t, NewUDPListener().Start(addr, "")).(UDPErrorMsg); !ok {
		t.Error("expected UDPErrorMsg for an unreachable server")
	}
}

// nextConn waits for the fake server's next connection instead of hanging.
func nextConn(t *testing.T, conns chan *websocket.Conn) *websocket.Conn {
	t.Helper()
	select {
	case c := <-conns:
		return c
	case <-time.After(5 * time.Second):
		t.Fatal("the fake chat server got no connection")
		return nil
	}
}

// fake chat server: echoes each client message to the client
func startFakeChatServer(t *testing.T) (url string, conns chan *websocket.Conn, closed chan *websocket.Conn) {
	t.Helper()
	conns = make(chan *websocket.Conn, 8)
	closed = make(chan *websocket.Conn, 8)
	upgrader := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		conns <- c
		go func() {
			for {
				var in map[string]string
				if c.ReadJSON(&in) != nil {
					closed <- c
					return
				}
				c.WriteJSON(map[string]interface{}{
					"username": "alice", "content": in["content"], "type": "message",
					"timestamp": 1790733468, "room_id": r.URL.Query().Get("room_id"),
				})
			}
		}()
	}))
	t.Cleanup(srv.Close)
	return "ws" + strings.TrimPrefix(srv.URL, "http"), conns, closed
}

func TestWSClientLifecycle(t *testing.T) {
	url, conns, closed := startFakeChatServer(t)
	c := NewWSClient()

	if _, ok := run(t, c.Connect(url, "tok", "general")).(WSConnectedMsg); !ok {
		t.Fatal("connect failed")
	}
	first := nextConn(t, conns)

	run(t, c.SendMessage("general", "hello"))
	m, ok := run(t, c.ListenForMessages()).(ChatMessageMsg)
	if !ok || m.Content != "hello" || m.RoomID != "general" || m.Timestamp.Unix() != 1790733468 {
		t.Fatalf("echo = %+v", m)
	}

	// Reconnecting replaces the old connection instead of stacking a second one
	pending := c.ListenForMessages() // listener tied to the old connection
	if _, ok := run(t, c.Connect(url, "tok", "manga_m1")).(WSConnectedMsg); !ok {
		t.Fatal("reconnect failed")
	}
	nextConn(t, conns)
	if msg := run(t, pending); msg != nil {
		t.Errorf("old listener after an intentional replace returned %#v, want nil", msg)
	}
	select {
	case c := <-closed:
		if c != first {
			t.Error("the wrong connection was closed")
		}
	case <-time.After(2 * time.Second):
		t.Error("old connection is still open")
	}
	if !c.IsConnectedTo("manga_m1") {
		t.Error("not connected to the new room")
	}
}

func TestWSClientDropAndDisconnect(t *testing.T) {
	url, conns, _ := startFakeChatServer(t)
	c := NewWSClient()
	run(t, c.Connect(url, "tok", "general"))
	serverSide := nextConn(t, conns)

	listen := c.ListenForMessages()
	serverSide.Close() // server goes away
	if _, ok := run(t, listen).(WSDisconnectedMsg); !ok {
		t.Error("a dropped connection should produce WSDisconnectedMsg")
	}

	// Reconnect works after a drop
	run(t, c.Connect(url, "tok", "general"))
	nextConn(t, conns)

	// After an intentional disconnect (logout), no auto-reconnect happens
	run(t, c.Disconnect())
	if c.IsConnected() {
		t.Error("still connected after Disconnect")
	}
	if msg := run(t, c.Reconnect()); msg != nil {
		t.Errorf("Reconnect after Disconnect = %#v, want nil", msg)
	}

	if _, ok := run(t, c.Connect(url, "wrong-token", "general")).(WSErrorMsg); !ok {
		t.Error("connecting with a bad token should fail")
	}
}

// Logged-in users register with their token so the server can send them
// notifications meant only for them (new chapters of manga they read).
func TestUDPListenerRegistersWithToken(t *testing.T) {
	server := startFakeUDPServer(t)
	addr := server.conn.LocalAddr().String()

	l := NewUDPListener()
	if _, ok := run(t, l.Start(addr, "jwt-123")).(UDPConnectedMsg); !ok {
		t.Fatal("registration with a token failed")
	}
	if got := <-server.registers; got != "REGISTER jwt-123" {
		t.Errorf("sent %q, want \"REGISTER jwt-123\"", got)
	}
	run(t, l.Stop())

	msg, ok := run(t, NewUDPListener().Start(addr, "revoked-token")).(UDPErrorMsg)
	if !ok || !strings.Contains(msg.Err.Error(), "invalid token") {
		t.Errorf("rejected token: %#v, want UDPErrorMsg with the server's reason", msg)
	}
}

// The listener re-sends REGISTER as a heartbeat so the server doesn't expire
// it; when the server refuses its (expired) token it falls back to an
// anonymous subscription and says so once.
func TestUDPListenerHeartbeat(t *testing.T) {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	var expired atomic.Bool
	server := udp.NewNotificationServer("127.0.0.1", 0)
	// Generous margins (TTL = 6 heartbeats) so a loaded machine doesn't miss beats
	server.SetSubscriberTTL(300 * time.Millisecond)
	server.SetTokenVerifier(func(token string) (string, error) {
		if expired.Load() {
			return "", errors.New("token is expired")
		}
		return "reader1", nil
	})
	go server.Serve(conn)
	defer server.Stop()

	l := NewUDPListener()
	l.heartbeat = 50 * time.Millisecond
	if _, ok := run(t, l.Start(conn.LocalAddr().String(), "jwt-1")).(UDPConnectedMsg); !ok {
		t.Fatal("start failed")
	}
	defer run(t, l.Stop())

	time.Sleep(700 * time.Millisecond) // more than two TTLs
	if n := server.SubscriberCount(); n != 1 {
		t.Fatalf("subscribers = %d, want the heartbeating listener kept", n)
	}

	expired.Store(true)
	n, ok := run(t, l.WaitForPacket()).(UDPNotificationMsg)
	if !ok || n.Type != "system" || !strings.Contains(n.Content, "Login expired") {
		t.Fatalf("after the token expired: %+v, want one system notice", n)
	}
	time.Sleep(700 * time.Millisecond)
	if n := server.SubscriberCount(); n != 1 {
		t.Errorf("subscribers = %d, want the anonymous re-registration kept", n)
	}

	server.Broadcast <- udp.NewProgressNotification("m1", "One Piece", "reader1", 7, "still here")
	got, ok := run(t, l.WaitForPacket()).(UDPNotificationMsg)
	if !ok || got.Content != "still here" {
		t.Errorf("broadcast after fallback = %+v", got)
	}
}
