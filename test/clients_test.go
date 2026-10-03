package test

import (
	"bytes"
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	climanga "mangahub/internal/cli/manga"
	cliprogress "mangahub/internal/cli/progress"
	"mangahub/internal/tui/api"
	"mangahub/internal/tui/network"
)

// pointViperAt makes the TUI API client and the CLI talk to the test stack.
func pointViperAt(t *testing.T, s *stack) {
	t.Helper()
	u, _ := url.Parse(s.base)
	viper.Set("server.host", u.Hostname())
	viper.Set("server.http_port", u.Port())
	viper.Set("user.token", "")
	t.Cleanup(viper.Reset)
}

func runCmd(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	if cmd == nil {
		return nil
	}
	ch := make(chan tea.Msg, 1)
	go func() { ch <- cmd() }()
	select {
	case m := <-ch:
		return m
	case <-time.After(8 * time.Second):
		t.Fatal("tea command timed out")
		return nil
	}
}

// The TUI's own HTTP, UDP and WebSocket code against the real API.
func TestTUIClientsAgainstTheStack(t *testing.T) {
	s := startStack(t)
	pointViperAt(t, s)
	ctx := context.Background()
	c := api.NewClient()

	if _, err := c.Login(ctx, "reader1", "password123"); err != nil {
		t.Fatal(err)
	}

	page1, total, err := c.SearchManga(ctx, "", 1, 5)
	page2, _, _ := c.SearchManga(ctx, "", 2, 5)
	if err != nil || total <= 5 || len(page2) != 5 || page1[0].ID == page2[0].ID {
		t.Errorf("search pagination: total %d, pages overlap or wrong size (%v)", total, err)
	}
	byGenre, _, err := c.SearchMangaByGenre(ctx, "Slice of Life", 1, 20)
	if err != nil || len(byGenre) == 0 {
		t.Fatalf("genre browse: %v", err)
	}
	mangaID := byGenre[0].ID

	if err := c.SubmitRating(ctx, mangaID, 9, "tui"); err != nil {
		t.Fatal(err)
	}
	sum, err := c.GetRatings(ctx, mangaID)
	if err != nil || sum.RatingCount != 1 || sum.AverageRating != 9 {
		t.Errorf("GetRatings = %+v, %v (always showed 0 before)", sum, err)
	}
	if err := c.SubmitRating(ctx, mangaID, 42, ""); err == nil {
		t.Error("invalid rating reported as success")
	}

	fav := true
	if err := c.UpdateProgress(ctx, mangaID, 5, "reading", &fav); err != nil {
		t.Fatal(err)
	}
	if err := c.UpdateLibraryStatus(ctx, mangaID, "on_hold"); err != nil {
		t.Errorf("status-only update: %v", err)
	}
	if err := c.ToggleFavorite(ctx, mangaID, true); err != nil {
		t.Errorf("favorite-only update: %v (was 500)", err)
	}
	lib, _ := c.GetLibrary(ctx)
	for _, e := range lib {
		if e.MangaID == mangaID && (e.CurrentChapter != 5 || !e.IsFavorite || e.Status != "on_hold") {
			t.Errorf("library entry after TUI actions = %+v", e)
		}
	}
	if acts, err := c.GetActivities(ctx, 50); err != nil || len(acts) == 0 {
		t.Errorf("activities: %d, %v", len(acts), err)
	}

	// UDP notifications. The bridge sends asynchronously, so a notification
	// for one of the updates above may still arrive first: wait for chapter 12.
	// (This used to call runCmd, which can t.Fatal, from another goroutine.)
	udp := network.NewUDPListener()
	if _, ok := runCmd(t, udp.Start(s.udpAddr, c.GetToken())).(network.UDPConnectedMsg); !ok {
		t.Fatal("UDP listener did not register")
	}
	defer runCmd(t, udp.Stop())
	if err := c.UpdateProgress(ctx, mangaID, 12, "", nil); err != nil {
		t.Fatal(err)
	}
	var note network.UDPNotificationMsg
	for i := 0; note.Chapter != 12; i++ {
		msg, ok := runCmd(t, udp.WaitForPacket()).(network.UDPNotificationMsg)
		if !ok || i == 10 {
			t.Fatalf("UDP listener: got %#v, want the notification for chapter 12", msg)
		}
		note = msg
	}
	if note.Type != "progress_update" || note.MangaName == "" {
		t.Errorf("UDP notification = %+v", note)
	}

	// WebSocket chat
	room := "manga_" + mangaID
	ws := network.NewWSClient()
	wsBase := "ws" + strings.TrimPrefix(s.base, "http")
	if _, ok := runCmd(t, ws.Connect(wsBase, c.GetToken(), room)).(network.WSConnectedMsg); !ok {
		t.Fatal("WebSocket connect failed")
	}
	join, ok := runCmd(t, ws.ListenForMessages()).(network.ChatMessageMsg)
	if !ok || join.Type != "join" || time.Since(join.Timestamp) > time.Minute {
		t.Errorf("join notice = %+v (timestamps used to break parsing)", join)
	}
	if err := c.UpdateProgress(ctx, mangaID, 13, "", nil); err != nil {
		t.Fatal(err)
	}
	// Skip a late notice about chapter 12 if one is still on its way
	var notice network.ChatMessageMsg
	for i := 0; !strings.Contains(notice.Content, "chapter 13"); i++ {
		msg, ok := runCmd(t, ws.ListenForMessages()).(network.ChatMessageMsg)
		if !ok || i == 10 {
			t.Fatalf("bridge notice: got %#v, want the notice for chapter 13", msg)
		}
		notice = msg
	}
	if notice.Type != "system" {
		t.Errorf("bridge notice = %+v", notice)
	}
	runCmd(t, ws.SendMessage(room, "hello"))
	echo, _ := runCmd(t, ws.ListenForMessages()).(network.ChatMessageMsg)
	if echo.Type != "message" || echo.Content != "hello" {
		t.Errorf("echo = %+v", echo)
	}
	// The TUI loads this history when entering the room
	eventually(t, "saved chat message", func() bool {
		history, err := c.GetRoomMessages(ctx, room, 50)
		return err == nil && len(history) == 1 && history[0].Content == "hello" && history[0].Username == "reader1"
	})
	runCmd(t, ws.Disconnect())
	if msg := runCmd(t, ws.Reconnect()); msg != nil {
		t.Errorf("reconnected after logout: %#v", msg)
	}

	// Registration returns a profile without a token; the client logs in after it
	c.ClearToken()
	u, err := c.Register(ctx, "tuiuser", "tuiuser@example.com", "password123")
	if err != nil || u == nil || u.Username != "tuiuser" || c.GetToken() == "" {
		t.Errorf("Register = %+v, %v (a nil user crashed the TUI)", u, err)
	}
}

