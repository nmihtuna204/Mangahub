// Package views - Library View
// Tabbed shelf layout for user's manga library
// Layout:
//   Reading  |  Plan  |  Completed  |  Dropped
//   ─────────────────────────────────────────────
//   [x] One Piece           Ch: 1093/1100   ★★★★★
//   [ ] Jujutsu Kaisen      Ch: 260/???     ★★★★☆
//   ─────────────────────────────────────────────
//   [Enter] Details  [d] Delete  [u] Update  [Tab] Next
package views

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"mangahub/internal/tui/api"
	"mangahub/internal/tui/styles"
)

// =====================================
// LIBRARY TABS
// =====================================

// LibraryTab represents a library filter tab
type LibraryTab int

const (
	TabReading LibraryTab = iota
	TabPlan
	TabCompleted
	TabOnHold
	TabDropped
)

var tabNames = []string{"Reading", "Plan", "Completed", "On-Hold", "Dropped"}
var tabStatuses = []string{"reading", "planning", "completed", "on_hold", "dropped"}

// =====================================
// LIBRARY MODEL
// =====================================

// LibraryModel holds the library view state
type LibraryModel struct {
	// Window dimensions
	width  int
	height int

	// Theme
	theme *styles.Theme

	// Data
	entries []api.LibraryEntry

	// Filtered views per tab
	filteredEntries []api.LibraryEntry

	// Current tab
	activeTab LibraryTab

	// Selection
	selectedIndex int
	cursor        int

	// Scroll offset
	scrollOffset int
	visibleRows  int

	// Loading
	loading bool

	// Components
	spinner spinner.Model

	// Error
	lastError error

	// API client
	client *api.Client
}

// =====================================
// MESSAGES
// =====================================

// LibraryDataLoadedMsg signals library data loaded
type LibraryDataLoadedMsg struct {
	Entries []api.LibraryEntry
}

// LibraryErrorMsg signals an error
type LibraryErrorMsg struct {
	Error error
}

// =====================================
// CONSTRUCTOR
// =====================================

// NewLibrary creates a new library model
func NewLibrary() LibraryModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = styles.DefaultTheme.Spinner

	return LibraryModel{
		theme:       styles.DefaultTheme,
		spinner:     s,
		client:      api.GetClient(),
		loading:     true,
		activeTab:   TabReading,
		visibleRows: 10,
	}
}

// =====================================
// BUBBLE TEA INTERFACE
// =====================================

// Init initializes the library view
func (m LibraryModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.loadLibrary,
	)
}

// loadLibrary fetches the user's library
func (m LibraryModel) loadLibrary() tea.Msg {
	ctx := context.Background()

	entries, err := m.client.GetLibrary(ctx)
	if err != nil {
		return LibraryErrorMsg{Error: err}
	}

	return LibraryDataLoadedMsg{Entries: entries}
}

