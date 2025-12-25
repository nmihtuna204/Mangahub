// Package views - Dashboard View
// Main dashboard với split-pane layout
// Layout:
//
//	┌── 📚 Continue Reading (2/3) ──┐┌── 🔥 Trending (1/3) ──┐
//	│ ▶ One Piece Ch. 1093 [████░]  ││ 1. Solo Leveling      │
//	└───────────────────────────────┘└───────────────────────┘
//	┌── 📌 Recent Activity (fixed height) ───────────────────┐
//	│ [12:05] User1 rated One Piece 5★                       │
//	└────────────────────────────────────────────────────────┘
package views

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"mangahub/internal/tui/api"
	"mangahub/internal/tui/styles"
)

// =====================================
// DASHBOARD MODEL
// =====================================

// DashboardModel holds the dashboard view state
type DashboardModel struct {
	// Window dimensions
	width  int
	height int

	// Theme
	theme *styles.Theme

	// Data
	reading  []ReadingEntry
	trending []TrendingEntry
	activity []ActivityEntry

	// Loading states
	loadingReading  bool
	loadingTrending bool
	loadingActivity bool

	// Selection
	selectedPane  int // 0=reading, 1=trending, 2=activity
	selectedIndex int

	// Components
	spinner spinner.Model

	// Error
	lastError error

	// API client
	client *api.Client
}

// ReadingEntry represents a manga in "Continue Reading"
type ReadingEntry struct {
	MangaID        string
	Title          string
	CurrentChapter int
	TotalChapters  int
	LastReadAt     time.Time
}

// TrendingEntry represents a trending manga
type TrendingEntry struct {
	Rank   int
	Title  string
	Rating float64
	Note   string // e.g., "New Season Announced!"
}

// ActivityEntry represents an activity feed item
type ActivityEntry struct {
	Time   time.Time
	User   string
	Action string
}

// =====================================
// MESSAGES
// =====================================

// DashboardDataLoadedMsg signals data has been loaded
type DashboardDataLoadedMsg struct {
	Reading  []ReadingEntry
	Trending []TrendingEntry
	Activity []ActivityEntry
}

// DashboardErrorMsg signals an error occurred
type DashboardErrorMsg struct {
	Error error
}

// =====================================
// CONSTRUCTOR
// =====================================

// NewDashboard creates a new dashboard model
func NewDashboard() DashboardModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = styles.DefaultTheme.Spinner

	return DashboardModel{
		theme:           styles.DefaultTheme,
		spinner:         s,
		client:          api.GetClient(),
		loadingReading:  true,
		loadingTrending: true,
		loadingActivity: true,
	}
}

// =====================================
// BUBBLE TEA INTERFACE
// =====================================

// Init initializes the dashboard
func (m DashboardModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.loadDashboardData,
	)
}

// loadDashboardData fetches all dashboard data
func (m DashboardModel) loadDashboardData() tea.Msg {
	ctx := context.Background()

	var reading []ReadingEntry
	var trending []TrendingEntry
	var activity []ActivityEntry

	// Load library (reading) if authenticated
	if m.client.IsAuthenticated() {
		library, err := m.client.GetLibrary(ctx)
		if err == nil {
			for _, entry := range library {
				if entry.Status == "reading" {
					reading = append(reading, ReadingEntry{
						MangaID:        entry.MangaID,
						Title:          entry.Manga.Title,
						CurrentChapter: entry.CurrentChapter,
						TotalChapters:  entry.Manga.TotalChapters, // Get from manga, not entry
						LastReadAt:     entry.LastReadAt,
					})
				}
			}
		}
	}

	// Load trending
	trendingData, err := m.client.GetTrending(ctx, 5, 7)
	if err == nil {
		for _, t := range trendingData {
			trending = append(trending, TrendingEntry{
				Rank:   t.Rank,
				Title:  t.Title,
				Rating: t.AverageRating,
			})
		}
	}

	// Load real activities from API
	activities, err := m.client.GetActivities(ctx, 10)
	if err == nil && len(activities) > 0 {
		for _, a := range activities {
			action := formatActivityAction(a.ActivityType, a.MangaTitle, a.Rating, a.Chapter)
			activity = append(activity, ActivityEntry{
				Time:   a.CreatedAt,
				User:   a.Username,
				Action: action,
			})
		}
	}

	// Fallback to sample activities if no real data
	if len(activity) == 0 {
		activity = []ActivityEntry{
			{Time: time.Now().Add(-3 * time.Minute), User: "reader1", Action: "rated One Piece 5★"},
			{Time: time.Now().Add(-1 * time.Hour), User: "mangafan", Action: "added Chainsaw Man to library"},
			{Time: time.Now().Add(-2 * time.Hour), User: "system", Action: "New chapter: Jujutsu Kaisen 260"},
		}
	}

	return DashboardDataLoadedMsg{
		Reading:  reading,
		Trending: trending,
		Activity: activity,
	}
}

