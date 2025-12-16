// Package views - Manga Detail View
// Bloomberg-style manga information card
// Layout:
//
//	┌───────────────────────────────────────────────────────┐
//	│  ONE PIECE                                    ⭐ 9.2  │
//	│  Eiichiro Oda • Action/Adventure • Ongoing            │
//	│                                                       │
//	│  [ Cover ]   SYNOPSIS                                 │
//	│  [  Art  ]   Monkey D. Luffy dreams of finding the    │
//	│  [ ASCII ]   One Piece treasure...                    │
//	│                                                       │
//	│  YOUR PROGRESS:                                       │
//	│  [████████████░░] 89% (Ch 1093)                       │
//	│                                                       │
//	│  [r] Read Next   [c] Comments   [R] Rate              │
//	└───────────────────────────────────────────────────────┘
package views

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"mangahub/internal/tui/api"
	"mangahub/internal/tui/network"
	"mangahub/internal/tui/styles"
	"mangahub/pkg/models"
)

// =====================================
// DETAIL MODEL
// =====================================

// DetailModel holds the manga detail view state
type DetailModel struct {
	// Window dimensions
	width  int
	height int

	// Theme
	theme *styles.Theme

	// Data
	mangaID string
	manga   *models.Manga
	ratings *models.MangaRatingsSummary
	library *api.LibraryEntry

	// Loading
	loading        bool
	loadingRatings bool

	// Components
	spinner spinner.Model

	// UI state
	selectedAction int
	actions        []string

	// Error
	lastError error

	// API client
	client *api.Client
}

// =====================================
// MESSAGES
// =====================================

// DetailDataLoadedMsg signals manga detail loaded
type DetailDataLoadedMsg struct {
	Manga   *models.Manga
	Ratings *models.MangaRatingsSummary
	Library *api.LibraryEntry
}

// DetailErrorMsg signals an error
type DetailErrorMsg struct {
	Error error
}

// =====================================
// CONSTRUCTOR
// =====================================

// NewDetail creates a new detail model for a manga
func NewDetail(mangaID string) DetailModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = styles.DefaultTheme.Spinner

	return DetailModel{
		theme:   styles.DefaultTheme,
		spinner: s,
		client:  api.GetClient(),
		mangaID: mangaID,
		loading: true,
		actions: []string{"Read Next", "💬 Chat", "Comments", "Rate", "Add to Library"},
	}
}

// =====================================
// BUBBLE TEA INTERFACE
// =====================================

// Init initializes the detail view
func (m DetailModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.loadMangaDetail,
	)
}

// loadMangaDetail fetches manga details and ratings
func (m DetailModel) loadMangaDetail() tea.Msg {
	ctx := context.Background()

	// Load manga
	manga, err := m.client.GetManga(ctx, m.mangaID)
	if err != nil {
		return DetailErrorMsg{Error: err}
	}

	// Load ratings
	ratings, _ := m.client.GetRatings(ctx, m.mangaID)

	// Check if in library
	var library *api.LibraryEntry
	if m.client.IsAuthenticated() {
		entries, err := m.client.GetLibrary(ctx)
		if err == nil {
			for _, entry := range entries {
				if entry.MangaID == m.mangaID {
					library = &entry
					break
				}
			}
		}
	}

	return DetailDataLoadedMsg{
		Manga:   manga,
		Ratings: ratings,
		Library: library,
	}
}

