// Package views - Command Palette
// Quick command launcher accessible via Ctrl+P
// Provides fuzzy search over all available commands and views
package views

import (
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"mangahub/internal/tui/styles"
)

// =====================================
// COMMAND PALETTE MODEL
// =====================================

// PaletteCommand represents a command in the palette
type PaletteCommand struct {
	ID       string
	Label    string
	Desc     string
	Keys     []string
	Category string
}

// FilterValue implements list.Item interface
func (c PaletteCommand) FilterValue() string {
	return c.Label + " " + c.Desc + " " + strings.Join(c.Keys, " ")
}

// Title implements list.DefaultItem interface
func (c PaletteCommand) Title() string { return c.Label }

// Description implements list.DefaultItem interface
func (c PaletteCommand) Description() string { return c.Desc }

// PaletteModel holds the command palette state
type PaletteModel struct {
	width       int
	height      int
	theme       *styles.Theme
	searchInput textinput.Model
	list        list.Model
	commands    []PaletteCommand
	selected    *PaletteCommand
	visible     bool
}

// CommandSelectedMsg signals a command was selected
type CommandSelectedMsg struct {
	CommandID string
}

// PaletteCloseMsg signals palette should close
type PaletteCloseMsg struct{}

// =====================================
// AVAILABLE COMMANDS
// =====================================

var allCommands = []PaletteCommand{
	// Navigation
	{ID: "goto_dashboard", Label: "Go to Dashboard", Desc: "View home dashboard", Keys: []string{"h"}, Category: "Navigation"},
	{ID: "goto_search", Label: "Go to Search", Desc: "Search for manga", Keys: []string{"s", "/"}, Category: "Navigation"},
	{ID: "goto_browse", Label: "Go to Browse", Desc: "Browse by category", Keys: []string{"b"}, Category: "Navigation"},
	{ID: "goto_library", Label: "Go to Library", Desc: "View your library", Keys: []string{"l"}, Category: "Navigation"},
	{ID: "goto_activity", Label: "Go to Activity", Desc: "View activity feed", Keys: []string{"a"}, Category: "Navigation"},
	{ID: "goto_stats", Label: "Go to Statistics", Desc: "View reading stats & rank", Keys: []string{"t"}, Category: "Navigation"},
	{ID: "goto_settings", Label: "Go to Settings", Desc: "App settings & preferences", Keys: []string{"x"}, Category: "Navigation"},
	{ID: "goto_chat", Label: "Go to Chat", Desc: "Open real-time chat", Keys: []string{"c"}, Category: "Navigation"},

	// Actions
	{ID: "login", Label: "Login / Logout", Desc: "Toggle authentication", Keys: []string{"L"}, Category: "Account"},
	{ID: "refresh", Label: "Refresh Data", Desc: "Reload current view", Keys: []string{"r"}, Category: "Actions"},
	{ID: "help", Label: "Show Help", Desc: "View all keybindings", Keys: []string{"?"}, Category: "Help"},
	{ID: "quit", Label: "Quit Application", Desc: "Exit MangaHub", Keys: []string{"q"}, Category: "System"},

	// List navigation
	{ID: "move_up", Label: "Move Up", Desc: "Navigate to previous item", Keys: []string{"↑", "k"}, Category: "List Navigation"},
	{ID: "move_down", Label: "Move Down", Desc: "Navigate to next item", Keys: []string{"↓", "j"}, Category: "List Navigation"},
	{ID: "select", Label: "Select Item", Desc: "Select current item", Keys: []string{"Enter"}, Category: "List Navigation"},
	{ID: "back", Label: "Go Back", Desc: "Return to previous view", Keys: []string{"Esc"}, Category: "Navigation"},
}

// =====================================
// CONSTRUCTOR
// =====================================

// NewPalette creates a new command palette
func NewPalette() PaletteModel {
	// Create search input
	ti := textinput.New()
	ti.Placeholder = "Type to search commands..."
	ti.Focus()
	ti.CharLimit = 50
	ti.Width = 60

	// Create list
	items := make([]list.Item, len(allCommands))
	for i, cmd := range allCommands {
		items[i] = cmd
	}

	delegate := list.NewDefaultDelegate()
	delegate.SetHeight(1)
	delegate.ShowDescription = true

	l := list.New(items, delegate, 70, 20)
	l.Title = "Command Palette"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false)
	l.Styles.Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(styles.ColorPrimary).
		Padding(0, 1)

	return PaletteModel{
		theme:       styles.DefaultTheme,
		searchInput: ti,
		list:        l,
		commands:    allCommands,
		visible:     false,
	}
}

// =====================================
// MODEL METHODS
// =====================================

func (m PaletteModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m PaletteModel) Update(msg tea.Msg) (PaletteModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width-20, msg.Height-10)

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.visible = false
			return m, func() tea.Msg { return PaletteCloseMsg{} }

		case "enter":
			if i, ok := m.list.SelectedItem().(PaletteCommand); ok {
				m.selected = &i
				m.visible = false
				return m, func() tea.Msg {
					return CommandSelectedMsg{CommandID: i.ID}
				}
			}

		case "ctrl+c":
			// Let parent handle quit
			return m, nil

		default:
			// Update list for filtering
			var cmd tea.Cmd
			m.list, cmd = m.list.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m PaletteModel) View() string {
	if !m.visible {
		return ""
	}

	// Build palette view
	content := m.list.View()

	// Wrap in a box
	box := m.theme.Card.
		Width(m.width-20).
		Height(m.height-10).
		Padding(1, 2).
		Render(content)

	// Center it
	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		box,
	)
}

// =====================================
// PUBLIC METHODS
// =====================================

// Show makes the palette visible
func (m *PaletteModel) Show() {
	m.visible = true
	m.list.ResetFilter()
}

// Hide hides the palette
func (m *PaletteModel) Hide() {
	m.visible = false
}

// IsVisible returns whether the palette is visible
func (m PaletteModel) IsVisible() bool {
	return m.visible
}

// SetWidth sets the view width
func (m *PaletteModel) SetWidth(w int) {
	m.width = w
	m.list.SetSize(w-20, m.height-10)
}

// SetHeight sets the view height
func (m *PaletteModel) SetHeight(h int) {
	m.height = h
	m.list.SetSize(m.width-20, h-10)
}
