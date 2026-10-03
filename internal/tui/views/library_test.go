package views

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mangahub/internal/tui/api"
)

// fakeAPI records the library requests the view makes
var fakeAPI struct {
	sync.Mutex
	requests []string // "METHOD path body"
}

func TestMain(m *testing.M) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		b, _ := json.Marshal(body)
		fakeAPI.Lock()
		fakeAPI.requests = append(fakeAPI.requests, r.Method+" "+r.URL.Path+" "+string(b))
		fakeAPI.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			w.Write([]byte(`{"success":true,"data":[]}`))
			return
		}
		w.Write([]byte(`{"success":true}`))
	}))
	api.InitClient(srv.URL) // the views use the shared client
	code := m.Run()
	srv.Close()
	os.Exit(code)
}

func lastRequests(t *testing.T) []string {
	t.Helper()
	fakeAPI.Lock()
	defer fakeAPI.Unlock()
	out := fakeAPI.requests
	fakeAPI.requests = nil
	return out
}

func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "backspace":
		return tea.KeyMsg{Type: tea.KeyBackspace}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func loadedLibrary(t *testing.T) LibraryModel {
	t.Helper()
	m := NewLibrary()
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	entries := []api.LibraryEntry{
		{MangaID: "m1", Status: "reading", CurrentChapter: 10},
		{MangaID: "m2", Status: "plan_to_read"},
	}
	entries[0].Manga.Title, entries[0].Manga.TotalChapters = "One Piece", 1100
	entries[1].Manga.Title = "Berserk"
	m, _ = m.Update(LibraryDataLoadedMsg{Entries: entries})
	lastRequests(t)
	return m
}

// run executes a command chain until it produces no further command
func runCmd(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	return cmd()
}

// Regression: the Plan tab filtered on "planning", which the API never returns.
func TestPlanTabShowsPlanToRead(t *testing.T) {
	m := loadedLibrary(t)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if len(m.filteredEntries) != 1 || m.filteredEntries[0].MangaID != "m2" {
		t.Fatalf("Plan tab = %+v, want Berserk", m.filteredEntries)
	}
	// Key 2 (move to Plan) must send the API's status value
	m.activeTab = TabReading
	m = m.filterEntries()
	_, cmd := m.Update(key("2"))
	runCmd(cmd)
	if reqs := lastRequests(t); len(reqs) == 0 || !strings.Contains(reqs[0], `"status":"plan_to_read"`) {
		t.Errorf("key 2 sent %v, want status plan_to_read", reqs)
	}
}

func TestRemoveNeedsConfirmation(t *testing.T) {
	m := loadedLibrary(t)

	m, _ = m.Update(key("d"))
	if !m.IsInputFocused() || !strings.Contains(m.View(), "Remove") {
		t.Fatal("d should open the remove confirmation")
	}
	m, cmd := m.Update(key("n"))
	runCmd(cmd)
	if m.HasModal() || len(lastRequests(t)) != 0 {
		t.Error("n should cancel without removing anything")
	}

	m, _ = m.Update(key("d"))
	m, _ = m.Update(key("x")) // unrelated keys are ignored while confirming
	if !m.HasModal() {
		t.Error("an unrelated key closed the confirmation")
	}
	m, cmd = m.Update(key("y"))
	runCmd(cmd)
	reqs := lastRequests(t)
	if m.HasModal() || len(reqs) < 1 || !strings.HasPrefix(reqs[0], "DELETE /users/library/m1") {
		t.Errorf("y should remove One Piece, got %v", reqs)
	}

	m, _ = m.Update(key("d"))
	m = m.CloseModal() // what the app does on Esc
	if m.HasModal() {
		t.Error("CloseModal left the confirmation open")
	}
}

func TestSetChapterInput(t *testing.T) {
	m := loadedLibrary(t)

	m, _ = m.Update(key("u"))
	if !m.IsInputFocused() || m.chapterInput.Value() != "11" {
		t.Fatalf("u should open the input prefilled with the next chapter, got %q", m.chapterInput.Value())
	}
	if !strings.Contains(m.View(), "of 1100") {
		t.Error("the input should show the manga's total chapters")
	}
	m, _ = m.Update(key("backspace"))
	m, _ = m.Update(key("backspace"))
	m, _ = m.Update(key("4"))
	m, _ = m.Update(key("x")) // letters are rejected
	m, _ = m.Update(key("2"))
	if v := m.chapterInput.Value(); v != "42" {
		t.Fatalf("input = %q, want 42", v)
	}
	m, cmd := m.Update(key("enter"))
	runCmd(cmd)
	reqs := lastRequests(t)
	if m.HasModal() || len(reqs) < 1 || !strings.Contains(reqs[0], `"current_chapter":42`) {
		t.Fatalf("enter sent %v, want chapter 42", reqs)
	}
	if strings.Contains(reqs[0], "status") || strings.Contains(reqs[0], "is_favorite") {
		t.Errorf("setting a chapter must not touch status or favorite: %s", reqs[0])
	}

	// + and - step one chapter without the input
	_, cmd = m.Update(key("+"))
	runCmd(cmd)
	if reqs := lastRequests(t); len(reqs) < 1 || !strings.Contains(reqs[0], `"current_chapter":11`) {
		t.Errorf("+ sent %v, want chapter 11", reqs)
	}
	_, cmd = m.Update(key("-"))
	runCmd(cmd)
	if reqs := lastRequests(t); len(reqs) < 1 || !strings.Contains(reqs[0], `"current_chapter":9`) {
		t.Errorf("- sent %v, want chapter 9", reqs)
	}
}