// Update handles messages
func (m DetailModel) Update(msg tea.Msg) (DetailModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "left", "h":
			m.selectedAction--
			if m.selectedAction < 0 {
				m.selectedAction = len(m.actions) - 1
			}
		case "right", "l":
			m.selectedAction = (m.selectedAction + 1) % len(m.actions)

		case "r":
			// Read next
			// TODO: Implement read next action
		case "c":
			// Join Chat for this manga
			if m.manga != nil {
				mangaName := m.manga.Title
				roomID := "manga_" + m.mangaID
				return m, func() tea.Msg {
					return network.JoinRoomMsg{
						RoomID:    roomID,
						RoomName:  mangaName + " Discussion",
						MangaID:   m.mangaID,
						MangaName: mangaName,
					}
				}
			}
		case "C":
			// Comments (capital C)
			// TODO: Navigate to comments view
		case "R":
			// Rate
			// TODO: Open rating modal
		case "a":
			// Add to library
			if m.manga != nil && m.library == nil {
				return m, m.addToLibrary
			}
		}

	case DetailDataLoadedMsg:
		m.manga = msg.Manga
		m.ratings = msg.Ratings
		m.library = msg.Library
		m.loading = false
		// Update actions based on library status
		if m.library != nil {
			m.actions = []string{"Read Next", "💬 Chat", "Update Progress", "Comments", "Rate"}
		} else {
			m.actions = []string{"Add to Library", "💬 Chat", "Comments", "Rate"}
		}

	case DetailErrorMsg:
		m.lastError = msg.Error
		m.loading = false

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// addToLibrary adds the manga to user's library
func (m DetailModel) addToLibrary() tea.Msg {
	ctx := context.Background()
	err := m.client.AddToLibrary(ctx, m.mangaID)
	if err != nil {
		return DetailErrorMsg{Error: err}
	}
	// Reload to update library status
	return m.loadMangaDetail()
}

// View renders the detail view
func (m DetailModel) View() string {
	if m.loading {
		return m.theme.Container.Width(m.width - 4).Render(
			m.theme.Title.Render("Loading manga details...") + "\n\n" +
				m.spinner.View())
	}

	if m.manga == nil {
		return m.theme.Container.Width(m.width - 4).Render(
			m.theme.ErrorText.Render("Failed to load manga"))
	}

	// Build the detail card
	card := m.renderCard()

	return m.theme.CardFocused.Width(m.width - 4).Render(card)
}

// =====================================
// CARD RENDERER
// =====================================