// Update handles messages
func (m LibraryModel) Update(msg tea.Msg) (LibraryModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Calculate visible rows based on height
		m.visibleRows = (m.height - 10) / 2 // Account for headers/footers
		if m.visibleRows < 3 {
			m.visibleRows = 3
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			m.selectedIndex++
			m = m.clampSelection()
			m = m.updateScroll()

		case "k", "up":
			m.selectedIndex--
			m = m.clampSelection()
			m = m.updateScroll()

		case "tab":
			m.activeTab = (m.activeTab + 1) % LibraryTab(len(tabNames))
			m.selectedIndex = 0
			m.scrollOffset = 0
			m = m.filterEntries()

		case "shift+tab":
			if m.activeTab == 0 {
				m.activeTab = LibraryTab(len(tabNames) - 1)
			} else {
				m.activeTab--
			}
			m.selectedIndex = 0
			m.scrollOffset = 0
			m = m.filterEntries()

		case "g", "home":
			m.selectedIndex = 0
			m.scrollOffset = 0

		case "G", "end":
			m.selectedIndex = len(m.filteredEntries) - 1
			m = m.updateScroll()

		case "r":
			// Refresh
			m.loading = true
			return m, m.loadLibrary

		case "d":
			// Delete (would trigger confirmation)
			if m.selectedIndex < len(m.filteredEntries) {
				// TODO: Implement delete confirmation
			}

		case "u":
			// Update progress (would open progress modal)
			if m.selectedIndex < len(m.filteredEntries) {
				// TODO: Implement progress update
			}
		}

	case LibraryDataLoadedMsg:
		m.entries = msg.Entries
		m.loading = false
		m = m.filterEntries()

	case LibraryErrorMsg:
		m.lastError = msg.Error
		m.loading = false

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// filterEntries filters entries by current tab
func (m LibraryModel) filterEntries() LibraryModel {
	m.filteredEntries = nil
	targetStatus := tabStatuses[m.activeTab]

	for _, entry := range m.entries {
		if entry.Status == targetStatus {
			m.filteredEntries = append(m.filteredEntries, entry)
		}
	}

	m = m.clampSelection()
	return m
}

// clampSelection ensures selection is within bounds
func (m LibraryModel) clampSelection() LibraryModel {
	maxIndex := len(m.filteredEntries) - 1
	if maxIndex < 0 {
		maxIndex = 0
	}
	if m.selectedIndex < 0 {
		m.selectedIndex = 0
	}
	if m.selectedIndex > maxIndex {
		m.selectedIndex = maxIndex
	}
	return m
}

// updateScroll updates scroll offset based on selection
func (m LibraryModel) updateScroll() LibraryModel {
	if m.selectedIndex < m.scrollOffset {
		m.scrollOffset = m.selectedIndex
	}
	if m.selectedIndex >= m.scrollOffset+m.visibleRows {
		m.scrollOffset = m.selectedIndex - m.visibleRows + 1
	}
	return m
}

// View renders the library view
func (m LibraryModel) View() string {
	// Render tabs
	tabs := m.renderTabs()

	// Render content
	content := m.renderContent()

	// Render footer hints
	footer := m.renderFooter()

	return lipgloss.JoinVertical(lipgloss.Left, tabs, content, footer)
}

// =====================================
// COMPONENT RENDERERS
// =====================================

// renderTabs renders the tab bar
func (m LibraryModel) renderTabs() string {
	var tabs []string

	for i, name := range tabNames {
		// Count entries for this tab
		count := 0
		for _, entry := range m.entries {
			if entry.Status == tabStatuses[i] {
				count++
			}
		}

		// Build tab label
		label := fmt.Sprintf(" %s (%d) ", name, count)

		// Apply style
		var style lipgloss.Style
		if LibraryTab(i) == m.activeTab {
			style = m.theme.ActiveTab
		} else {
			style = m.theme.Tab
		}

		tabs = append(tabs, style.Render(label))
	}

	// Join tabs with separator
	tabBar := lipgloss.JoinHorizontal(lipgloss.Bottom, tabs...)

	// Add underline
	underline := m.theme.DimText.Render(repeatString("─", m.width-4))

	return tabBar + "\n" + underline
}

// renderContent renders the manga list
func (m LibraryModel) renderContent() string {
	if m.loading {
		return m.theme.Container.Width(m.width - 4).Height(m.visibleRows + 2).Render(
			m.spinner.View() + " Loading library...")
	}

	if len(m.filteredEntries) == 0 {
		emptyMsg := fmt.Sprintf("No manga in '%s' shelf.\n\nAdd manga from Search or Browse.",
			tabNames[m.activeTab])
		return m.theme.Container.Width(m.width - 4).Height(m.visibleRows + 2).Render(
			m.theme.DimText.Render(emptyMsg))
	}

	// Render visible entries
	var rows []string

	// Header row
	headerStyle := m.theme.DimText.Bold(true)
	header := fmt.Sprintf("  %-30s %-15s %-12s",
		headerStyle.Render("TITLE"),
		headerStyle.Render("PROGRESS"),
		headerStyle.Render("RATING"))
	rows = append(rows, header)

	// Separator
	rows = append(rows, m.theme.DimText.Render(repeatString("─", m.width-8)))

	// Entry rows
	endIndex := m.scrollOffset + m.visibleRows
	if endIndex > len(m.filteredEntries) {
		endIndex = len(m.filteredEntries)
	}

	for i := m.scrollOffset; i < endIndex; i++ {
		entry := m.filteredEntries[i]
		rows = append(rows, m.renderEntryRow(i, entry))
	}

	// Scroll indicator
	if len(m.filteredEntries) > m.visibleRows {
		scrollInfo := fmt.Sprintf("  Showing %d-%d of %d",
			m.scrollOffset+1, endIndex, len(m.filteredEntries))
		rows = append(rows, m.theme.DimText.Render(scrollInfo))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return m.theme.Container.Width(m.width - 4).Render(content)
}

// renderEntryRow renders a single library entry row
func (m LibraryModel) renderEntryRow(index int, entry api.LibraryEntry) string {
	// Selection indicator
	prefix := "  "
	style := m.theme.ListItem
	if index == m.selectedIndex {
		prefix = "▶ "
		style = m.theme.ListItemSelected
	}

	// Title (truncated)
	title := truncateLib(entry.Manga.Title, 28)

	// Progress
	var progress string
	if entry.TotalChapters > 0 {
		progress = fmt.Sprintf("Ch. %d/%d", entry.CurrentChapter, entry.TotalChapters)
	} else {
		progress = fmt.Sprintf("Ch. %d/???", entry.CurrentChapter)
	}

	// Progress bar
	var progressPct float64
	if entry.TotalChapters > 0 {
		progressPct = float64(entry.CurrentChapter) / float64(entry.TotalChapters)
	}
	progressBar := styles.RenderProgressBar(progressPct, 6)

	// Rating
	var rating string
	if entry.Rating > 0 {
		rating = styles.RenderRating(entry.Rating, true) // 10-scale to 5-star
	} else {
		rating = m.theme.DimText.Render("Unrated")
	}

	// Build row
	row := fmt.Sprintf("%s%-28s %-8s %s  %s",
		prefix, title, progress, progressBar, rating)

	return style.Render(row)
}

// renderFooter renders the action hints footer
func (m LibraryModel) renderFooter() string {
	hints := []string{
		styles.RenderKeyHint("Enter", "Details"),
		styles.RenderKeyHint("u", "Update"),
		styles.RenderKeyHint("d", "Delete"),
		styles.RenderKeyHint("Tab", "Next Tab"),
		styles.RenderKeyHint("r", "Refresh"),
	}

	hintsStr := ""
	for i, hint := range hints {
		if i > 0 {
			hintsStr += m.theme.DimText.Render("  │  ")
		}
		hintsStr += hint
	}

	return m.theme.Footer.Render(hintsStr)
}

// =====================================
// HELPERS
// =====================================

func truncateLib(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func repeatString(s string, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += s
	}
	return result
}

// GetSelectedEntry returns the currently selected entry
func (m LibraryModel) GetSelectedEntry() *api.LibraryEntry {
	if m.selectedIndex >= 0 && m.selectedIndex < len(m.filteredEntries) {
		return &m.filteredEntries[m.selectedIndex]
	}
	return nil
}

// SetWidth sets the library width
func (m *LibraryModel) SetWidth(w int) {
	m.width = w
}

// SetHeight sets the library height
func (m *LibraryModel) SetHeight(h int) {
	m.height = h
	m.visibleRows = (h - 10) / 2
	if m.visibleRows < 3 {
		m.visibleRows = 3
	}
}
