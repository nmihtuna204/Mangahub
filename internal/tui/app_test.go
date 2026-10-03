package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mangahub/internal/tui/views"
)

func press(m Model, k string) Model {
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)})
	return next.(Model)
}

// Regression: global shortcuts (a = activity, c = chat, h = dashboard,
// l = library) swallowed the same keys in views that use them, so "add to
// library" and "join this manga's chat" in the detail view never ran.
func TestViewKeysWinOverGlobalShortcuts(t *testing.T) {
	cases := []struct {
		view View
		key  string
	}{
		{ViewDetail, "a"}, {ViewDetail, "c"}, {ViewDetail, "h"}, {ViewDetail, "l"},
		{ViewBrowse, "h"}, {ViewBrowse, "l"},
		{ViewActivity, "l"},
	}
	for _, c := range cases {
		m := New()
		m.detailModel = views.NewDetail("m1")
		m.currentView, m.previousView = c.view, ViewDashboard
		if got := press(m, c.key).currentView; got != c.view {
			t.Errorf("%q in view %v switched to view %v; the view should handle it", c.key, c.view, got)
		}
	}

	// Elsewhere the global shortcuts still work
	m := New()
	m.currentView = ViewDashboard
	if got := press(m, "a").currentView; got != ViewActivity {
		t.Errorf("a on the dashboard went to %v, want the activity view", got)
	}
	m.currentView = ViewDashboard
	if got := press(m, "l").currentView; got != ViewLibrary && got != ViewAuth {
		t.Errorf("l on the dashboard went to %v, want library (or login first)", got)
	}
}
