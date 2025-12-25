// Package views - Browse View
// Category-based manga discovery
// Layout:
//
//	┌────────────────────────────────────────────────────────┐
//	│  📚 BROWSE BY CATEGORY                                │
//	│                                                       │
//	│  ┌──────────┐  ┌──────────┐  ┌──────────┐            │
//	│  │  ACTION  │  │ ROMANCE  │  │ COMEDY   │            │
//	│  │    ⚔️    │  │    💕    │  │    😄    │           │
//	│  └──────────┘  └──────────┘  └──────────┘             │
//	│  ┌──────────┐  ┌──────────┐  ┌──────────┐             │
//	│  │ FANTASY  │  │  HORROR  │  │  SCI-FI  │             │
//	│  │    🧙    │  │    👻    │  │    🚀    │            │
//	│  └──────────┘  └──────────┘  └──────────┘             │
//	│                                                       │
//	│  TRENDING IN ACTION                                   │
//	│  ┌─────────────────────────────────────────────────┐  │
//	│  │ > One Piece          #1   ⭐ 9.2                │  │
//	│  │   Jujutsu Kaisen     #2   ⭐ 8.9                │  │
//	│  └─────────────────────────────────────────────────┘  │
//	└────────────────────────────────────────────────────────┘
package views

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"mangahub/internal/tui/api"
	"mangahub/internal/tui/styles"
	"mangahub/pkg/models"
)

// =====================================
// CATEGORY DEFINITIONS
// =====================================

// Category represents a manga genre category
type Category struct {
	Name  string
	Icon  string
	Color lipgloss.Color
}

// Categories available for browsing
var Categories = []Category{
	{Name: "Action", Icon: "⚔️", Color: styles.ColorError},
	{Name: "Romance", Icon: "💕", Color: styles.ColorSecondary},
	{Name: "Comedy", Icon: "😄", Color: styles.ColorWarning},
	{Name: "Fantasy", Icon: "🧙", Color: styles.ColorPrimary},
	{Name: "Horror", Icon: "👻", Color: styles.ColorDim},
	{Name: "Sci-Fi", Icon: "🚀", Color: styles.ColorSuccess},
	{Name: "Slice of Life", Icon: "🏠", Color: lipgloss.Color("#8be9fd")},
	{Name: "Sports", Icon: "⚽", Color: lipgloss.Color("#ffb86c")},
	{Name: "Mystery", Icon: "🔍", Color: lipgloss.Color("#f1fa8c")},
	{Name: "Adventure", Icon: "🗺️", Color: lipgloss.Color("#50fa7b")},
	{Name: "Drama", Icon: "🎭", Color: lipgloss.Color("#ff79c6")},
	{Name: "Supernatural", Icon: "✨", Color: lipgloss.Color("#bd93f9")},
}

// =====================================
// BROWSE MODEL
// =====================================

// BrowseModel holds the browse view state
type BrowseModel struct {
	// Window dimensions
	width  int
	height int

	// Theme
	theme *styles.Theme

	// Selection
	selectedCategory int
	selectedManga    int

	// Grid configuration
	columns int

	// Results for selected category
	categoryResults []models.Manga
	loading         bool

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

// BrowseCategoryLoadedMsg signals category manga loaded
type BrowseCategoryLoadedMsg struct {
	Category string
	Results  []models.Manga
}

// BrowseErrorMsg signals an error
type BrowseErrorMsg struct {
	Error error
}

// =====================================
// CONSTRUCTOR
// =====================================

// NewBrowse creates a new browse model
func NewBrowse() BrowseModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = styles.DefaultTheme.Spinner

	return BrowseModel{
		theme:            styles.DefaultTheme,
		spinner:          s,
		client:           api.GetClient(),
		columns:          4,
		selectedCategory: 0,
		categoryResults:  []models.Manga{},
	}
}

// =====================================
// BUBBLE TEA INTERFACE
// =====================================

// Init initializes the browse view
func (m BrowseModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.loadCategoryManga(Categories[0].Name),
	)
}

