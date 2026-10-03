package views

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"mangahub/internal/tui/api"
)

func float(v float64) *float64 { return &v }
func intp(v int) *int          { return &v }

// The API's activity types are comment, rating, progress (no chapter means the
// manga was finished) and list_add.
func TestActivitiesFromAPI(t *testing.T) {
	now := time.Now()
	got := activitiesFromAPI([]api.ActivityEntry{
		{ID: "1", Username: "a", ActivityType: "comment", MangaTitle: "One Piece", CommentText: "great arc", CreatedAt: now},
		{ID: "2", Username: "b", ActivityType: "rating", MangaTitle: "Berserk", Rating: float(9)},
		{ID: "3", Username: "c", ActivityType: "progress", MangaTitle: "Naruto", Chapter: intp(12)},
		{ID: "4", Username: "d", ActivityType: "progress", MangaTitle: "Bleach"},
		{ID: "5", Username: "e", ActivityType: "list_add", MangaTitle: "Monster", CommentText: "Must read"},
	})
	want := []struct {
		typ     ActivityType
		message string
	}{
		{ActivityComment, "great arc"},
		{ActivityRated, ""},
		{ActivityProgress, ""},
		{ActivityCompleted, ""},
		{ActivityListAdd, "Must read"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d activities, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i].Type != w.typ || got[i].Message != w.message {
			t.Errorf("activity %d = type %q message %q, want %q %q", i, got[i].Type, got[i].Message, w.typ, w.message)
		}
	}
	if got[1].Rating != 9 || got[2].Chapter != 12 || !got[0].Timestamp.Equal(now) {
		t.Errorf("rating/chapter/time not carried over: %+v", got)
	}
}

// Regression: when the API failed, or the feed was empty, the view showed
// made-up activities (users and posts that don't exist).
func TestActivityFeedShowsErrorsAndEmptyFeedHonestly(t *testing.T) {
	m := NewActivity()
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})

	m, _ = m.Update(ActivityErrorMsg{Error: errors.New("connection refused")})
	view := m.View()
	if !strings.Contains(view, "Couldn't load the activity feed") || !strings.Contains(view, "connection refused") {
		t.Errorf("error not shown:\n%s", view)
	}
	if strings.Contains(view, "@") {
		t.Errorf("activities shown after a failed load:\n%s", view)
	}

	m, _ = m.Update(ActivityLoadedMsg{Activities: nil})
	if view := m.View(); !strings.Contains(view, "No recent activity") || strings.Contains(view, "Couldn't load") {
		t.Errorf("empty feed:\n%s", view)
	}

	m, _ = m.Update(ActivityLoadedMsg{Activities: activitiesFromAPI([]api.ActivityEntry{
		{Username: "reader1", ActivityType: "progress", MangaTitle: "Naruto", Chapter: intp(3), CreatedAt: time.Now()},
	})})
	if view := m.View(); !strings.Contains(view, "@reader1") || !strings.Contains(view, "Naruto") {
		t.Errorf("real activity not shown:\n%s", view)
	}
}

// Regression: the dashboard matched activity types the API never sends
// (manga_rated, chapter_read, ...), so every entry fell through to the
// default and rendered as e.g. "progress Naruto".
func TestDashboardActivityText(t *testing.T) {
	cases := []struct {
		entry api.ActivityEntry
		want  string
	}{
		{api.ActivityEntry{ActivityType: "rating", MangaTitle: "Berserk", Rating: float(9)}, "rated Berserk 9/10"},
		{api.ActivityEntry{ActivityType: "progress", MangaTitle: "Naruto", Chapter: intp(12)}, "read Ch.12 of Naruto"},
		{api.ActivityEntry{ActivityType: "progress", MangaTitle: "Bleach"}, "completed Bleach"},
		{api.ActivityEntry{ActivityType: "comment", MangaTitle: "One Piece"}, "commented on One Piece"},
		{api.ActivityEntry{ActivityType: "list_add", MangaTitle: "Monster", CommentText: "Must read"}, `added Monster to "Must read"`},
	}
	for _, c := range cases {
		if got := formatActivityAction(c.entry); got != c.want {
			t.Errorf("%s: %q, want %q", c.entry.ActivityType, got, c.want)
		}
	}
}
