// Package views - Activity Feed View
// Social activity feed with real-time updates
// Layout:
//
//	┌────────────────────────────────────────────────────────┐
//	│  🌐 ACTIVITY FEED                        [Live ●]     │
//	│                                                       │
//	│  ┌─────────────────────────────────────────────────┐  │
//	│  │ 📖 @manga_king started reading One Piece        │  │
//	│  │    "This is going to be epic!"                  │  │
//	│  │    2 min ago                           ♥ 5  💬 2│  │
//	│  ├─────────────────────────────────────────────────┤  │
//	│  │ ⭐ @reader42 rated Naruto 9/10                  │  │
//	│  │    5 min ago                           ♥ 3  💬 0│  │
//	│  ├─────────────────────────────────────────────────┤  │
//	│  │ ✅ @bookworm completed Attack on Titan         │  │
//	│  │    "What a journey!"                            │  │
//	│  │    10 min ago                          ♥ 12 💬 5│  │
//	│  └─────────────────────────────────────────────────┘  │
//	│                                                       │
//	│  [↑↓] Navigate  [Enter] View  [l] Like  [r] Refresh   │
//	└────────────────────────────────────────────────────────┘
package views

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"mangahub/internal/tui/api"
	"mangahub/internal/tui/styles"
)

// =====================================
// ACTIVITY TYPES
// =====================================

// ActivityType represents the type of activity
type ActivityType string

const (
	ActivityStarted   ActivityType = "started"
	ActivityCompleted ActivityType = "completed"
	ActivityRated     ActivityType = "rated"
	ActivityComment   ActivityType = "comment"
	ActivityProgress  ActivityType = "progress"
)

// Activity represents a single activity item
type Activity struct {
	ID        string
	Type      ActivityType
	Username  string
	MangaID   string
	MangaName string
	Message   string
	Rating    float64
	Chapter   int
	Likes     int
	Comments  int
	Timestamp time.Time
}

// =====================================
// ACTIVITY MODEL
// =====================================

// ActivityModel holds the activity feed state
type ActivityModel struct {
	// Window dimensions
	width  int
	height int

	// Theme
	theme *styles.Theme

	// Data
	activities    []Activity
	selectedIndex int

	// Loading
	loading   bool
	isLive    bool
	lastFetch time.Time

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

// ActivityLoadedMsg signals activities were loaded
type ActivityLoadedMsg struct {
	Activities []Activity
}

// ActivityErrorMsg signals an error
type ActivityErrorMsg struct {
	Error error
}

// ActivityTickMsg for live updates
type ActivityTickMsg struct{}

// =====================================
// CONSTRUCTOR
// =====================================

// NewActivity creates a new activity feed model
func NewActivity() ActivityModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = styles.DefaultTheme.Spinner

	return ActivityModel{
		theme:      styles.DefaultTheme,
		spinner:    s,
		client:     api.GetClient(),
		activities: []Activity{},
		isLive:     true,
		loading:    true,
	}
}

// =====================================
// BUBBLE TEA INTERFACE
// =====================================

// Init initializes the activity view
func (m ActivityModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.loadActivities,
	)
}

// loadActivities fetches recent activities
func (m ActivityModel) loadActivities() tea.Msg {
	ctx := context.Background()

	// Get real activity feed from API
	activityEntries, err := m.client.GetActivities(ctx, 20)
	if err != nil {
		// Generate mock activities if API fails
		return ActivityLoadedMsg{
			Activities: m.generateMockActivities(),
		}
	}

	// Convert API ActivityEntry to view Activity struct
	var activities []Activity
	for _, entry := range activityEntries {
		// Determine activity type from API's activity_type
		var actType ActivityType
		switch entry.ActivityType {
		case "comment":
			actType = ActivityComment
		case "rating":
			actType = ActivityRated
		case "progress":
			actType = ActivityProgress
		case "list_add":
			actType = ActivityStarted
		default:
			actType = ActivityProgress
		}

		// Build message from API data
		message := ""
		if entry.CommentText != "" {
			message = entry.CommentText
		}

		// Extract rating and chapter
		rating := 0.0
		if entry.Rating != nil {
			rating = *entry.Rating
		}
		chapter := 0
		if entry.Chapter != nil {
			chapter = *entry.Chapter
		}

		activities = append(activities, Activity{
			ID:        entry.ID,
			Type:      actType,
			Username:  entry.Username,
			MangaID:   entry.MangaID,
			MangaName: entry.MangaTitle,
			Message:   message,
			Rating:    rating,
			Chapter:   chapter,
			Likes:     0, // Not provided by API
			Comments:  0, // Not provided by API
			Timestamp: entry.CreatedAt,
		})
	}

	// Fallback to mock if no activities
	if len(activities) == 0 {
		return ActivityLoadedMsg{Activities: m.generateMockActivities()}
	}

	return ActivityLoadedMsg{Activities: activities}
}

