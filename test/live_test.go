// Live tests run against the four real server processes on their default
// ports (localhost:8080/9090/9091/9092). They are skipped unless
// MANGAHUB_LIVE=1 is set (make test-integration sets it); with it set, a
// server that isn't reachable is a failure, not a skip. The self-contained
// end-to-end tests in e2e_test.go start everything in-process instead and
// always run.
//
// TestLiveProgressFansOut changes reader1's progress on one manga.
package test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	gws "github.com/gorilla/websocket"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	pb "mangahub/internal/grpc/pb"
)

const (
	liveHTTP = "http://localhost:8080"
	liveWS   = "ws://localhost:8080"
	liveTCP  = "localhost:9090"
	liveUDP  = "127.0.0.1:9091"
	liveGRPC = "localhost:9092"
)

func requireLive(t *testing.T) {
	t.Helper()
	if testing.Short() || os.Getenv("MANGAHUB_LIVE") == "" {
		t.Skip("live test: set MANGAHUB_LIVE=1 and start all four servers")
	}
}

// liveCall makes an HTTP request to the running API and decodes the JSON body.
func liveCall(t *testing.T, method, path string, body interface{}, token string) (int, map[string]interface{}) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req, _ := http.NewRequest(method, liveHTTP+path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v (is the API server running?)", method, path, err)
	}
	defer resp.Body.Close()
	var out map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func liveLogin(t *testing.T, user, password string) string {
	t.Helper()
	code, out := liveCall(t, "POST", "/auth/login", map[string]string{"username": user, "password": password}, "")
	if code != 200 {
		t.Fatalf("login %s: %d %v", user, code, out)
	}
	return data(out)["token"].(string)
}

func liveGRPCClient(t *testing.T) pb.MangaServiceClient {
	t.Helper()
	conn, err := grpc.NewClient(liveGRPC, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return pb.NewMangaServiceClient(conn)
}

// udpSubscribe registers an anonymous UDP subscriber.
func udpSubscribe(t *testing.T) *net.UDPConn {
	t.Helper()
	addr, _ := net.ResolveUDPAddr("udp", liveUDP)
	c, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		c.Write([]byte("UNREGISTER"))
		c.Close()
	})
	c.Write([]byte("REGISTER"))
	buf := make([]byte, 64)
	c.SetReadDeadline(time.Now().Add(3 * time.Second))
	if n, err := c.Read(buf); err != nil || string(buf[:n]) != "REGISTERED" {
		t.Fatalf("UDP REGISTER: %q, %v (is the UDP server running?)", buf[:n], err)
	}
	return c
}

// udpWait reads notifications until match returns true for one.
func udpWait(t *testing.T, c *net.UDPConn, what string, match func(map[string]interface{}) bool) map[string]interface{} {
	t.Helper()
	buf := make([]byte, 65536)
	c.SetReadDeadline(time.Now().Add(5 * time.Second))
	for {
		n, err := c.Read(buf)
		if err != nil {
			t.Fatalf("UDP: no %s: %v", what, err)
		}
		var m map[string]interface{}
		if json.Unmarshal(buf[:n], &m) == nil && match(m) {
			return m
		}
	}
}

// tcpWait reads relayed lines until match returns true for one.
func tcpWait(t *testing.T, r *bufio.Reader, c net.Conn, what string, match func(map[string]interface{}) bool) {
	t.Helper()
	c.SetReadDeadline(time.Now().Add(5 * time.Second))
	for {
		line, err := r.ReadBytes('\n')
		if err != nil {
			t.Fatalf("TCP: no %s: %v", what, err)
		}
		var m map[string]interface{}
		if json.Unmarshal(line, &m) == nil && match(m) {
			return
		}
	}
}

func TestLiveHTTPHealthAndSearch(t *testing.T) {
	requireLive(t)

	code, health := liveCall(t, "GET", "/health", nil, "")
	if code != 200 || health["status"] != "healthy" {
		t.Errorf("GET /health = %d %v", code, health)
	}
	code, out := liveCall(t, "GET", "/manga?limit=5", nil, "")
	if code != 200 || len(list(data(out)["data"])) != 5 {
		t.Errorf("GET /manga?limit=5 = %d, %d results", code, len(list(data(out)["data"])))
	}
	if code, _ := liveCall(t, "GET", "/users/library", nil, ""); code != 401 {
		t.Errorf("GET /users/library without a token = %d, want 401", code)
	}
}

func TestLiveGRPC(t *testing.T) {
	requireLive(t)
	client := liveGRPCClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := client.SearchManga(ctx, &pb.SearchRequest{Query: "one", Limit: 10})
	if err != nil || len(res.Manga) == 0 {
		t.Fatalf("SearchManga(one) = %v, %v; want results", res, err)
	}
	m, err := client.GetManga(ctx, &pb.GetMangaRequest{MangaId: res.Manga[0].Id})
	if err != nil || m.Title != res.Manga[0].Title {
		t.Errorf("GetManga(%s) = %v, %v", res.Manga[0].Id, m, err)
	}
	if _, err := client.GetManga(ctx, &pb.GetMangaRequest{MangaId: "no-such-manga"}); status.Code(err) != codes.NotFound {
		t.Errorf("GetManga(unknown) = %v, want NotFound", err)
	}
	_, err = client.UpdateProgress(ctx, &pb.ProgressRequest{UserId: "reader1", MangaId: m.Id, CurrentChapter: 1})
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("UpdateProgress without a token = %v, want Unauthenticated", err)
	}
}

