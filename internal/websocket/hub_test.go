package websocket

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	gws "github.com/gorilla/websocket"

	"mangahub/internal/auth"
	"mangahub/internal/chat"
	"mangahub/pkg/models"
)

// fakeRepo records persistence calls; unused Repository methods panic.
type fakeRepo struct {
	chat.Repository
	mu       sync.Mutex
	ensured  map[string]int
	messages []chat.Message
	delay    time.Duration // per SaveMessage, to simulate a slow database
}

func (f *fakeRepo) EnsureRoom(ctx context.Context, roomID, ownerID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ensured[roomID]++
	return nil
}

func (f *fakeRepo) SaveMessage(ctx context.Context, msg *chat.Message) error {
	time.Sleep(f.delay)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.messages = append(f.messages, *msg)
	return nil
}

func (f *fakeRepo) snapshot() (map[string]int, []chat.Message) {
	f.mu.Lock()
	defer f.mu.Unlock()
	ensured := map[string]int{}
	for k, v := range f.ensured {
		ensured[k] = v
	}
	return ensured, append([]chat.Message(nil), f.messages...)
}

type testServer struct {
	hub  *Hub
	repo *fakeRepo
	url  string
}

// newTestServer serves ServeWS with a stand-in auth middleware that takes the
// user from ?user=.
func newTestServer(t *testing.T) *testServer {
	t.Helper()
	gin.SetMode(gin.TestMode)
	hub := NewHub()
	repo := &fakeRepo{ensured: map[string]int{}}
	hub.SetChatRepository(repo)
	go hub.Run()

	r := gin.New()
	r.GET("/ws", func(c *gin.Context) {
		u := c.Query("user")
		c.Set(auth.ContextUserKey, &models.UserProfile{ID: "id-" + u, Username: u})
	}, NewHandler(hub).ServeWS)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return &testServer{hub: hub, repo: repo, url: "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"}
}

