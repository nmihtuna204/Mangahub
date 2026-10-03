// End-to-end tests: the whole system runs in-process (see stack_test.go), so
// these need no running servers and run in CI.
package test

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	gws "github.com/gorilla/websocket"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pb "mangahub/internal/grpc/pb"
	"mangahub/internal/server"
)

// One PUT /users/progress must reach every protocol: the TCP sync clients,
// the UDP subscribers, the gRPC audit log and the manga's chat room.
func TestProgressFansOutToAllProtocols(t *testing.T) {
	s := startStack(t)
	tok := s.login("reader1", "password123")
	m := s.manga()[0]
	mangaID, title := m["id"].(string), m["title"].(string)

	tcpSub, err := net.Dial("tcp", s.tcpAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer tcpSub.Close()
	eventually(t, "TCP subscriber registered", func() bool { return s.tcp.ClientCount() >= 2 }) // bridge + us

	udpSub, _ := net.Dial("udp", s.udpAddr)
	defer udpSub.Close()
	udpSub.Write([]byte("REGISTER"))
	buf := make([]byte, 4096)
	udpSub.SetReadDeadline(time.Now().Add(3 * time.Second))
	if n, err := udpSub.Read(buf); err != nil || string(buf[:n]) != "REGISTERED" {
		t.Fatalf("UDP register: %q %v", buf[:n], err)
	}

	room, _, err := gws.DefaultDialer.Dial(s.wsURL("/ws/chat?room_id=manga_"+mangaID+"&token="+tok), nil)
	if err != nil {
		t.Fatalf("WebSocket with ?token=: %v", err)
	}
	defer room.Close()
	readWS(t, room) // own join notice

	s.mustCall(200, "PUT", "/users/progress", map[string]interface{}{"manga_id": mangaID, "current_chapter": 7, "status": "reading"}, tok)

	tcpSub.SetReadDeadline(time.Now().Add(3 * time.Second))
	line, err := bufio.NewReader(tcpSub).ReadBytes('\n')
	var update map[string]interface{}
	if err != nil || json.Unmarshal(line, &update) != nil || update["manga_id"] != mangaID || update["chapter"] != float64(7) {
		t.Errorf("TCP: got %s (%v)", line, err)
	}

	udpSub.SetReadDeadline(time.Now().Add(3 * time.Second))
	n, err := udpSub.Read(buf)
	var note map[string]interface{}
	if err != nil || json.Unmarshal(buf[:n], &note) != nil || note["type"] != "progress_update" ||
		note["manga_title"] != title || note["chapter"] != float64(7) {
		t.Errorf("UDP: got %s (%v)", buf[:n], err)
	}

	msg := readWS(t, room)
	if msg["type"] != "system" || !strings.Contains(fmt.Sprint(msg["content"]), "chapter 7 of "+title) {
		t.Errorf("WebSocket: got %v", msg)
	}

	eventually(t, "gRPC audit", func() bool { return atomic.LoadInt32(s.audits) == 1 })
	if e := s.entry(tok, mangaID); e["current_chapter"] != float64(7) {
		t.Errorf("stored chapter = %v", e["current_chapter"])
	}
}

// Regression: the bridge's gRPC leg re-wrote progress asynchronously and could
// land after a newer HTTP update, briefly rolling the chapter back.
func TestRapidUpdatesNeverRollBack(t *testing.T) {
	s := startStack(t)
	tok := s.login("reader2", "password123")
	mangaID := s.manga()[1]["id"].(string)

	for i := 0; i < 20; i++ {
		base := 10 + 2*i
		s.mustCall(200, "PUT", "/users/progress", map[string]interface{}{"manga_id": mangaID, "current_chapter": base}, tok)
		s.mustCall(200, "PUT", "/users/progress", map[string]interface{}{"manga_id": mangaID, "current_chapter": base + 1}, tok)
		for r := 0; r < 3; r++ {
			if got := s.entry(tok, mangaID)["current_chapter"]; got != float64(base+1) {
				t.Fatalf("iteration %d: read chapter %v right after saving %d", i, got, base+1)
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
}

// Regression: nested queries inside open result sets deadlocked the 25-connection
// pool as soon as more than 25 list requests ran at once.
func TestConcurrentRequestsDontExhaustThePool(t *testing.T) {
	s := startStack(t)
	tok := s.login("reader1", "password123")
	for _, m := range s.manga()[:10] {
		s.mustCall(200, "PUT", "/users/progress", map[string]interface{}{"manga_id": m["id"], "current_chapter": 1}, tok)
	}

	var wg sync.WaitGroup
	var failures int32
	start := time.Now()
	for i := 0; i < 60; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			path, token := "/manga?limit=100", ""
			if i%3 == 0 {
				path, token = "/users/library", tok
			}
			if code, _ := s.call("GET", path, nil, token); code != 200 {
				atomic.AddInt32(&failures, 1)
			}
		}(i)
	}
	wg.Wait()
	if failures > 0 {
		t.Errorf("%d of 60 concurrent requests failed", failures)
	}
	if d := time.Since(start); d > 10*time.Second {
		t.Errorf("60 concurrent requests took %v", d)
	}
}

func TestPartialProgressUpdatesOverHTTP(t *testing.T) {
	s := startStack(t)
	tok := s.login("reader1", "password123")
	id := s.manga()[2]["id"].(string)
	put := func(body map[string]interface{}) int {
		body["manga_id"] = id
		code, _ := s.call("PUT", "/users/progress", body, tok)
		return code
	}

	put(map[string]interface{}{"current_chapter": 5, "status": "reading", "is_favorite": true})
	if code := put(map[string]interface{}{"current_chapter": 6}); code != 200 {
		t.Fatalf("PUT without status = %d (was 500)", code)
	}
	if e := s.entry(tok, id); e["is_favorite"] != true || e["current_chapter"] != float64(6) {
		t.Errorf("chapter update cleared the favorite: %v", e)
	}
	put(map[string]interface{}{"is_favorite": false})  // the TUI's favorite toggle
	put(map[string]interface{}{"status": "completed"}) // the TUI's status change
	e := s.entry(tok, id)
	if e["current_chapter"] != float64(6) || e["is_favorite"] != false || e["status"] != "completed" || e["completed_at"] == nil {
		t.Errorf("after favorite + status updates: %v", e)
	}
	for body, want := range map[string]int{
		`{"status":"bogus"}`:       400,
		`{"current_chapter":-1}`:   400,
		`{"status":""}`:            200,
		`{"current_chapter":null}`: 200,
	} {
		var m map[string]interface{}
		json.Unmarshal([]byte(body), &m)
		if code := put(m); code != want {
			t.Errorf("%s: status %d, want %d", body, code, want)
		}
	}
}

func TestRatingsAndActivityFeed(t *testing.T) {
	s := startStack(t)
	tok := s.login("reader1", "password123")
	ids := s.manga()
	rated, read := ids[3]["id"].(string), ids[4]["id"].(string)

	out := s.mustCall(200, "POST", "/manga/"+rated+"/ratings", map[string]interface{}{"rating": 8, "review_text": "great", "is_spoiler": true}, tok)
	if out["success"] != true || data(out)["is_spoiler"] != true {
		t.Errorf("rating response = %v", out)
	}
	s.mustCall(200, "PUT", "/users/progress", map[string]interface{}{"manga_id": read, "current_chapter": 3}, tok)

	activity := func() []map[string]interface{} {
		out := s.mustCall(200, "GET", "/activities?limit=100", nil, "") // was 500 once any rating existed
		return list(out["activities"])
	}
	count := func(kind, mangaID string) (n int, last map[string]interface{}) {
		for _, a := range activity() {
			if a["activity_type"] == kind && a["manga_id"] == mangaID && a["username"] == "reader1" {
				n++
				last = a
			}
		}
		return
	}
	eventually(t, "progress activity", func() bool { n, _ := count("progress", read); return n == 1 })
	if n, _ := count("rating", rated); n != 1 {
		t.Errorf("rating activities = %d, want 1", n)
	}

	s.mustCall(200, "POST", "/manga/"+rated+"/ratings", map[string]interface{}{"rating": 5}, tok)
	if n, a := count("rating", rated); n != 1 || a["rating"] != float64(5) {
		t.Errorf("after re-rating: %d entries, last %v; want 1 entry with rating 5", n, a)
	}
	summary := data(s.mustCall(200, "GET", "/manga/"+rated+"/ratings", nil, ""))["summary"].(map[string]interface{})
	if summary["rating_count"] != float64(1) || summary["average_rating"] != float64(5) {
		t.Errorf("summary = %v", summary)
	}

	s.mustCall(200, "DELETE", "/manga/"+rated+"/ratings", nil, tok)
	if n, _ := count("rating", rated); n != 0 {
		t.Errorf("deleted rating still in the feed")
	}

	for _, c := range []struct {
		method, path string
		body         interface{}
		want         int
	}{
		{"POST", "/manga/" + rated + "/ratings", map[string]int{"rating": 11}, 400},
		{"POST", "/manga/nope/ratings", map[string]int{"rating": 7}, 404},
		{"DELETE", "/manga/" + rated + "/ratings", nil, 404},
		{"GET", "/manga/nope/ratings", nil, 404},
	} {
		if code, _ := s.call(c.method, c.path, c.body, tok); code != c.want {
			t.Errorf("%s %s: %d, want %d (every rating error used to be 500)", c.method, c.path, code, c.want)
		}
	}
}

func TestSearchPaginationAndLeaderboards(t *testing.T) {
	s := startStack(t)

	out := s.mustCall(200, "GET", "/manga?genre=Slice%20of%20Life&limit=100", nil, "")
	results := list(data(out)["data"])
	if len(results) == 0 {
		t.Fatal("genre filter by display name returned nothing")
	}
	for _, m := range results {
		found := false
		for _, g := range list(m["genres"]) {
			found = found || g["slug"] == "slice-of-life"
		}
		if !found {
			t.Errorf("%v has no slice-of-life genre", m["title"])
		}
	}

	p1 := list(data(s.mustCall(200, "GET", "/manga?limit=5&offset=0", nil, ""))["data"])
	p2 := list(data(s.mustCall(200, "GET", "/manga?limit=5&offset=5", nil, ""))["data"])
	if len(p1) != 5 || len(p2) != 5 || p1[0]["id"] == p2[0]["id"] {
		t.Error("limit/offset pages overlap")
	}

	empty := s.mustCall(200, "GET", "/manga?q=zzzz-not-a-title", nil, "")
	if d, ok := data(empty)["data"].([]interface{}); !ok || len(d) != 0 {
		t.Errorf("empty search data = %v, want []", data(empty)["data"])
	}

	trend := s.mustCall(200, "GET", "/leaderboards/trending?offset=1000", nil, "") // fallback query used to 500
	if e, ok := data(trend)["entries"].([]interface{}); !ok || len(e) != 0 {
		t.Errorf("trending past the end = %v", data(trend)["entries"])
	}
	fallback := s.mustCall(200, "GET", "/leaderboards/trending", nil, "")
	if e, _ := data(fallback)["entries"].([]interface{}); len(e) == 0 {
		t.Error("trending with no recent activity should fall back to top manga")
	}
	s.mustCall(200, "GET", "/leaderboards/manga", nil, "")
	s.mustCall(200, "GET", "/leaderboards/users", nil, "")
}

func TestCommentThreads(t *testing.T) {
	s := startStack(t)
	t1, t2 := s.login("reader1", "password123"), s.login("reader2", "password123")
	ids := s.manga()
	m, other := ids[5]["id"].(string), ids[6]["id"].(string)

	top := data(s.mustCall(201, "POST", "/manga/"+m+"/comments", map[string]string{"content": "top level"}, t1))["id"].(string)
	reply := data(s.mustCall(201, "POST", "/manga/"+m+"/comments", map[string]string{"content": "reply", "parent_id": top}, t2))["id"].(string)
	nested := data(s.mustCall(201, "POST", "/manga/"+m+"/comments", map[string]string{"content": "reply to reply", "parent_id": reply}, t1))
	if nested["parent_id"] != top {
		t.Errorf("reply-to-reply parent = %v, want the top-level comment", nested["parent_id"])
	}
	s.mustCall(200, "POST", "/comments/"+top+"/like", nil, t2)

	page := data(s.mustCall(200, "GET", "/manga/"+m+"/comments?page_size=1", nil, t2))
	comments := list(page["comments"])
	if page["total_count"] != float64(1) || page["has_more"] != false || len(comments) != 1 {
		t.Errorf("page = total %v has_more %v", page["total_count"], page["has_more"])
	} else if comments[0]["liked_by_me"] != true || len(list(comments[0]["replies"])) != 2 {
		t.Errorf("comment = liked_by_me %v, %d replies", comments[0]["liked_by_me"], len(list(comments[0]["replies"])))
	}
	anon := data(s.mustCall(200, "GET", "/manga/"+m+"/comments", nil, ""))
	if list(anon["comments"])[0]["liked_by_me"] != false {
		t.Error("anonymous request reports liked_by_me")
	}

	if code, _ := s.call("POST", "/manga/nope/comments", map[string]string{"content": "x"}, t1); code != 404 {
		t.Errorf("comment on unknown manga: %d", code)
	}
	if code, _ := s.call("POST", "/manga/"+other+"/comments", map[string]string{"content": "x", "parent_id": top}, t1); code != 400 {
		t.Errorf("parent from another manga: %d", code)
	}
}

func TestAuthHardening(t *testing.T) {
	s := startStack(t)

	s.mustCall(201, "POST", "/auth/register", map[string]string{"username": "disabled1", "email": "d1@example.com", "password": "password123"}, "")
	if _, err := s.db.Exec(`UPDATE users SET is_active = 0 WHERE username = 'disabled1'`); err != nil {
		t.Fatal(err)
	}
	if code, _ := s.call("POST", "/auth/login", map[string]string{"username": "disabled1", "password": "password123"}, ""); code != 403 {
		t.Errorf("disabled account login: %d, want 403", code)
	}

	tok := s.login("reader1", "password123")
	none := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`)) + "." + strings.Split(tok, ".")[1] + "."
	if code, _ := s.call("GET", "/auth/me", nil, none); code != 401 {
		t.Errorf("alg=none token: %d, want 401", code)
	}
	if code, _ := s.call("GET", "/users/library?token="+tok, nil, ""); code != 401 {
		t.Errorf("?token= on a REST route: %d, want 401 (only /ws/chat accepts it)", code)
	}

	me := data(s.mustCall(200, "GET", "/auth/me", nil, tok))
	if me["display_name"] == "" || strings.HasPrefix(fmt.Sprint(me["created_at"]), "0001") {
		t.Errorf("/auth/me = %v, want the stored profile (it used to echo only the token's id/username)", me)
	}

	refreshed := data(s.mustCall(200, "POST", "/auth/refresh", nil, s.login("admin", "admin123")))["token"].(string)
	raw, _ := base64.RawURLEncoding.DecodeString(strings.Split(refreshed, ".")[1])
	var claims map[string]interface{}
	json.Unmarshal(raw, &claims)
	if claims["role"] != "admin" {
		t.Errorf("refreshed admin token role = %v", claims["role"])
	}
}

func TestChatPersistenceAndHistory(t *testing.T) {
	s := startStack(t)
	t1, t2 := s.login("reader1", "password123"), s.login("reader2", "password123")

	alice, _, err := gws.DefaultDialer.Dial(s.wsURL("/ws/chat?room_id=general&token="+t1), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer alice.Close()
	readWS(t, alice)
	bob, _, err := gws.DefaultDialer.Dial(s.wsURL("/ws/chat?room_id=general&token="+t2), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer bob.Close()
	readWS(t, alice)
	readWS(t, bob)

	alice.WriteJSON(map[string]string{"content": "first"})
	bob.WriteJSON(map[string]string{"content": "fake notice", "type": "system"})
	alice.WriteJSON(map[string]string{"content": strings.Repeat("x", 600)}) // frames over 512 bytes used to kill the connection

	var got []map[string]interface{}
	for i := 0; i < 3; i++ {
		got = append(got, readWS(t, bob))
	}
	// alice and bob send concurrently, so find messages by content, not position
	long := false
	for _, m := range got {
		if m["content"] == "fake notice" && m["type"] != "message" {
			t.Errorf("client-sent type was honored: %v", m)
		}
		long = long || len(fmt.Sprint(m["content"])) == 600
	}
	if !long {
		t.Error("600-char message not delivered")
	}

	if empty := s.mustCall(200, "GET", "/rooms/nobody-here", nil, ""); empty["clients"] == nil {
		t.Error("empty room lists clients as null, want []")
	}
	info := s.mustCall(200, "GET", "/rooms/general", nil, "")
	if info["count"] != float64(2) {
		t.Errorf("room count = %v", info["count"])
	}
	var msgs []map[string]interface{}
	eventually(t, "3 saved messages", func() bool {
		msgs = list(data(s.mustCall(200, "GET", "/rooms/general/messages", nil, ""))["messages"])
		return len(msgs) == 3
	})
	// Same order the room saw live
	for i := range msgs {
		if msgs[i]["content"] != got[i]["content"] {
			t.Errorf("history[%d] = %v, live order had %v", i, msgs[i]["content"], got[i]["content"])
		}
	}
	if len(msgs) != 3 || msgs[0]["username"] == nil {
		t.Errorf("history = %v (chat used to be persisted nowhere)", msgs)
	}
}

func TestGRPCOverTheWire(t *testing.T) {
	s := startStack(t)
	conn, err := grpc.NewClient(s.grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	client := pb.NewMangaServiceClient(conn)
	ctx := t.Context()

	_, err = client.GetManga(ctx, &pb.GetMangaRequest{MangaId: "nope"})
	if status.Code(err) != codes.NotFound {
		t.Errorf("GetManga unknown: %v", err)
	}
	res, err := client.SearchManga(ctx, &pb.SearchRequest{Genres: []string{"mecha"}, Limit: 50})
	if err != nil || res.Total == 0 {
		t.Fatalf("SearchManga by genre: %v, %v", res, err)
	}
	for _, m := range res.Manga {
		ok := false
		for _, g := range m.Genres {
			ok = ok || g.Slug == "mecha"
		}
		if !ok {
			t.Errorf("%s returned for genre mecha without that genre", m.Title)
		}
	}
	req := &pb.ProgressRequest{UserId: "admin", MangaId: res.Manga[0].Id, CurrentChapter: 4}
	withToken := func(tok string) context.Context {
		return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+tok)
	}

	// UpdateProgress needs a JWT, and only for your own progress (admins: anyone's)
	if _, err := client.UpdateProgress(ctx, req); status.Code(err) != codes.Unauthenticated {
		t.Errorf("no token: %v, want Unauthenticated", err)
	}
	if _, err := client.UpdateProgress(withToken("forged"), req); status.Code(err) != codes.Unauthenticated {
		t.Errorf("forged token: %v, want Unauthenticated", err)
	}
	reader := s.login("reader1", "password123")
	if _, err := client.UpdateProgress(withToken(reader), req); status.Code(err) != codes.PermissionDenied {
		t.Errorf("reader1 updating admin's progress: %v, want PermissionDenied", err)
	}
	admin := s.login("admin", "admin123")
	bad := &pb.ProgressRequest{UserId: "admin", MangaId: res.Manga[0].Id, CurrentChapter: -5}
	if _, err := client.UpdateProgress(withToken(admin), bad); status.Code(err) != codes.InvalidArgument {
		t.Errorf("negative chapter: %v", err)
	}
	up, err := client.UpdateProgress(withToken(admin), req)
	if err != nil || up.CurrentChapter != 4 {
		t.Errorf("UpdateProgress with the user's own token should write: %v, %v", up, err)
	}
}

func readWS(t *testing.T, c *gws.Conn) map[string]interface{} {
	t.Helper()
	c.SetReadDeadline(time.Now().Add(3 * time.Second))
	var m map[string]interface{}
	if err := c.ReadJSON(&m); err != nil {
		t.Fatalf("websocket read: %v", err)
	}
	return m
}

func TestCORSAndErrorSanitizing(t *testing.T) {
	s := startStack(t)

	req, _ := http.NewRequest(http.MethodOptions, s.base+"/users/progress", nil)
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Access-Control-Request-Method", "PUT")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent || resp.Header.Get("Access-Control-Allow-Origin") != "*" ||
		!strings.Contains(resp.Header.Get("Access-Control-Allow-Headers"), "Authorization") {
		t.Errorf("preflight: %d %v", resp.StatusCode, resp.Header)
	}

	// Internal errors must not leak SQL details to clients
	if _, err := s.db.Exec(`DROP TABLE manga_ratings`); err != nil {
		t.Fatal(err)
	}
	code, out := s.call("GET", "/leaderboards/manga", nil, "")
	body, _ := json.Marshal(out)
	if code != 500 || strings.Contains(strings.ToLower(string(body)), "sql") || strings.Contains(string(body), "manga_ratings") {
		t.Errorf("leaderboard error response leaks internals: %d %s", code, body)
	}
}

func TestCustomListsOverHTTP(t *testing.T) {
	s := startStack(t)
	alice, bob := s.login("reader1", "password123"), s.login("reader2", "password123")
	ids := s.manga()

	if code, _ := s.call("GET", "/lists", nil, ""); code != 401 {
		t.Errorf("GET /lists anonymously without user_id: %d, want 401", code)
	}
	pub := data(s.mustCall(201, "POST", "/lists", map[string]interface{}{"name": "Must read", "is_public": true}, alice))["id"].(string)
	priv := data(s.mustCall(201, "POST", "/lists", map[string]interface{}{"name": "Guilty pleasures"}, alice))["id"].(string)

	for _, m := range ids[:3] {
		s.mustCall(201, "POST", "/lists/"+pub+"/items", map[string]interface{}{"manga_id": m["id"], "notes": "x"}, alice)
	}
	if code, _ := s.call("POST", "/lists/"+pub+"/items", map[string]interface{}{"manga_id": "nope"}, alice); code != 404 {
		t.Errorf("unknown manga: %d", code)
	}
	if code, _ := s.call("POST", "/lists/"+pub+"/items", map[string]interface{}{"manga_id": ids[4]["id"]}, bob); code != 403 {
		t.Errorf("bob adding to alice's list: %d, want 403", code)
	}

	pubList := data(s.mustCall(200, "GET", "/lists/"+pub, nil, ""))
	items := list(pubList["items"])
	if pubList["item_count"] != float64(3) || len(items) != 3 || items[0]["manga"].(map[string]interface{})["title"] != ids[0]["title"] {
		t.Errorf("public list = count %v, %d items", pubList["item_count"], len(items))
	}
	if code, _ := s.call("GET", "/lists/"+priv, nil, bob); code != 404 {
		t.Errorf("bob reading alice's private list: %d, want 404", code)
	}

	order := []string{items[2]["id"].(string), items[1]["id"].(string), items[0]["id"].(string)}
	s.mustCall(200, "PUT", "/lists/"+pub+"/order", map[string]interface{}{"item_ids": order}, alice)
	if first := list(data(s.mustCall(200, "GET", "/lists/"+pub, nil, alice))["items"])[0]["id"]; first != order[0] {
		t.Errorf("after reorder first item = %v", first)
	}

	mine := data(s.mustCall(200, "GET", "/lists", nil, alice))
	theirs := data(s.mustCall(200, "GET", "/lists?user_id="+data(s.mustCall(200, "GET", "/auth/me", nil, alice))["id"].(string), nil, bob))
	if mine["total"] != float64(2) || theirs["total"] != float64(1) {
		t.Errorf("alice sees %v lists, bob sees %v of hers; want 2 and 1", mine["total"], theirs["total"])
	}

	acts := list(s.mustCall(200, "GET", "/activities?limit=100", nil, "")["activities"])
	adds := 0
	for _, a := range acts {
		if a["activity_type"] == "list_add" && a["comment_text"] == "Must read" {
			adds++
		}
	}
	if adds != 3 {
		t.Errorf("list_add activities = %d, want 3", adds)
	}

	s.mustCall(200, "DELETE", "/lists/"+pub+"/items/"+ids[0]["id"].(string), nil, alice)
	s.mustCall(200, "DELETE", "/lists/"+priv, nil, alice)
	if code, _ := s.call("GET", "/lists/"+priv, nil, alice); code != 404 {
		t.Errorf("deleted list: %d", code)
	}
}

// A new chapter reaches exactly the manga's readers: over UDP (only to
// subscribers registered with their token) and in the manga's chat room.
func TestChapterReleaseReachesOnlyReaders(t *testing.T) {
	s := startStack(t)
	reader, other := s.login("reader1", "password123"), s.login("reader2", "password123")
	// A manga nobody has in their library yet, so reader1 is its only reader
	// (a fixed index used to collide with the seeded libraries now and then)
	var mangaID string
	if err := s.db.QueryRow(`SELECT id FROM manga WHERE id NOT IN (SELECT manga_id FROM reading_progress)
		ORDER BY title LIMIT 1`).Scan(&mangaID); err != nil {
		t.Fatal(err)
	}
	m := data(s.mustCall(200, "GET", "/manga/"+mangaID, nil, ""))
	known := int(m["total_chapters"].(float64))

	subscribe := func(token string) net.Conn {
		c, err := net.Dial("udp", s.udpAddr)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { c.Close() })
		msg := "REGISTER"
		if token != "" {
			msg += " " + token
		}
		c.Write([]byte(msg))
		buf := make([]byte, 64)
		c.SetReadDeadline(time.Now().Add(3 * time.Second))
		if n, err := c.Read(buf); err != nil || string(buf[:n]) != "REGISTERED" {
			t.Fatalf("register: %q %v", buf[:n], err)
		}
		return c
	}
	// Put the manga in reader1's library, and wait until that update's own
	// (asynchronous) progress broadcast has gone out, so it can't be mistaken
	// for the chapter release below
	probe := subscribe("")
	s.mustCall(200, "PUT", "/users/progress", map[string]interface{}{"manga_id": mangaID, "current_chapter": 1}, reader)
	probe.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := probe.Read(make([]byte, 4096)); err != nil {
		t.Fatalf("progress broadcast: %v", err)
	}

	readerSub, otherSub, anonSub := subscribe(reader), subscribe(other), subscribe("")

	room, _, err := gws.DefaultDialer.Dial(s.wsURL("/ws/chat?room_id=manga_"+mangaID+"&token="+other), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer room.Close()
	readWS(t, room)

	if code, _ := s.call("POST", "/admin/manga/"+mangaID+"/chapters", map[string]int{"chapter": known + 1}, reader); code != 403 {
		t.Errorf("non-admin release: %d, want 403", code)
	}
	admin := s.login("admin", "admin123")
	rel := data(s.mustCall(201, "POST", "/admin/manga/"+mangaID+"/chapters", map[string]int{"chapter": known + 1}, admin))
	if rel["chapter"] != float64(known+1) || rel["previous_latest"] != float64(known) || rel["notified_readers"] != float64(1) {
		t.Errorf("release = %v", rel)
	}

	buf := make([]byte, 4096)
	readerSub.SetReadDeadline(time.Now().Add(3 * time.Second))
	n, err := readerSub.Read(buf)
	var note map[string]interface{}
	if err != nil || json.Unmarshal(buf[:n], &note) != nil || note["type"] != "chapter_release" ||
		note["chapter"] != float64(known+1) || note["manga_title"] != m["title"] || note["user_ids"] != nil {
		t.Errorf("reader got %s (%v)", buf[:n], err)
	}
	for name, c := range map[string]net.Conn{"reader2 (not reading it)": otherSub, "anonymous": anonSub} {
		c.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
		if n, err := c.Read(buf); err == nil {
			t.Errorf("%s received %s", name, buf[:n])
		}
	}

	// The room may first get the (asynchronous) notice of the progress update above
	want := fmt.Sprintf("Chapter %d", known+1)
	var notice map[string]interface{}
	for i := 0; i < 5 && !strings.Contains(fmt.Sprint(notice["content"]), want); i++ {
		notice = readWS(t, room)
	}
	if notice["type"] != "system" || !strings.Contains(fmt.Sprint(notice["content"]), want) {
		t.Errorf("chat room notice = %v", notice)
	}
	if got := data(s.mustCall(200, "GET", "/manga/"+mangaID, nil, ""))["total_chapters"]; got != float64(known+1) {
		t.Errorf("total_chapters = %v", got)
	}
	if code, _ := s.call("POST", "/admin/manga/"+mangaID+"/chapters", map[string]int{"chapter": known + 1}, admin); code != 409 {
		t.Errorf("releasing the same chapter twice: %d, want 409", code)
	}

	// A forged token can't register for someone else's notifications
	c, _ := net.Dial("udp", s.udpAddr)
	defer c.Close()
	c.Write([]byte("REGISTER not-a-real-token"))
	c.SetReadDeadline(time.Now().Add(3 * time.Second))
	if n, _ := c.Read(buf); !strings.HasPrefix(string(buf[:n]), "ERROR") {
		t.Errorf("forged token reply = %q", buf[:n])
	}
}

func TestRateLimits(t *testing.T) {
	// A slow refill (1 token/s) so the result doesn't depend on how fast this
	// machine gets through the requests: with 20/s a slow (-race, loaded CI)
	// run could refill as fast as it spent and never hit the limit
	s := startStack(t, func(c *server.Config) {
		c.RateLimit, c.RateBurst = 1, 20
		c.AuthRateLimit, c.AuthRateBurst = 5, 5
	})

	// Password guessing: the 6th login attempt in a minute is refused
	codes := []int{}
	for i := 0; i < 6; i++ {
		code, _ := s.call("POST", "/auth/login", map[string]string{"username": "reader1", "password": fmt.Sprint("guess", i)}, "")
		codes = append(codes, code)
	}
	if codes[4] != 401 || codes[5] != 429 {
		t.Errorf("login attempts = %v, want 401s then 429", codes)
	}
	// Correct logins don't use up that budget (another client IP isn't
	// available here, so check with a fresh stack)
	s2 := startStack(t, func(c *server.Config) { c.AuthRateLimit, c.AuthRateBurst = 5, 5 })
	for i := 0; i < 8; i++ {
		if code, _ := s2.call("POST", "/auth/login", map[string]string{"username": "reader1", "password": "password123"}, ""); code != 200 {
			t.Fatalf("correct login %d: %d (successful logins must not be rate limited)", i+1, code)
		}
	}

	// General limit, and a fake X-Forwarded-For can't reset it
	limited := 0
	for i := 0; i < 30; i++ {
		req, _ := http.NewRequest("GET", s.base+"/manga?limit=1", nil)
		req.Header.Set("X-Forwarded-For", fmt.Sprintf("203.0.113.%d", i))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests {
			limited++
			if resp.Header.Get("Retry-After") == "" {
				t.Error("429 without Retry-After")
			}
		}
	}
	if limited == 0 {
		t.Error("30 rapid requests with burst 20 were never limited (X-Forwarded-For spoofing?)")
	}
}