// loadCategoryManga loads manga for a category
func (m BrowseModel) loadCategoryManga(category string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		// Search by genre - the API will match genres in the genres JSON array
		results, _, err := m.client.SearchMangaByGenre(ctx, category, 1, 20)
		if err != nil {
			return BrowseErrorMsg{Error: err}
		}
		return BrowseCategoryLoadedMsg{
			Category: category,
			Results:  results,
		}
	}
}

// Update handles messages
func (m BrowseModel) Update(msg tea.Msg) (BrowseModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Adjust columns based on width
		m.columns = max((m.width-8)/20, 2)
		if m.columns > 6 {
			m.columns = 6
		}

	case tea.KeyMsg:
		// Calculate grid navigation
		rows := (len(Categories) + m.columns - 1) / m.columns
		currentRow := m.selectedCategory / m.columns
		currentCol := m.selectedCategory % m.columns

		switch msg.String() {
		case "left", "h":
			if len(m.categoryResults) > 0 && m.selectedManga >= 0 {
				// In results mode, go back to categories
				m.selectedManga = -1
			} else {
				// Navigate categories
				if currentCol > 0 {
					m.selectedCategory--
				}
			}
		case "right", "l":
			if m.selectedManga >= 0 {
				// Already in results
			} else if currentCol < m.columns-1 && m.selectedCategory < len(Categories)-1 {
				m.selectedCategory++
			}
		case "up", "k":
			if m.selectedManga >= 0 {
				// Navigate in results
				m.selectedManga--
				if m.selectedManga < 0 {
					m.selectedManga = -1 // Back to categories
				}
			} else if currentRow > 0 {
				m.selectedCategory -= m.columns
				if m.selectedCategory < 0 {
					m.selectedCategory = 0
				}
			}
		case "down", "j":
			if m.selectedManga >= 0 {
				// Navigate in results
				if m.selectedManga < len(m.categoryResults)-1 {
					m.selectedManga++
				}
			} else if currentRow < rows-1 {
				newIdx := m.selectedCategory + m.columns
				if newIdx < len(Categories) {
					m.selectedCategory = newIdx
				}
			}
		case "enter":
			if m.selectedManga >= 0 {
				// Select manga for details
				// Will be handled by parent
			} else {
				// Load category and enter results mode
				m.loading = true
				m.selectedManga = 0
				cmds = append(cmds, m.loadCategoryManga(Categories[m.selectedCategory].Name))
			}
		case "esc":
			if m.selectedManga >= 0 {
				m.selectedManga = -1 // Back to categories
			}
		case "tab":
			// Move to results if we have them
			if len(m.categoryResults) > 0 && m.selectedManga < 0 {
				m.selectedManga = 0
			}
		}

	case BrowseCategoryLoadedMsg:
		m.categoryResults = msg.Results
		m.loading = false
		if len(m.categoryResults) > 0 {
			m.selectedManga = 0
		}

	case BrowseErrorMsg:
		m.lastError = msg.Error
		m.loading = false

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// View renders the browse view
func (m BrowseModel) View() string {
	var sections []string

	// ===== HEADER =====
	header := m.theme.PanelHeader.Render("📚 BROWSE BY CATEGORY")
	sections = append(sections, header+"\n")

	// ===== CATEGORY GRID =====
	grid := m.renderCategoryGrid()
	sections = append(sections, grid+"\n")

	// ===== CATEGORY RESULTS =====
	results := m.renderCategoryResults()
	sections = append(sections, results)

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)
	return m.theme.Container.Width(m.width - 4).Render(content)
}

// =====================================
// RENDERERS
// =====================================