func (s *testServer) dial(t *testing.T, user, room string) *gws.Conn {
	t.Helper()
	conn, _, err := gws.DefaultDialer.Dial(s.url+"?user="+user+"&room_id="+room, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func read(t *testing.T, conn *gws.Conn) RoomMessage {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	var m RoomMessage
	if err := conn.ReadJSON(&m); err != nil {
		t.Fatalf("read: %v", err)
	}
	return m
}

func expectNothing(t *testing.T, conn *gws.Conn) {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	var m RoomMessage
	if err := conn.ReadJSON(&m); err == nil {
		t.Errorf("unexpected message %+v", m)
	}
}

func TestChatRoomsAndMessageRules(t *testing.T) {
	s := newTestServer(t)
	alice := s.dial(t, "alice", "r1")
	if m := read(t, alice); m.Type != "join" || m.Username != "alice" {
		t.Fatalf("first message = %+v, want alice's join notice", m)
	}
	bob := s.dial(t, "bob", "r1")
	read(t, alice) // bob joined
	read(t, bob)
	carol := s.dial(t, "carol", "r2")
	read(t, carol)

	// Client-chosen types are ignored: nobody can post fake join/system notices
	alice.WriteJSON(map[string]string{"content": "  hello  ", "type": "system"})
	for _, c := range []*gws.Conn{alice, bob} {
		if m := read(t, c); m.Type != "message" || m.Content != "hello" || m.RoomID != "r1" {
			t.Errorf("got %+v, want trimmed 'hello' as a normal message in r1", m)
		}
	}
	expectNothing(t, carol) // other rooms don't see it

	// Empty messages are dropped; long ones are capped (the frame limit used to be 512 bytes)
	alice.WriteJSON(map[string]string{"content": "   "})
	alice.WriteJSON(map[string]string{"content": strings.Repeat("é", 2500)})
	if m := read(t, bob); len([]rune(m.Content)) != maxContentRunes {
		t.Errorf("long message delivered with %d runes, want %d", len([]rune(m.Content)), maxContentRunes)
	}
	read(t, alice)

	var ensured map[string]int
	var msgs []chat.Message
	deadline := time.Now().Add(3 * time.Second)
	for {
		ensured, msgs = s.repo.snapshot()
		if len(msgs) >= 2 || time.Now().After(deadline) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if ensured["r1"] != 1 || len(msgs) != 2 || msgs[0].Content != "hello" || msgs[0].UserID != "id-alice" {
		t.Errorf("persistence: ensured %v, messages %+v; want room r1 created once and 2 saved messages", ensured, msgs)
	}
}

func TestNotifyRoom(t *testing.T) {
	s := newTestServer(t)
	in := s.dial(t, "alice", "manga_m1")
	read(t, in)
	out := s.dial(t, "bob", "general")
	read(t, out)

	s.hub.NotifyRoom("manga_m1", "u1", "reader1", "reader1 is now on chapter 7", "system")
	if m := read(t, in); m.Type != "system" || m.Content != "reader1 is now on chapter 7" || m.RoomID != "manga_m1" {
		t.Errorf("got %+v", m)
	}
	expectNothing(t, out)

	// Notices to empty rooms are simply dropped
	s.hub.NotifyRoom("nobody-here", "u1", "reader1", "x", "system")

	if _, msgs := s.repo.snapshot(); len(msgs) != 0 {
		t.Errorf("server notices were persisted as chat messages: %+v", msgs)
	}
}

func TestLeaveNoticeAndRoomInfo(t *testing.T) {
	s := newTestServer(t)
	alice := s.dial(t, "alice", "r1")
	read(t, alice)
	bob := s.dial(t, "bob", "r1")
	read(t, alice)
	read(t, bob)

	if got := s.hub.GetRoomClients("r1"); len(got) != 2 {
		t.Errorf("room clients = %v, want 2", got)
	}
	bob.Close()
	if m := read(t, alice); m.Type != "leave" || m.Username != "bob" {
		t.Errorf("got %+v, want bob's leave notice", m)
	}
}

// Regression: when a client's send buffer was full, the broadcast path closed
// client.send and unregister closed it again, panicking the hub goroutine
// (which takes down the whole API server).
func TestSlowClientIsDroppedWithoutPanic(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	slow := &Client{hub: hub, send: make(chan RoomMessage, 1), userID: "u", username: "slow", roomID: "r"}
	hub.register <- slow // its own join notice fills the buffer

	for i := 0; i < 5; i++ {
		hub.NotifyRoom("r", "u1", "reader1", "notice", "system")
	}

	deadline := time.Now().Add(3 * time.Second)
	for len(hub.GetRoomClients("r")) != 0 {
		if time.Now().After(deadline) {
			t.Fatal("slow client was never removed")
		}
		time.Sleep(20 * time.Millisecond)
	}
	// send was closed exactly once: draining ends instead of blocking
	for range slow.send {
	}
}

// Shutdown: clients get a "going away" close frame instead of a dropped
// connection, chat messages still queued are saved before Stop returns (the
// database is closed right after), and late connections are turned away
// instead of hanging.
func TestStopClosesClientsAndSavesQueuedMessages(t *testing.T) {
	s := newTestServer(t)
	s.repo.delay = 30 * time.Millisecond
	alice := s.dial(t, "alice", "r1")
	read(t, alice)

	for i := 0; i < 5; i++ {
		alice.WriteJSON(map[string]string{"content": fmt.Sprintf("m%d", i)})
	}
	for i := 0; i < 5; i++ {
		read(t, alice) // broadcast, so queued for saving
	}
	s.hub.Stop()
	if _, msgs := s.repo.snapshot(); len(msgs) != 5 {
		t.Errorf("saved %d messages by the time Stop returned, want 5", len(msgs))
	}

	alice.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, _, err := alice.ReadMessage(); !gws.IsCloseError(err, gws.CloseGoingAway) {
		t.Errorf("connected client got %v, want a going-away close frame", err)
	}

	late := s.dial(t, "bob", "r1")
	late.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, _, err := late.ReadMessage(); !gws.IsCloseError(err, gws.CloseGoingAway) {
		t.Errorf("connection after Stop got %v, want a going-away close frame", err)
	}

	s.hub.Stop() // twice must not panic
}