// generateMockActivities creates sample activities for demo
func (m ActivityModel) generateMockActivities() []Activity {
	return []Activity{
		{
			ID:        "1",
			Type:      ActivityStarted,
			Username:  "manga_king",
			MangaName: "One Piece",
			Message:   "Finally starting the greatest adventure!",
			Likes:     42,
			Comments:  8,
			Timestamp: time.Now().Add(-2 * time.Minute),
		},
		{
			ID:        "2",
			Type:      ActivityRated,
			Username:  "reader42",
			MangaName: "Naruto",
			Rating:    9.0,
			Likes:     15,
			Comments:  3,
			Timestamp: time.Now().Add(-5 * time.Minute),
		},
		{
			ID:        "3",
			Type:      ActivityCompleted,
			Username:  "bookworm",
			MangaName: "Attack on Titan",
			Message:   "What an incredible journey! 10/10",
			Likes:     128,
			Comments:  24,
			Timestamp: time.Now().Add(-10 * time.Minute),
		},
		{
			ID:        "4",
			Type:      ActivityProgress,
			Username:  "speedreader",
			MangaName: "Jujutsu Kaisen",
			Chapter:   250,
			Likes:     8,
			Comments:  1,
			Timestamp: time.Now().Add(-15 * time.Minute),
		},
		{
			ID:        "5",
			Type:      ActivityComment,
			Username:  "otaku_prime",
			MangaName: "Demon Slayer",
			Message:   "The animation in this arc is godly!",
			Likes:     56,
			Comments:  12,
			Timestamp: time.Now().Add(-20 * time.Minute),
		},
	}
}

func getRandomMessage(i int) string {
	messages := []string{
		"Starting this masterpiece!",
		"Finally caught up!",
		"This chapter was insane!",
		"Can't stop reading!",
		"Highly recommended!",
		"The plot thickens...",
		"Mind = blown",
		"This arc is fire!",
		"Peak fiction right here",
		"What a ride!",
	}
	return messages[i%len(messages)]
}

// Update handles messages
func (m ActivityModel) Update(msg tea.Msg) (ActivityModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if len(m.activities) > 0 {
				m.selectedIndex--
				if m.selectedIndex < 0 {
					m.selectedIndex = len(m.activities) - 1
				}
			}
		case "down", "j":
			if len(m.activities) > 0 {
				m.selectedIndex = (m.selectedIndex + 1) % len(m.activities)
			}
		case "r":
			// Refresh
			m.loading = true
			cmds = append(cmds, m.loadActivities)
		case "l":
			// Toggle live
			m.isLive = !m.isLive
		case "enter":
			// View manga details
			// Will be handled by parent
		}

	case ActivityLoadedMsg:
		m.activities = msg.Activities
		m.loading = false
		m.lastFetch = time.Now()

	case ActivityErrorMsg:
		m.lastError = msg.Error
		m.loading = false

	case ActivityTickMsg:
		if m.isLive {
			cmds = append(cmds, m.loadActivities)
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// View renders the activity view
func (m ActivityModel) View() string {
	var sections []string

	// ===== HEADER =====
	header := m.renderHeader()
	sections = append(sections, header+"\n")

	// ===== ACTIVITY FEED =====
	feed := m.renderFeed()
	sections = append(sections, feed)

	// ===== HELP =====
	help := m.renderHelp()
	sections = append(sections, help)

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)
	return m.theme.Container.Width(m.width - 4).Render(content)
}

// =====================================
// RENDERERS
// =====================================

func (m ActivityModel) renderHeader() string {
	title := m.theme.PanelHeader.Render("🌐 ACTIVITY FEED")

	// Live indicator
	var liveIndicator string
	if m.isLive {
		liveIndicator = m.theme.Success.Render("[Live ●]")
	} else {
		liveIndicator = m.theme.DimText.Render("[Paused ○]")
	}

	// Calculate padding
	titleWidth := lipgloss.Width(title)
	indicatorWidth := lipgloss.Width(liveIndicator)
	availableWidth := m.width - 10
	padding := availableWidth - titleWidth - indicatorWidth
	if padding < 2 {
		padding = 2
	}

	return title + strings.Repeat(" ", padding) + liveIndicator
}

func (m ActivityModel) renderFeed() string {
	if m.loading {
		return m.theme.DimText.Render("Loading activities... " + m.spinner.View())
	}

	if len(m.activities) == 0 {
		return m.theme.DimText.Render("No recent activity. Be the first to share!")
	}

	// Build activity list
	listStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.ColorDim).
		Width(m.width-10).
		Padding(0, 1)

	var items []string
	maxVisible := minInt((m.height-10)/5, len(m.activities))
	if maxVisible < 1 {
		maxVisible = 1
	}

	for i := 0; i < maxVisible; i++ {
		activity := m.activities[i]
		item := m.renderActivityItem(activity, i == m.selectedIndex)
		items = append(items, item)

		// Add separator (except for last)
		if i < maxVisible-1 {
			sep := m.theme.DimText.Render(strings.Repeat("─", m.width-16))
			items = append(items, sep)
		}
	}

	list := lipgloss.JoinVertical(lipgloss.Left, items...)
	return listStyle.Render(list)
}