func (m DetailModel) renderCard() string {
	var sections []string

	// ===== HEADER =====
	header := m.renderHeader()
	sections = append(sections, header)

	// ===== METADATA =====
	metadata := m.renderMetadata()
	sections = append(sections, metadata)

	// ===== BODY (ASCII Art + Synopsis) =====
	body := m.renderBody()
	sections = append(sections, body)

	// ===== PROGRESS (if in library) =====
	if m.library != nil {
		progress := m.renderProgress()
		sections = append(sections, progress)
	}

	// ===== RATING SUMMARY =====
	if m.ratings != nil {
		ratingSummary := m.renderRatingSummary()
		sections = append(sections, ratingSummary)
	}

	// ===== ACTIONS =====
	actions := m.renderActions()
	sections = append(sections, actions)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderHeader renders the title and rating
func (m DetailModel) renderHeader() string {
	// Title
	title := m.theme.Title.Bold(true).Render(strings.ToUpper(m.manga.Title))

	// Rating badge
	var ratingBadge string
	if m.ratings != nil && m.ratings.Aggregate.TotalRatings > 0 {
		ratingBadge = styles.RenderRatingWithNumber(m.ratings.Aggregate.AverageRating)
	} else {
		ratingBadge = m.theme.DimText.Render("No ratings yet")
	}

	// Combine with spacing
	titleWidth := lipgloss.Width(title)
	ratingWidth := lipgloss.Width(ratingBadge)
	availableWidth := m.width - 12
	padding := availableWidth - titleWidth - ratingWidth
	if padding < 2 {
		padding = 2
	}

	headerLine := title + strings.Repeat(" ", padding) + ratingBadge
	return headerLine + "\n"
}

// renderMetadata renders author, genres, status
func (m DetailModel) renderMetadata() string {
	parts := []string{}

	if m.manga.Author != "" {
		parts = append(parts, m.manga.Author)
	}

	// Genres (if available)
	if len(m.manga.Genres) > 0 {
		genres := strings.Join(m.manga.Genres[:min(3, len(m.manga.Genres))], "/")
		parts = append(parts, genres)
	}

	// Status
	status := "Ongoing"
	if m.manga.Status != "" {
		status = m.manga.Status
	}
	parts = append(parts, status)

	metadata := m.theme.Subtitle.Render(strings.Join(parts, " • "))
	return metadata + "\n"
}

// renderBody renders ASCII art placeholder and synopsis
func (m DetailModel) renderBody() string {
	// ASCII art placeholder (left side)
	asciiArt := m.renderASCIIArt()

	// Synopsis (right side)
	synopsis := m.renderSynopsis()

	// Calculate widths
	artWidth := 22
	synopsisWidth := m.width - artWidth - 12

	// Style containers
	artBox := lipgloss.NewStyle().
		Width(artWidth).
		Align(lipgloss.Center).
		Foreground(styles.ColorDim).
		Render(asciiArt)

	synopsisBox := lipgloss.NewStyle().
		Width(synopsisWidth).
		Render(synopsis)

	return lipgloss.JoinHorizontal(lipgloss.Top, artBox, "  ", synopsisBox) + "\n"
}

// renderASCIIArt creates a placeholder manga cover in ASCII
func (m DetailModel) renderASCIIArt() string {
	// Simple ASCII book/manga placeholder
	return `
┌──────────────────┐
│    ╔══════╗      │
│    ║ MANGA║      │
│    ║ COVER║      │
│    ║      ║      │
│    ╚══════╝      │
│                  │
│   📖 Vol. 1     │
│                  │
└──────────────────┘`
}

// renderSynopsis renders the manga description
func (m DetailModel) renderSynopsis() string {
	header := m.theme.PanelHeader.Render("SYNOPSIS")

	desc := m.manga.Description
	if desc == "" {
		desc = "No description available."
	}

	// Word wrap
	maxWidth := m.width - 36
	if maxWidth < 30 {
		maxWidth = 30
	}
	wrapped := wordWrap(desc, maxWidth)

	// Limit lines
	lines := strings.Split(wrapped, "\n")
	if len(lines) > 5 {
		lines = lines[:5]
		lines = append(lines, m.theme.DimText.Render("..."))
	}

	return header + "\n" + strings.Join(lines, "\n")
}

// renderProgress renders the reading progress section
func (m DetailModel) renderProgress() string {
	header := m.theme.PanelHeader.Render("YOUR PROGRESS")

	current := m.library.CurrentChapter
	total := m.library.TotalChapters

	var progressPct float64
	var progressText string
	if total > 0 {
		progressPct = float64(current) / float64(total)
		progressText = fmt.Sprintf("Chapter %d of %d", current, total)
	} else {
		progressPct = 0
		progressText = fmt.Sprintf("Chapter %d", current)
	}

	progressBar := styles.RenderProgressBar(progressPct, 20)

	return header + "\n" + progressBar + "  " + m.theme.Description.Render(progressText) + "\n"
}

// renderRatingSummary renders the rating statistics
func (m DetailModel) renderRatingSummary() string {
	header := m.theme.PanelHeader.Render("COMMUNITY RATINGS")

	agg := m.ratings.Aggregate
	avgRating := styles.RenderRating(agg.AverageRating, true)
	countText := m.theme.DimText.Render(fmt.Sprintf("(%d ratings)", agg.TotalRatings))

	return header + "\n" + avgRating + " " + countText + "\n"
}

// renderActions renders the action buttons
func (m DetailModel) renderActions() string {
	header := m.theme.PanelHeader.Render("ACTIONS")

	var buttons []string
	for i, action := range m.actions {
		var style lipgloss.Style
		if i == m.selectedAction {
			style = m.theme.ButtonActive
		} else {
			style = m.theme.ButtonInactive
		}
		buttons = append(buttons, style.Render(" "+action+" "))
	}

	buttonRow := lipgloss.JoinHorizontal(lipgloss.Center, buttons...)
	return header + "\n" + buttonRow
}

// =====================================
// HELPERS
// =====================================

// wordWrap wraps text to a maximum width
func wordWrap(text string, maxWidth int) string {
	if maxWidth <= 0 {
		return text
	}

	var lines []string
	words := strings.Fields(text)
	currentLine := ""

	for _, word := range words {
		if len(currentLine)+len(word)+1 <= maxWidth {
			if currentLine != "" {
				currentLine += " "
			}
			currentLine += word
		} else {
			if currentLine != "" {
				lines = append(lines, currentLine)
			}
			currentLine = word
		}
	}

	if currentLine != "" {
		lines = append(lines, currentLine)
	}

	return strings.Join(lines, "\n")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// SetMangaID sets the manga to display
func (m *DetailModel) SetMangaID(id string) {
	m.mangaID = id
	m.loading = true
	m.manga = nil
	m.ratings = nil
	m.library = nil
}

// SetWidth sets the view width
func (m *DetailModel) SetWidth(w int) {
	m.width = w
}

// SetHeight sets the view height
func (m *DetailModel) SetHeight(h int) {
	m.height = h
}
