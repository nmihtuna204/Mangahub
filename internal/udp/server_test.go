package udp

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

func startServer(t *testing.T) (*NotificationServer, *net.UDPAddr, chan error) {
	return startServerWith(t, nil)
}

// startServerWith sets the token verifier before serving (setting it later would race).
func startServerWith(t *testing.T, verify TokenVerifier) (*NotificationServer, *net.UDPAddr, chan error) {
	t.Helper()
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	s := NewNotificationServer("127.0.0.1", 0)
	s.SetTokenVerifier(verify)
	done := make(chan error, 1)
	go func() { done <- s.Serve(conn) }()
	return s, conn.LocalAddr().(*net.UDPAddr), done
}

func subscribe(t *testing.T, server *net.UDPAddr) *net.UDPConn {
	t.Helper()
	c, err := net.DialUDP("udp", nil, server)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	c.Write([]byte("REGISTER"))
	if got := readString(t, c); got != "REGISTERED" {
		t.Fatalf("register reply = %q", got)
	}
	return c
}

func readString(t *testing.T, c *net.UDPConn) string {
	t.Helper()
	buf := make([]byte, 4096)
	c.SetReadDeadline(time.Now().Add(3 * time.Second))
	n, err := c.Read(buf)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return string(buf[:n])
}

func TestRegisterBroadcastUnregister(t *testing.T) {
	s, addr, _ := startServer(t)
	defer s.Stop()

	a := subscribe(t, addr)
	b := subscribe(t, addr)
	if n := s.SubscriberCount(); n != 2 {
		t.Fatalf("subscribers = %d, want 2", n)
	}

	// External broadcast request, as the API's protocol bridge sends it
	sender, _ := net.DialUDP("udp", nil, addr)
	defer sender.Close()
	payload, _ := json.Marshal(NewProgressNotification("m1", "One Piece", "reader1", 7, "reader1 is now on chapter 7"))
	sender.Write(append([]byte("BROADCAST "), payload...))

	for name, c := range map[string]*net.UDPConn{"a": a, "b": b} {
		var n Notification
		if err := json.Unmarshal([]byte(readString(t, c)), &n); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if n.Type != "progress_update" || n.MangaTitle != "One Piece" || n.Chapter != 7 {
			t.Errorf("%s got %+v", name, n)
		}
	}

	a.Write([]byte("UNREGISTER"))
	if got := readString(t, a); got != "UNREGISTERED" {
		t.Errorf("unregister reply = %q", got)
	}
	if n := s.SubscriberCount(); n != 1 {
		t.Errorf("subscribers after unregister = %d, want 1", n)
	}
}

// Regression: the closed-socket check never matched, so after Stop the read
// loop spun forever on "use of closed network connection".
func TestStopEndsServe(t *testing.T) {
	s, _, done := startServer(t)
	time.Sleep(50 * time.Millisecond)
	s.Stop()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Serve did not return after Stop")
	}
	if !isClosedErr(&net.OpError{Err: net.ErrClosed}) {
		t.Error("isClosedErr does not recognize net.ErrClosed")
	}
}

// subscribeAs registers with a token and returns the socket.
func subscribeAs(t *testing.T, server *net.UDPAddr, token string) *net.UDPConn {
	t.Helper()
	c, err := net.DialUDP("udp", nil, server)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	c.Write([]byte("REGISTER " + token))
	if got := readString(t, c); got != "REGISTERED" {
		t.Fatalf("register with %q: reply %q", token, got)
	}
	return c
}

func expectSilence(t *testing.T, c *net.UDPConn, who string) {
	t.Helper()
	buf := make([]byte, 4096)
	c.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	if n, err := c.Read(buf); err == nil {
		t.Errorf("%s received %s, want nothing", who, buf[:n])
	}
}

