// Package views - Manga Search View
// Interactive search with instant results
// Layout:
//
//	┌────────────────────────────────────────────────────────┐
//	│  🔍 SEARCH                                             │
//	│  ┌─────────────────────────────────────────────────┐   │
//	│  │ one piece_                                      │   │
//	│  └─────────────────────────────────────────────────┘   │
//	│                                                        │
//	│  RESULTS (42 found)                                    │
//	│  ┌─────────────────────────────────────────────────┐   │
//	│  │ > ONE PIECE           Eiichiro Oda    ⭐ 9.2   │   │
//	│  │   One Punch Man       ONE             ⭐ 8.9   │   │
//	│  │   One Piece: Side...  Oda/Boichi      ⭐ 8.1   │   │
//	│  └─────────────────────────────────────────────────┘   │
//	│                                                        │
//	│  [↑↓] Navigate  [Enter] View  [Esc] Clear              │
//	└────────────────────────────────────────────────────────┘
package views

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"mangahub/internal/tui/api"
	"mangahub/internal/tui/styles"
	"mangahub/pkg/models"
)

// =====================================
// SEARCH MODEL
// =====================================

// SearchModel holds the search view state
type SearchModel struct {
	// Window dimensions
	width  int
	height int

	// Theme
	theme *styles.Theme

	// Components
	input   textinput.Model
	spinner spinner.Model

	// Results
	results       []models.Manga
	selectedIndex int
	totalResults  int

	// Loading state
	loading   bool
	lastQuery string

	// Debounce
	debounceTimer time.Time

	// Error
	lastError error

	// API client
	client *api.Client
}

// =====================================
// MESSAGES
// =====================================

// SearchResultsMsg carries search results
type SearchResultsMsg struct {
	Query   string
	Results []models.Manga
	Total   int
}

// SearchErrorMsg signals search error
type SearchErrorMsg struct {
	Error error
}

// SearchDebounceMsg triggers debounced search
type SearchDebounceMsg struct {
	Query string
}

// =====================================
// CONSTRUCTOR
// =====================================

// NewSearch creates a new search model
func NewSearch() SearchModel {
	// Create text input
	ti := textinput.New()
	ti.Placeholder = "Search manga by title..."
	ti.Focus()
	ti.CharLimit = 100
	ti.Width = 50
	ti.PromptStyle = styles.DefaultTheme.Primary
	ti.TextStyle = styles.DefaultTheme.Description
	ti.PlaceholderStyle = styles.DefaultTheme.DimText

	// Create spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = styles.DefaultTheme.Spinner

	return SearchModel{
		theme:   styles.DefaultTheme,
		input:   ti,
		spinner: s,
		client:  api.GetClient(),
		results: []models.Manga{},
	}
}

// =====================================
// BUBBLE TEA INTERFACE
// =====================================

// Init initializes the search view
func (m SearchModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles messages
func (m SearchModel) Update(msg tea.Msg) (SearchModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.Width = msg.Width - 16

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if len(m.results) > 0 {
				m.selectedIndex--
				if m.selectedIndex < 0 {
					m.selectedIndex = len(m.results) - 1
				}
			}
		case "down", "j":
			if len(m.results) > 0 {
				m.selectedIndex = (m.selectedIndex + 1) % len(m.results)
			}
		case "enter":
			// Return the selected manga ID
			// Will be handled by parent
			if len(m.results) > 0 && m.selectedIndex < len(m.results) {
				// Navigation will be handled by parent
			}
		case "esc":
			// Clear input
			m.input.SetValue("")
			m.results = []models.Manga{}
			m.totalResults = 0
		default:
			// Update text input
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			cmds = append(cmds, cmd)

			// Trigger debounced search
			query := m.input.Value()
			if query != m.lastQuery && len(query) >= 2 {
				m.lastQuery = query
				m.debounceTimer = time.Now()
				cmds = append(cmds, m.debounceSearch(query))
			}
		}

	case SearchDebounceMsg:
		// Only search if query hasn't changed
		if msg.Query == m.input.Value() {
			m.loading = true
			cmds = append(cmds, m.executeSearch(msg.Query))
		}

	case SearchResultsMsg:
		if msg.Query == m.input.Value() {
			m.results = msg.Results
			m.totalResults = msg.Total
			m.loading = false
			m.selectedIndex = 0
		}

	case SearchErrorMsg:
		m.lastError = msg.Error
		m.loading = false

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// debounceSearch creates a debounced search command
func (m SearchModel) debounceSearch(query string) tea.Cmd {
	return tea.Tick(300*time.Millisecond, func(t time.Time) tea.Msg {
		return SearchDebounceMsg{Query: query}
	})
}

// executeSearch performs the actual search
func (m SearchModel) executeSearch(query string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		results, total, err := m.client.SearchManga(ctx, query, 1, 20)
		if err != nil {
			return SearchErrorMsg{Error: err}
		}
		return SearchResultsMsg{
			Query:   query,
			Results: results,
			Total:   total,
		}
	}
}