// formatActivityAction converts activity type to human-readable action
func formatActivityAction(activityType, mangaTitle string, rating *float64, chapter *int) string {
	switch activityType {
	case "manga_rated":
		if rating != nil {
			return fmt.Sprintf("rated %s %.1f★", mangaTitle, *rating)
		}
		return fmt.Sprintf("rated %s", mangaTitle)
	case "chapter_read":
		if chapter != nil {
			return fmt.Sprintf("read Ch.%d of %s", *chapter, mangaTitle)
		}
		return fmt.Sprintf("is reading %s", mangaTitle)
	case "manga_completed":
		return fmt.Sprintf("completed %s", mangaTitle)
	case "comment_added":
		return fmt.Sprintf("commented on %s", mangaTitle)
	case "library_add":
		return fmt.Sprintf("added %s to library", mangaTitle)
	default:
		return fmt.Sprintf("%s %s", activityType, mangaTitle)
	}
}

// Update handles messages
func (m DashboardModel) Update(msg tea.Msg) (DashboardModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			m.selectedIndex++
			m = m.clampSelection()
		case "k", "up":
			m.selectedIndex--
			m = m.clampSelection()
		case "tab":
			m.selectedPane = (m.selectedPane + 1) % 3
			m.selectedIndex = 0
		case "shift+tab":
			m.selectedPane = (m.selectedPane + 2) % 3
			m.selectedIndex = 0
		case "r":
			// Refresh
			m.loadingReading = true
			m.loadingTrending = true
			m.loadingActivity = true
			return m, m.loadDashboardData
		}

	case DashboardDataLoadedMsg:
		m.reading = msg.Reading
		m.trending = msg.Trending
		m.activity = msg.Activity
		m.loadingReading = false
		m.loadingTrending = false
		m.loadingActivity = false

	case DashboardErrorMsg:
		m.lastError = msg.Error

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// clampSelection ensures selection is within bounds
func (m DashboardModel) clampSelection() DashboardModel {
	var maxIndex int
	switch m.selectedPane {
	case 0:
		maxIndex = len(m.reading) - 1
	case 1:
		maxIndex = len(m.trending) - 1
	case 2:
		maxIndex = len(m.activity) - 1
	}
	if m.selectedIndex < 0 {
		m.selectedIndex = 0
	}
	if maxIndex < 0 {
		maxIndex = 0
	}
	if m.selectedIndex > maxIndex {
		m.selectedIndex = maxIndex
	}
	return m
}

// View renders the dashboard
func (m DashboardModel) View() string {
	// Calculate layout widths
	leftWidth, rightWidth := styles.PanelWidths(m.width)
	isCompact := styles.IsCompactMode(m.width)

	// Render panels
	readingPanel := m.renderReadingPanel(leftWidth)
	trendingPanel := m.renderTrendingPanel(rightWidth)
	activityPanel := m.renderActivityPanel(m.width - 4)

	// Layout based on terminal width
	var topRow string
	if isCompact || rightWidth == 0 {
		// Vertical stack for narrow terminals
		topRow = lipgloss.JoinVertical(lipgloss.Left, readingPanel, trendingPanel)
	} else {
		// Horizontal split for wide terminals
		topRow = lipgloss.JoinHorizontal(lipgloss.Top, readingPanel, trendingPanel)
	}

	// Combine with activity panel
	return lipgloss.JoinVertical(lipgloss.Left, topRow, activityPanel)
}

// =====================================
// PANEL RENDERERS
// =====================================

