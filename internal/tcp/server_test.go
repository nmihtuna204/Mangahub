package tcp

import (
	"bufio"
	"encoding/json"
	"net"
	"testing"
	"time"
)

func startServer(t *testing.T) (*ProgressSyncServer, string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := NewProgressSyncServer("127.0.0.1", 0)
	go s.Serve(ln)
	t.Cleanup(func() { s.Stop() })
	return s, ln.Addr().String()
}

func dial(t *testing.T, addr string) net.Conn {
	t.Helper()
	c, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestRelaysUpdatesToAllClients(t *testing.T) {
	s, addr := startServer(t)
	a, b := dial(t, addr), dial(t, addr)
	waitFor(t, "2 clients", func() bool { return s.ClientCount() == 2 })

	data, _ := json.Marshal(NewProgressUpdate("u1", "m1", 42))
	a.Write(append(data, '\n'))

	for name, c := range map[string]net.Conn{"sender": a, "other": b} {
		c.SetReadDeadline(time.Now().Add(3 * time.Second))
		line, err := bufio.NewReader(c).ReadBytes('\n')
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var u ProgressUpdate
		if json.Unmarshal(line, &u) != nil || u.MangaID != "m1" || u.Chapter != 42 {
			t.Errorf("%s got %s", name, line)
		}
	}
}

// Regression: handleConnection waited for both loops, but the write loop only
// ends when unregister closes its channel, so disconnected clients were never
// removed.
func TestDisconnectedClientsAreUnregistered(t *testing.T) {
	s, addr := startServer(t)
	c := dial(t, addr)
	waitFor(t, "client registered", func() bool { return s.ClientCount() == 1 })

	c.Close()
	waitFor(t, "client unregistered", func() bool { return s.ClientCount() == 0 })
}

func TestInvalidJSONIsIgnored(t *testing.T) {
	s, addr := startServer(t)
	c := dial(t, addr)
	waitFor(t, "client registered", func() bool { return s.ClientCount() == 1 })

	c.Write([]byte("not json\n"))
	data, _ := json.Marshal(NewProgressUpdate("u1", "m1", 1))
	c.Write(append(data, '\n'))

	c.SetReadDeadline(time.Now().Add(3 * time.Second))
	line, err := bufio.NewReader(c).ReadBytes('\n')
	if err != nil || s.ClientCount() != 1 {
		t.Fatalf("connection should survive bad input: %v (clients %d)", err, s.ClientCount())
	}
	var u ProgressUpdate
	json.Unmarshal(line, &u)
	if u.Chapter != 1 {
		t.Errorf("got %s", line)
	}
}