// Chapter releases go only to the manga's readers: subscribers that
// registered with a token are matched against the notification's user IDs.
func TestTargetedNotifications(t *testing.T) {
	s, addr, _ := startServerWith(t, func(token string) (string, error) {
		if token == "bad" {
			return "", net.ErrClosed
		}
		return "user-" + token, nil
	})
	defer s.Stop()

	alice := subscribeAs(t, addr, "alice")
	bob := subscribeAs(t, addr, "bob")
	anon := subscribe(t, addr)

	rejected, _ := net.DialUDP("udp", nil, addr)
	defer rejected.Close()
	rejected.Write([]byte("REGISTER bad"))
	if got := readString(t, rejected); got != "ERROR invalid token" {
		t.Errorf("bad token reply = %q", got)
	}
	if n := s.SubscriberCount(); n != 3 {
		t.Errorf("subscribers = %d, want 3 (the bad token must not register)", n)
	}

	sender, _ := net.DialUDP("udp", nil, addr)
	defer sender.Close()
	req, _ := BroadcastRequest(NewChapterReleaseNotification("m1", "One Piece", 1194, []string{"user-alice"}))
	sender.Write(req)

	var n Notification
	raw := readString(t, alice)
	if err := json.Unmarshal([]byte(raw), &n); err != nil || n.Type != "chapter_release" || n.Chapter != 1194 {
		t.Fatalf("alice got %s", raw)
	}
	if n.UserIDs != nil || strings.Contains(raw, "user_ids") {
		t.Errorf("delivered payload leaks the target list: %s", raw)
	}
	expectSilence(t, bob, "bob (not a reader)")
	expectSilence(t, anon, "anonymous subscriber")

	// Untargeted broadcasts still reach everyone
	req, _ = BroadcastRequest(NewSystemNotification("maintenance"))
	sender.Write(req)
	for name, c := range map[string]*net.UDPConn{"alice": alice, "bob": bob, "anon": anon} {
		if !strings.Contains(readString(t, c), "maintenance") {
			t.Errorf("%s missed the general broadcast", name)
		}
	}
}

func TestTokenRegistrationNeedsVerifier(t *testing.T) {
	s, addr, _ := startServer(t)
	defer s.Stop()
	c, _ := net.DialUDP("udp", nil, addr)
	defer c.Close()
	c.Write([]byte("REGISTER some-token"))
	if got := readString(t, c); !strings.HasPrefix(got, "ERROR") {
		t.Errorf("token registration without a verifier: %q, want an ERROR reply", got)
	}
}

func TestChapterReleaseBatchesFitInADatagram(t *testing.T) {
	ids := make([]string, 2500)
	for i := range ids {
		ids[i] = fmt.Sprintf("%08d-0000-0000-0000-000000000000", i) // UUID-sized
	}
	batches := ChapterReleaseBatches("m1", "One Piece", 1194, ids)
	if len(batches) != 3 {
		t.Fatalf("got %d batches, want 3", len(batches))
	}
	total := 0
	for _, b := range batches {
		total += len(b.UserIDs)
		req, _ := BroadcastRequest(b)
		if len(req) > maxDatagram {
			t.Errorf("batch is %d bytes, over the %d-byte UDP limit", len(req), maxDatagram)
		}
	}
	if total != 2500 {
		t.Errorf("batches cover %d users, want 2500", total)
	}
	if len(ChapterReleaseBatches("m1", "x", 1, nil)) != 0 {
		t.Error("no readers should mean no notifications")
	}
}

// Subscribers that stop re-sending REGISTER are dropped after the TTL;
// ones that keep sending it (the clients' heartbeat) stay.
func TestSilentSubscribersExpire(t *testing.T) {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	s := NewNotificationServer("127.0.0.1", 0)
	// Generous margins (TTL = 6 heartbeats) so a loaded machine doesn't miss beats
	s.SetSubscriberTTL(300 * time.Millisecond)
	go s.Serve(conn)
	defer s.Stop()
	addr := conn.LocalAddr().(*net.UDPAddr)

	alive := subscribe(t, addr)
	silent := subscribe(t, addr)

	stop := make(chan struct{})
	defer close(stop)
	go func() {
		for {
			select {
			case <-time.After(50 * time.Millisecond):
				alive.Write([]byte("REGISTER"))
			case <-stop:
				return
			}
		}
	}()

	deadline := time.Now().Add(3 * time.Second)
	for s.SubscriberCount() != 1 {
		if time.Now().After(deadline) {
			t.Fatalf("subscribers = %d, want the silent one expired", s.SubscriberCount())
		}
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(450 * time.Millisecond) // longer than the TTL: the heartbeat keeps it
	if n := s.SubscriberCount(); n != 1 {
		t.Fatalf("subscribers = %d, want the heartbeating one kept", n)
	}

	s.Broadcast <- NewProgressNotification("m1", "One Piece", "reader1", 7, "hello")
	// alive also gets REGISTERED replies to its heartbeat; skip them
	for {
		if got := readString(t, alive); got != "REGISTERED" {
			if !strings.Contains(got, "hello") {
				t.Errorf("alive got %q", got)
			}
			break
		}
	}
	expectSilence(t, silent, "expired subscriber")
}