// View renders the search view
func (m SearchModel) View() string {
	var sections []string

	// ===== HEADER =====
	header := m.theme.PanelHeader.Render("🔍 SEARCH")
	sections = append(sections, header)

	// ===== INPUT BOX =====
	inputBox := m.renderInputBox()
	sections = append(sections, inputBox)

	// ===== RESULTS =====
	results := m.renderResults()
	sections = append(sections, results)

	// ===== HELP =====
	help := m.renderHelp()
	sections = append(sections, help)

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)
	return m.theme.Container.Width(m.width - 4).Render(content)
}

// =====================================
// RENDERERS
// =====================================

func (m SearchModel) renderInputBox() string {
	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.ColorPrimary).
		Padding(0, 1).
		Width(m.width - 10)

	return inputStyle.Render(m.input.View()) + "\n"
}

func (m SearchModel) renderResults() string {
	// Results header
	var headerText string
	if m.loading {
		headerText = fmt.Sprintf("SEARCHING... %s", m.spinner.View())
	} else if len(m.results) > 0 {
		headerText = fmt.Sprintf("RESULTS (%d found)", m.totalResults)
	} else if m.input.Value() != "" {
		headerText = "NO RESULTS"
	} else {
		headerText = "TYPE TO SEARCH"
	}

	header := m.theme.PanelHeader.Render(headerText)

	// No results state
	if len(m.results) == 0 {
		if m.input.Value() == "" {
			hint := m.theme.DimText.Render("Enter at least 2 characters to search...")
			return header + "\n" + hint
		} else if !m.loading {
			hint := m.theme.DimText.Render("No manga found matching your search.")
			return header + "\n" + hint
		}
		return header + "\n"
	}

	// Build results list
	listStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.ColorDim).
		Width(m.width - 10).
		MaxHeight(m.height - 14)

	var rows []string
	maxVisible := min(10, len(m.results))

	for i := 0; i < maxVisible; i++ {
		manga := m.results[i]
		row := m.renderResultRow(manga, i == m.selectedIndex)
		rows = append(rows, row)
	}

	// Show truncation indicator
	if len(m.results) > maxVisible {
		more := m.theme.DimText.Render(fmt.Sprintf("  ... and %d more", len(m.results)-maxVisible))
		rows = append(rows, more)
	}

	list := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return header + "\n" + listStyle.Render(list)
}

func (m SearchModel) renderResultRow(manga models.Manga, selected bool) string {
	// Selector
	selector := "  "
	if selected {
		selector = m.theme.Primary.Render("> ")
	}

	// Title (truncated)
	title := manga.Title
	maxTitleLen := 30
	if len(title) > maxTitleLen {
		title = title[:maxTitleLen-3] + "..."
	}

	// Style title based on selection
	var titleStyle lipgloss.Style
	if selected {
		titleStyle = m.theme.Title.Bold(true)
	} else {
		titleStyle = m.theme.Description
	}
	titleText := titleStyle.Render(fmt.Sprintf("%-30s", title))

	// Author (truncated)
	author := manga.Author
	maxAuthorLen := 20
	if len(author) > maxAuthorLen {
		author = author[:maxAuthorLen-3] + "..."
	}
	authorText := m.theme.DimText.Render(fmt.Sprintf("%-20s", author))

	// Status indicator
	var statusIndicator string
	switch strings.ToLower(manga.Status) {
	case "ongoing":
		statusIndicator = m.theme.Success.Render("●")
	case "completed":
		statusIndicator = m.theme.Secondary.Render("✓")
	default:
		statusIndicator = m.theme.DimText.Render("○")
	}

	// Combine
	return selector + titleText + "  " + authorText + "  " + statusIndicator
}

func (m SearchModel) renderHelp() string {
	helpItems := []string{
		m.theme.Key.Render("[↑↓]") + " " + m.theme.DimText.Render("Navigate"),
		m.theme.Key.Render("[Enter]") + " " + m.theme.DimText.Render("View Details"),
		m.theme.Key.Render("[Esc]") + " " + m.theme.DimText.Render("Clear"),
	}
	return "\n" + lipgloss.JoinHorizontal(lipgloss.Center, helpItems...)
}

// =====================================
// PUBLIC METHODS
// =====================================

// GetSelectedManga returns the currently selected manga
func (m SearchModel) GetSelectedManga() *models.Manga {
	if len(m.results) > 0 && m.selectedIndex < len(m.results) {
		return &m.results[m.selectedIndex]
	}
	return nil
}

// Focus focuses the search input
func (m *SearchModel) Focus() tea.Cmd {
	return m.input.Focus()
}

// Blur removes focus from search input
func (m *SearchModel) Blur() {
	m.input.Blur()
}

// SetWidth sets the view width
func (m *SearchModel) SetWidth(w int) {
	m.width = w
	m.input.Width = w - 16
}

// SetHeight sets the view height
func (m *SearchModel) SetHeight(h int) {
	m.height = h
}

// ClearResults clears the search results
func (m *SearchModel) ClearResults() {
	m.input.SetValue("")
	m.results = []models.Manga{}
	m.totalResults = 0
	m.selectedIndex = 0
}

// IsInputFocused reports whether the search input is focused.
func (m SearchModel) IsInputFocused() bool {
	return m.input.Focused()
}
