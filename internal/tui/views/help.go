// Package views - Help View
// Comprehensive keybinding reference and usage guide
package views

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"mangahub/internal/tui/styles"
)

// =====================================
// HELP MODEL
// =====================================

// HelpModel holds the help view state
type HelpModel struct {
	width  int
	height int
	theme  *styles.Theme
	scroll int
}

// NewHelp creates a new help model
func NewHelp() HelpModel {
	return HelpModel{
		theme: styles.DefaultTheme,
	}
}

// =====================================
// MODEL METHODS
// =====================================

func (m HelpModel) Init() tea.Cmd {
	return nil
}

func (m HelpModel) Update(msg tea.Msg) (HelpModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			m.scroll++
		case "k", "up":
			if m.scroll > 0 {
				m.scroll--
			}
		case "g", "home":
			m.scroll = 0
		case "G", "end":
			m.scroll = 100 // Will be clamped in view
		}
	}

	return m, nil
}

func (m HelpModel) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	var sections []string

	// Title
	title := m.theme.Title.Render("📖 MangaHub Help & Keybindings")
	subtitle := m.theme.DimText.Render("Complete reference for all keyboard shortcuts")
	sections = append(sections, title, subtitle, "")

	// Command Palette section
	sections = append(sections,
		m.renderSection("🎯 Command Palette", []KeyBinding{
			{"Ctrl+P", "Open Command Palette", "Quick access to all commands"},
			{"Esc", "Close palette / Go back", "Return to previous view"},
			{"?", "Show this help", "View all keybindings"},
		}),
	)

	// Navigation section
	sections = append(sections,
		m.renderSection("🧭 Navigation", []KeyBinding{
			{"h", "Dashboard (Home)", "View continue reading & trending"},
			{"s or /", "Search", "Search for manga by title"},
			{"b", "Browse", "Browse manga by category"},
			{"l", "Library", "View your manga library (login required)"},
			{"a", "Activity", "View activity feed"},
			{"c", "Chat", "Open real-time chat (login required)"},
			{"t", "Statistics", "View reading stats & rank (login required)"},
			{"x", "Settings", "App settings & preferences"},
			{"L", "Login/Logout", "Toggle authentication"},
		}),
	)

	// List Navigation section
	sections = append(sections,
		m.renderSection("📋 List Navigation", []KeyBinding{
			{"↑ or k", "Move up", "Navigate to previous item"},
			{"↓ or j", "Move down", "Navigate to next item"},
			{"← or h", "Move left", "Navigate left in grid"},
			{"→ or l", "Move right", "Navigate right in grid"},
			{"PgUp", "Page up", "Scroll up one page"},
			{"PgDn", "Page down", "Scroll down one page"},
			{"Home or g", "Go to top", "Jump to first item"},
			{"End or G", "Go to bottom", "Jump to last item"},
			{"Enter", "Select item", "Open/select current item"},
		}),
	)

	// Tab Navigation section
	sections = append(sections,
		m.renderSection("📑 Tab Navigation", []KeyBinding{
			{"Tab", "Next tab", "Switch to next tab"},
			{"Shift+Tab", "Previous tab", "Switch to previous tab"},
		}),
	)

	// Actions section
	sections = append(sections,
		m.renderSection("⚡ Actions", []KeyBinding{
			{"r", "Refresh", "Reload current view data"},
			{"Enter", "Submit/Confirm", "Submit form or select item"},
			{"Esc", "Cancel/Back", "Cancel action or go back"},
			{"q", "Quit", "Exit MangaHub"},
			{"Ctrl+C", "Force quit", "Emergency exit"},
		}),
	)

	// Form Input section
	sections = append(sections,
		m.renderSection("✍️ Form Input (Search, Login, etc.)", []KeyBinding{
			{"Any key", "Type", "Enter text into focused field"},
			{"Tab", "Next field", "Move to next input field"},
			{"Shift+Tab", "Previous field", "Move to previous input field"},
			{"Backspace", "Delete", "Delete character"},
			{"Ctrl+U", "Clear", "Clear entire field"},
			{"Enter", "Submit", "Submit form"},
		}),
	)

	// Auth View section
	sections = append(sections,
		m.renderSection("🔐 Authentication (L key or Login view)", []KeyBinding{
			{"Tab", "Switch field", "Move between username/password"},
			{"Ctrl+S", "Toggle mode", "Switch between Login and Signup"},
			{"Enter", "Submit", "Login or register"},
			{"Esc", "Guest mode", "Continue without login"},
		}),
	)

	// Chat View section
	sections = append(sections,
		m.renderSection("💬 Chat (c key)", []KeyBinding{
			{"Enter", "Send message", "Send your typed message"},
			{"Tab", "Focus input", "Focus the message input box"},
			{"Esc", "Unfocus/Back", "Unfocus input or go back"},
			{"↑/↓", "Scroll history", "Browse message history"},
			{"c (in detail)", "Join room", "Join manga discussion room"},
		}),
	)

	// Stats View section
	sections = append(sections,
		m.renderSection("📊 Statistics (t key)", []KeyBinding{
			{"View", "Reading stats", "Chapters read, streak, avg/day"},
			{"View", "Rank badge", "Bronze/Silver/Gold/Emerald/Diamond"},
			{"View", "Genre distribution", "Your favorite genres"},
			{"View", "Rank progress", "Progress to next rank"},
			{"r", "Refresh", "Reload statistics"},
		}),
	)

	// Rank System Info
	sections = append(sections,
		m.renderSection("🏆 Rank System", []KeyBinding{
			{"🥉 Bronze", "0-99 chapters", "Beginner reader"},
			{"🥈 Silver", "100-499 chapters", "Regular reader"},
			{"🥇 Gold", "500-999 chapters", "Avid reader"},
			{"💎 Emerald", "1,000-2,499 chapters", "Dedicated reader"},
			{"👑 Diamond", "2,500+ chapters", "Master reader (MAX RANK)"},
		}),
	)

	// Tips section
	sections = append(sections, "", m.theme.Subtitle.Render("💡 Tips:"))
	tips := []string{
		"• Use Ctrl+P anytime to open Command Palette without interfering with text input",
		"• Press ? to view this help page from anywhere",
		"• When typing in forms, global shortcuts are disabled to prevent conflicts",
		"• Press Esc to go back or cancel current action",
		"• Protected views (Library, Stats) will redirect to login if not authenticated",
	}
	for _, tip := range tips {
		sections = append(sections, m.theme.DimText.Render(tip))
	}

	content := strings.Join(sections, "\n")

	// Wrap in container
	return m.theme.Container.
		Width(m.width - 4).
		Height(m.height - 4).
		Render(content)
}

// SetWidth sets the view width
func (m *HelpModel) SetWidth(w int) {
	m.width = w
}

// SetHeight sets the view height
func (m *HelpModel) SetHeight(h int) {
	m.height = h
}

// =====================================
// HELPERS
// =====================================

type KeyBinding struct {
	Key         string
	Action      string
	Description string
}

func (m HelpModel) renderSection(title string, bindings []KeyBinding) string {
	var lines []string
	lines = append(lines, "", m.theme.Subtitle.Render(title))

	for _, kb := range bindings {
		keyStyle := lipgloss.NewStyle().
			Foreground(styles.ColorPrimary).
			Bold(true).
			Width(18)
		actionStyle := lipgloss.NewStyle().
			Width(25)

		line := keyStyle.Render(kb.Key) + "  " +
			actionStyle.Render(kb.Action) + "  " +
			m.theme.DimText.Render(kb.Description)
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}