// executeCLI runs a CLI command group with args and returns what it printed.
func executeCLI(t *testing.T, group *cobra.Command, args ...string) (string, error) {
	t.Helper()
	resetFlags(group)
	var out bytes.Buffer
	group.SetOut(&out)
	group.SetErr(&out)
	group.SetArgs(args)
	err := group.Execute()
	return out.String(), err
}

// resetFlags puts every flag of cmd and its subcommands back to its default.
// The CLI's commands are package-level values, so a flag set by one Execute
// stayed set for the next (e.g. "view" after "view --manga-id x"), which also
// broke this test on a second run in the same process (go test -count=2).
func resetFlags(cmd *cobra.Command) {
	reset := func(f *pflag.Flag) {
		if sv, ok := f.Value.(pflag.SliceValue); ok {
			sv.Replace(nil)
		} else {
			f.Value.Set(f.DefValue)
		}
		f.Changed = false
	}
	cmd.Flags().VisitAll(reset)
	cmd.PersistentFlags().VisitAll(reset)
	for _, sub := range cmd.Commands() {
		resetFlags(sub)
	}
}

func TestCLIInfoAndProgressCommands(t *testing.T) {
	s := startStack(t)
	pointViperAt(t, s)
	// A new user, so the library starts empty (the seed gives reader1 a library)
	s.mustCall(201, "POST", "/auth/register", map[string]string{"username": "cliuser", "email": "cli@example.com", "password": "password123"}, "")
	tok := s.login("cliuser", "password123")
	viper.Set("user.token", tok)
	m := s.manga()[0]
	mangaID := m["id"].(string)

	out, err := executeCLI(t, climanga.MangaCmd, "info", mangaID)
	if err != nil || !strings.Contains(out, m["title"].(string)) || !strings.Contains(out, "Chapters:") || !strings.Contains(out, "Genres:") {
		t.Errorf("manga info: %v\n%s", err, out)
	}
	if _, err := executeCLI(t, climanga.MangaCmd, "info", "no-such-id"); err == nil || !strings.Contains(err.Error(), "manga not found") {
		t.Errorf("manga info unknown id: %v", err)
	}

	out, err = executeCLI(t, cliprogress.ProgressCmd, "view")
	if err != nil || !strings.Contains(out, "library is empty") {
		t.Errorf("progress view (empty): %v\n%s", err, out)
	}

	// progress update prints with fmt.Printf; check its effect through the API
	if _, err := executeCLI(t, cliprogress.ProgressCmd, "update", "--manga-id", mangaID, "--chapter", "9", "--rating", "8"); err != nil {
		t.Fatalf("progress update: %v", err)
	}
	if e := s.entry(tok, mangaID); e["current_chapter"] != float64(9) {
		t.Errorf("chapter after CLI update = %v", e["current_chapter"])
	}
	summary := data(s.mustCall(200, "GET", "/manga/"+mangaID+"/ratings", nil, ""))["summary"].(map[string]interface{})
	if summary["rating_count"] != float64(1) {
		t.Error("--rating was not saved (it used to be ignored)")
	}

	out, err = executeCLI(t, cliprogress.ProgressCmd, "view")
	if err != nil || !strings.Contains(out, "Reading progress (1 manga)") || !strings.Contains(out, "ch 9/") {
		t.Errorf("progress view: %v\n%s", err, out)
	}
	out, err = executeCLI(t, cliprogress.ProgressCmd, "view", "--manga-id", mangaID)
	if err != nil || !strings.Contains(out, "Progress:") || !strings.Contains(out, "Status:    reading") {
		t.Errorf("progress view --manga-id: %v\n%s", err, out)
	}
	if _, err := executeCLI(t, cliprogress.ProgressCmd, "view", "--manga-id", "not-in-library"); err == nil {
		t.Error("progress view for a manga not in the library should fail")
	}

	viper.Set("user.token", "")
	if _, err := executeCLI(t, cliprogress.ProgressCmd, "view"); err == nil || !strings.Contains(err.Error(), "not logged in") {
		t.Errorf("progress view logged out: %v", err)
	}
}