func (m BrowseModel) renderCategoryGrid() string {
	var rows []string
	var currentRow []string

	cardWidth := (m.width - 12) / m.columns
	if cardWidth < 14 {
		cardWidth = 14
	}

	for i, cat := range Categories {
		card := m.renderCategoryCard(cat, i == m.selectedCategory, cardWidth)
		currentRow = append(currentRow, card)

		// Start new row
		if len(currentRow) >= m.columns || i == len(Categories)-1 {
			row := lipgloss.JoinHorizontal(lipgloss.Top, currentRow...)
			rows = append(rows, row)
			currentRow = []string{}
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func (m BrowseModel) renderCategoryCard(cat Category, selected bool, width int) string {
	// Base style
	style := lipgloss.NewStyle().
		Width(width-2).
		Height(3).
		Align(lipgloss.Center).
		AlignVertical(lipgloss.Center).
		Margin(0, 1)

	if selected && m.selectedManga < 0 {
		// Selected category
		style = style.
			Border(lipgloss.RoundedBorder()).
			BorderForeground(cat.Color).
			Background(styles.ColorBackground)
	} else {
		// Unselected category
		style = style.
			Border(lipgloss.RoundedBorder()).
			BorderForeground(styles.ColorDim)
	}

	// Card content
	icon := lipgloss.NewStyle().Foreground(cat.Color).Render(cat.Icon)
	name := lipgloss.NewStyle().Foreground(lipgloss.Color("#f8f8f2")).Bold(true).Render(cat.Name)

	content := icon + "\n" + name
	return style.Render(content)
}

func (m BrowseModel) renderCategoryResults() string {
	if m.selectedCategory >= len(Categories) {
		return ""
	}

	cat := Categories[m.selectedCategory]

	// Header
	var headerText string
	if m.loading {
		headerText = fmt.Sprintf("LOADING %s... %s", strings.ToUpper(cat.Name), m.spinner.View())
	} else if len(m.categoryResults) > 0 {
		headerText = fmt.Sprintf("TRENDING IN %s", strings.ToUpper(cat.Name))
	} else {
		headerText = fmt.Sprintf("NO MANGA FOUND IN %s", strings.ToUpper(cat.Name))
	}

	header := m.theme.PanelHeader.Render(headerText)

	if len(m.categoryResults) == 0 {
		return header
	}

	// Build results list
	listStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cat.Color).
		Width(m.width-10).
		Padding(0, 1)

	var rows []string
	maxVisible := min(5, len(m.categoryResults))

	for i := 0; i < maxVisible; i++ {
		manga := m.categoryResults[i]
		row := m.renderResultRow(manga, i, i == m.selectedManga)
		rows = append(rows, row)
	}

	list := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return header + "\n" + listStyle.Render(list)
}

func (m BrowseModel) renderResultRow(manga models.Manga, rank int, selected bool) string {
	// Selector
	selector := "  "
	if selected {
		selector = m.theme.Primary.Render("> ")
	}

	// Rank badge
	rankBadge := m.theme.Badge.Render(fmt.Sprintf("#%d", rank+1))

	// Title
	title := manga.Title
	maxTitleLen := 35
	if len(title) > maxTitleLen {
		title = title[:maxTitleLen-3] + "..."
	}

	var titleStyle lipgloss.Style
	if selected {
		titleStyle = m.theme.Title.Bold(true)
	} else {
		titleStyle = m.theme.Description
	}
	titleText := titleStyle.Render(fmt.Sprintf("%-35s", title))

	// Author
	author := manga.Author
	if len(author) > 15 {
		author = author[:12] + "..."
	}
	authorText := m.theme.DimText.Render(fmt.Sprintf("%-15s", author))

	return selector + rankBadge + "  " + titleText + "  " + authorText
}

// =====================================
// PUBLIC METHODS
// =====================================

// GetSelectedManga returns the selected manga (if any)
func (m BrowseModel) GetSelectedManga() *models.Manga {
	if m.selectedManga >= 0 && m.selectedManga < len(m.categoryResults) {
		return &m.categoryResults[m.selectedManga]
	}
	return nil
}

// GetSelectedCategory returns the selected category
func (m BrowseModel) GetSelectedCategory() *Category {
	if m.selectedCategory < len(Categories) {
		return &Categories[m.selectedCategory]
	}
	return nil
}

// SetWidth sets the view width
func (m *BrowseModel) SetWidth(w int) {
	m.width = w
	m.columns = max((w-8)/20, 2)
	if m.columns > 6 {
		m.columns = 6
	}
}

// SetHeight sets the view height
func (m *BrowseModel) SetHeight(h int) {
	m.height = h
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