func TestLiveTCPRelay(t *testing.T) {
	requireLive(t)
	dial := func() (net.Conn, *bufio.Reader) {
		c, err := net.DialTimeout("tcp", liveTCP, 2*time.Second)
		if err != nil {
			t.Fatalf("TCP: %v (is the TCP server running?)", err)
		}
		t.Cleanup(func() { c.Close() })
		return c, bufio.NewReader(c)
	}
	a, _ := dial()
	b, br := dial()
	time.Sleep(200 * time.Millisecond) // let the server register both

	user := fmt.Sprintf("live-%d", time.Now().UnixNano())
	line, _ := json.Marshal(map[string]interface{}{"user_id": user, "manga_id": "live-test", "chapter": 42, "timestamp": time.Now().Unix()})
	a.Write(append(line, '\n'))
	tcpWait(t, br, b, "relay of the other client's update", func(m map[string]interface{}) bool {
		return m["user_id"] == user && m["chapter"] == float64(42)
	})
}

func TestLiveConcurrentTCPConnections(t *testing.T) {
	requireLive(t)
	const clients = 5
	var wg sync.WaitGroup
	errs := make(chan error, clients)
	for i := 0; i < clients; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c, err := net.DialTimeout("tcp", liveTCP, 2*time.Second)
			if err != nil {
				errs <- err
				return
			}
			defer c.Close()
			user := fmt.Sprintf("live-concurrent-%d-%d", i, time.Now().UnixNano())
			line, _ := json.Marshal(map[string]interface{}{"user_id": user, "manga_id": "live-test", "chapter": i, "timestamp": time.Now().Unix()})
			c.Write(append(line, '\n'))
			// The server relays to every client, the sender included
			r := bufio.NewReader(c)
			c.SetReadDeadline(time.Now().Add(5 * time.Second))
			for {
				got, err := r.ReadBytes('\n')
				if err != nil {
					errs <- fmt.Errorf("client %d: %v", i, err)
					return
				}
				if bytes.Contains(got, []byte(user)) {
					return
				}
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func TestLiveUDPBroadcast(t *testing.T) {
	requireLive(t)
	sub := udpSubscribe(t)

	text := fmt.Sprintf("live-udp-%d", time.Now().UnixNano())
	payload, _ := json.Marshal(map[string]interface{}{"type": "system", "message": text, "timestamp": time.Now().Unix()})
	sender, _ := net.Dial("udp", liveUDP)
	defer sender.Close()
	sender.Write(append([]byte("BROADCAST "), payload...))

	udpWait(t, sub, "broadcast", func(m map[string]interface{}) bool { return m["message"] == text })
}

func TestLiveWebSocket(t *testing.T) {
	requireLive(t)

	resp, err := http.Get(liveHTTP + "/ws/chat?room_id=live")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("/ws/chat without a token = %d, want 401", resp.StatusCode)
	}

	tok := liveLogin(t, "reader1", "password123")
	room := fmt.Sprintf("live-%d", time.Now().UnixNano())
	conn, _, err := gws.DefaultDialer.Dial(liveWS+"/ws/chat?room_id="+room+"&token="+tok, nil)
	if err != nil {
		t.Fatalf("WebSocket dial: %v", err)
	}
	defer conn.Close()
	if m := readWS(t, conn); m["type"] != "join" {
		t.Errorf("first message = %v, want the join notice", m)
	}
	conn.WriteJSON(map[string]string{"content": "hello from the live test"})
	if m := readWS(t, conn); m["type"] != "message" || m["content"] != "hello from the live test" {
		t.Errorf("echo = %v", m)
	}
}

// The project's core feature against the real processes: one HTTP progress
// update reaches TCP sync clients, UDP subscribers and the manga's chat room.
func TestLiveProgressFansOut(t *testing.T) {
	requireLive(t)
	tok := liveLogin(t, "reader1", "password123")
	_, me := liveCall(t, "GET", "/auth/me", nil, tok)
	userID, _ := data(me)["id"].(string)
	_, out := liveCall(t, "GET", "/manga?limit=1", nil, "")
	manga := list(data(out)["data"])[0]
	mangaID := manga["id"].(string)

	chapter := 1
	_, lib := liveCall(t, "GET", "/users/library", nil, tok)
	for _, e := range list(lib["data"]) {
		if e["manga_id"] == mangaID {
			chapter = int(e["current_chapter"].(float64)) + 1
		}
	}

	tcpConn, err := net.DialTimeout("tcp", liveTCP, 2*time.Second)
	if err != nil {
		t.Fatalf("TCP: %v", err)
	}
	defer tcpConn.Close()
	udpSub := udpSubscribe(t)
	room, _, err := gws.DefaultDialer.Dial(liveWS+"/ws/chat?room_id=manga_"+mangaID+"&token="+tok, nil)
	if err != nil {
		t.Fatalf("WebSocket: %v", err)
	}
	defer room.Close()
	readWS(t, room) // own join notice
	time.Sleep(200 * time.Millisecond)

	if code, out := liveCall(t, "PUT", "/users/progress", map[string]interface{}{"manga_id": mangaID, "current_chapter": chapter}, tok); code != 200 {
		t.Fatalf("PUT /users/progress = %d %v", code, out)
	}

	tcpWait(t, bufio.NewReader(tcpConn), tcpConn, "progress update", func(m map[string]interface{}) bool {
		return m["manga_id"] == mangaID && m["chapter"] == float64(chapter) && m["user_id"] == userID
	})
	udpWait(t, udpSub, "progress_update notification", func(m map[string]interface{}) bool {
		return m["type"] == "progress_update" && m["manga_id"] == mangaID && m["chapter"] == float64(chapter)
	})
	want := fmt.Sprintf("chapter %d of %s", chapter, manga["title"])
	for i := 0; ; i++ {
		m := readWS(t, room)
		if m["type"] == "system" && strings.Contains(fmt.Sprint(m["content"]), want) {
			break
		}
		if i == 10 {
			t.Fatalf("chat room: no notice containing %q", want)
		}
	}
}