func (m ActivityModel) renderActivityItem(activity Activity, selected bool) string {
	var lines []string

	// ===== LINE 1: Action =====
	icon := m.getActivityIcon(activity.Type)
	username := m.theme.Primary.Bold(true).Render("@" + activity.Username)
	action := m.getActivityAction(activity)

	line1 := icon + " " + username + " " + action
	if selected {
		line1 = m.theme.Secondary.Render("> ") + line1
	} else {
		line1 = "  " + line1
	}
	lines = append(lines, line1)

	// ===== LINE 2: Message (if any) =====
	if activity.Message != "" {
		quote := m.theme.DimText.Italic(true).Render(`"` + activity.Message + `"`)
		lines = append(lines, "     "+quote)
	}

	// ===== LINE 3: Time + Engagement =====
	timeAgo := formatTimeAgo(activity.Timestamp)
	timeText := m.theme.DimText.Render(timeAgo)

	engagement := m.theme.Secondary.Render(fmt.Sprintf("♥ %d", activity.Likes)) + "  " +
		m.theme.DimText.Render(fmt.Sprintf("💬 %d", activity.Comments))

	// Padding between time and engagement
	timeWidth := lipgloss.Width(timeText)
	engWidth := lipgloss.Width(engagement)
	availableWidth := m.width - 20
	padding := availableWidth - timeWidth - engWidth
	if padding < 2 {
		padding = 2
	}

	line3 := "     " + timeText + strings.Repeat(" ", padding) + engagement
	lines = append(lines, line3)

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m ActivityModel) getActivityIcon(actType ActivityType) string {
	switch actType {
	case ActivityStarted:
		return "📖"
	case ActivityCompleted:
		return "✅"
	case ActivityRated:
		return "⭐"
	case ActivityComment:
		return "💬"
	case ActivityProgress:
		return "📈"
	default:
		return "📌"
	}
}

func (m ActivityModel) getActivityAction(activity Activity) string {
	mangaStyle := m.theme.Title
	manga := mangaStyle.Render(activity.MangaName)

	switch activity.Type {
	case ActivityStarted:
		return "started reading " + manga
	case ActivityCompleted:
		return "completed " + manga
	case ActivityRated:
		rating := m.theme.Warning.Render(fmt.Sprintf("%.1f/10", activity.Rating))
		return "rated " + manga + " " + rating
	case ActivityComment:
		return "commented on " + manga
	case ActivityProgress:
		chapter := m.theme.Primary.Render(fmt.Sprintf("Ch. %d", activity.Chapter))
		return "reached " + chapter + " in " + manga
	default:
		return "interacted with " + manga
	}
}

func (m ActivityModel) renderHelp() string {
	helpItems := []string{
		m.theme.Key.Render("[↑↓]") + " " + m.theme.DimText.Render("Navigate"),
		m.theme.Key.Render("[Enter]") + " " + m.theme.DimText.Render("View Manga"),
		m.theme.Key.Render("[l]") + " " + m.theme.DimText.Render("Toggle Live"),
		m.theme.Key.Render("[r]") + " " + m.theme.DimText.Render("Refresh"),
	}
	return "\n" + lipgloss.JoinHorizontal(lipgloss.Center, helpItems...)
}

// =====================================
// HELPERS
// =====================================

func formatTimeAgo(t time.Time) string {
	duration := time.Since(t)

	switch {
	case duration < time.Minute:
		return "just now"
	case duration < time.Hour:
		mins := int(duration.Minutes())
		if mins == 1 {
			return "1 min ago"
		}
		return fmt.Sprintf("%d mins ago", mins)
	case duration < 24*time.Hour:
		hours := int(duration.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	default:
		days := int(duration.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// =====================================
// PUBLIC METHODS
// =====================================

// GetSelectedActivity returns the selected activity
func (m ActivityModel) GetSelectedActivity() *Activity {
	if len(m.activities) > 0 && m.selectedIndex < len(m.activities) {
		return &m.activities[m.selectedIndex]
	}
	return nil
}

// SetWidth sets the view width
func (m *ActivityModel) SetWidth(w int) {
	m.width = w
}

// SetHeight sets the view height
func (m *ActivityModel) SetHeight(h int) {
	m.height = h
}

// Refresh triggers a refresh of the activity feed
func (m *ActivityModel) Refresh() tea.Cmd {
	m.loading = true
	return m.loadActivities
}