// renderReadingPanel renders the "Continue Reading" panel
func (m DashboardModel) renderReadingPanel(width int) string {
	// Panel header
	header := m.theme.PanelHeader.Render(styles.BookIcon() + " CONTINUE READING")

	// Panel border style
	borderStyle := m.theme.Panel
	if m.selectedPane == 0 {
		borderStyle = m.theme.FocusedContainer
	}

	// Content
	var content string
	if m.loadingReading {
		content = m.spinner.View() + " Loading..."
	} else if len(m.reading) == 0 {
		content = m.theme.DimText.Render("No manga in progress.\nStart reading something!")
	} else {
		for i, entry := range m.reading {
			// Progress calculation
			var progress float64
			if entry.TotalChapters > 0 {
				progress = float64(entry.CurrentChapter) / float64(entry.TotalChapters)
			} else {
				progress = 0.5 // Unknown total, show 50%
			}

			// Selection highlight
			prefix := "  "
			style := m.theme.ListItem
			if m.selectedPane == 0 && m.selectedIndex == i {
				prefix = "▶ "
				style = m.theme.ListItemSelected
			}

			// Format entry
			title := truncate(entry.Title, 20)
			chapterInfo := fmt.Sprintf("Ch. %d", entry.CurrentChapter)
			if entry.TotalChapters > 0 {
				chapterInfo = fmt.Sprintf("Ch. %d/%d", entry.CurrentChapter, entry.TotalChapters)
			}

			progressBar := styles.RenderProgressBar(progress, 8)
			line := style.Render(fmt.Sprintf("%s%-20s %s %s",
				prefix, title, chapterInfo, progressBar))

			content += line + "\n"
		}
	}

	// Combine and wrap in border
	panelContent := header + "\n" + content
	return borderStyle.Width(width).Render(panelContent)
}

// renderTrendingPanel renders the "Trending" panel
func (m DashboardModel) renderTrendingPanel(width int) string {
	if width == 0 {
		return ""
	}

	// Panel header
	header := m.theme.PanelHeader.Render(styles.FireIcon() + " TRENDING NOW")

	// Panel border style
	borderStyle := m.theme.Panel
	if m.selectedPane == 1 {
		borderStyle = m.theme.FocusedContainer
	}

	// Content
	var content string
	if m.loadingTrending {
		content = m.spinner.View() + " Loading..."
	} else if len(m.trending) == 0 {
		content = m.theme.DimText.Render("No trending data")
	} else {
		for i, entry := range m.trending {
			// Selection highlight
			style := m.theme.ListItem
			if m.selectedPane == 1 && m.selectedIndex == i {
				style = m.theme.ListItemSelected
			}

			// Format rating
			ratingStr := styles.RenderRatingWithNumber(entry.Rating)

			// Format entry
			line := style.Render(fmt.Sprintf("%d. %s (%s)",
				entry.Rank, truncate(entry.Title, 15), ratingStr))

			if entry.Note != "" {
				line += "\n   " + m.theme.DimText.Render(entry.Note)
			}

			content += line + "\n"
		}
	}

	// Combine and wrap in border
	panelContent := header + "\n" + content
	return borderStyle.Width(width).Render(panelContent)
}

// renderActivityPanel renders the "Recent Activity" panel
func (m DashboardModel) renderActivityPanel(width int) string {
	// Panel header
	header := m.theme.PanelHeader.Render(styles.ActivityIcon() + " RECENT ACTIVITY")

	// Panel border style
	borderStyle := m.theme.Panel
	if m.selectedPane == 2 {
		borderStyle = m.theme.FocusedContainer
	}

	// Content
	var content string
	if m.loadingActivity {
		content = m.spinner.View() + " Loading..."
	} else if len(m.activity) == 0 {
		content = m.theme.DimText.Render("No recent activity")
	} else {
		for i, entry := range m.activity {
			// Selection highlight
			style := m.theme.ListItem
			if m.selectedPane == 2 && m.selectedIndex == i {
				style = m.theme.ListItemSelected
			}

			// Format time
			timeStr := entry.Time.Format("15:04")

			// Build activity line
			line := m.theme.ActivityTime.Render("["+timeStr+"] ") +
				m.theme.ActivityUser.Render(entry.User+" ") +
				style.Render(entry.Action)

			content += line + "\n"
		}
	}

	// Combine and wrap in border (fixed height)
	panelContent := header + "\n" + content
	return borderStyle.Width(width).Height(8).Render(panelContent)
}

// =====================================
// HELPER FUNCTIONS
// =====================================

// truncate shortens a string to max length with ellipsis
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// GetSelectedMangaID returns the currently selected manga ID
func (m DashboardModel) GetSelectedMangaID() string {
	if m.selectedPane == 0 && m.selectedIndex < len(m.reading) {
		return m.reading[m.selectedIndex].MangaID
	}
	return ""
}

// SetWidth sets the dashboard width
func (m *DashboardModel) SetWidth(w int) {
	m.width = w
}

// SetHeight sets the dashboard height
func (m *DashboardModel) SetHeight(h int) {
	m.height = h
}
