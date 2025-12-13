// Package tui - Global Key Bindings
// Phím tắt toàn cục cho TUI application
// Sử dụng bubbles/key cho key binding management
package tui

import (
	"github.com/charmbracelet/bubbles/key"
)

// KeyMap defines global keyboard shortcuts
type KeyMap struct {
	// Navigation
	Quit       key.Binding
	Help       key.Binding
	Back       key.Binding
	Enter      key.Binding

	// View switching
	Dashboard  key.Binding
	Search     key.Binding
	Browse     key.Binding
	Library    key.Binding
	Profile    key.Binding
	Activity   key.Binding

	// List navigation
	Up         key.Binding
	Down       key.Binding
	Left       key.Binding
	Right      key.Binding
	PageUp     key.Binding
	PageDown   key.Binding
	Home       key.Binding
	End        key.Binding

	// Tabs
	NextTab    key.Binding
	PrevTab    key.Binding

	// Actions
	Refresh    key.Binding
	Delete     key.Binding
	Update     key.Binding
	Rate       key.Binding
	Comment    key.Binding
}

// DefaultKeyMap returns the default key bindings
func DefaultKeyMap() KeyMap {
	return KeyMap{
		// === NAVIGATION ===
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc", "backspace"),
			key.WithHelp("esc", "back"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),

		// === VIEW SWITCHING ===
		Dashboard: key.NewBinding(
			key.WithKeys("h"),
			key.WithHelp("h", "home"),
		),
		Search: key.NewBinding(
			key.WithKeys("s", "/"),
			key.WithHelp("s", "search"),
		),
		Browse: key.NewBinding(
			key.WithKeys("b"),
			key.WithHelp("b", "browse"),
		),
		Library: key.NewBinding(
			key.WithKeys("l"),
			key.WithHelp("l", "library"),
		),
		Profile: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "profile"),
		),
		Activity: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "activity"),
		),

		// === LIST NAVIGATION ===
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Left: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←/h", "left"),
		),
		Right: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("→/l", "right"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup", "ctrl+u"),
			key.WithHelp("pgup", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown", "ctrl+d"),
			key.WithHelp("pgdn", "page down"),
		),
		Home: key.NewBinding(
			key.WithKeys("home", "g"),
			key.WithHelp("home", "go to top"),
		),
		End: key.NewBinding(
			key.WithKeys("end", "G"),
			key.WithHelp("end", "go to bottom"),
		),

		// === TABS ===
		NextTab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next tab"),
		),
		PrevTab: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("shift+tab", "prev tab"),
		),

		// === ACTIONS ===
		Refresh: key.NewBinding(
			key.WithKeys("r", "ctrl+r"),
			key.WithHelp("r", "refresh"),
		),
		Delete: key.NewBinding(
			key.WithKeys("d", "delete"),
			key.WithHelp("d", "delete"),
		),
		Update: key.NewBinding(
			key.WithKeys("u"),
			key.WithHelp("u", "update"),
		),
		Rate: key.NewBinding(
			key.WithKeys("R"),
			key.WithHelp("R", "rate"),
		),
		Comment: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "comment"),
		),
	}
}

// ShortHelp returns keybindings to be shown in the mini help view
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.Help,
		k.Quit,
		k.Search,
		k.Library,
		k.Back,
	}
}

// FullHelp returns keybindings for the expanded help view
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		// Navigation column
		{k.Up, k.Down, k.Enter, k.Back},
		// View switching column
		{k.Dashboard, k.Search, k.Library, k.Browse},
		// Actions column
		{k.Refresh, k.Update, k.Rate, k.Comment},
		// Misc column
		{k.Help, k.Quit},
	}
}
